package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var ErrAnalyzerUnavailable = errors.New("analyzer service is unavailable")

type AnalyzerClient interface {
	Analyze(ctx context.Context, input AnalyzerInput) (*AnalyzeLastSubmissionResult, error)
}

type HTTPAnalyzerClient struct {
	url         string
	apiKey      string
	openAIKey   string
	openAIModel string
	httpClient  *http.Client
}

func NewHTTPAnalyzerClient(url, apiKey, openAIKey, openAIModel string) *HTTPAnalyzerClient {
	openAIModel = strings.TrimSpace(openAIModel)
	if openAIModel == "" {
		openAIModel = "gpt-4o-mini"
	}
	return &HTTPAnalyzerClient{
		url:         strings.TrimSpace(url),
		apiKey:      strings.TrimSpace(apiKey),
		openAIKey:   strings.TrimSpace(openAIKey),
		openAIModel: openAIModel,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type analyzerResponse struct {
	Summary     string   `json:"summary"`
	Strengths   []string `json:"strengths"`
	Issues      []string `json:"issues"`
	Suggestions []string `json:"suggestions"`
	Score       *float64 `json:"score"`
	Analysis    string   `json:"analysis"`
}

func (c *HTTPAnalyzerClient) Analyze(ctx context.Context, input AnalyzerInput) (*AnalyzeLastSubmissionResult, error) {
	// Prefer direct OpenAI call when configured.
	if c.openAIKey != "" {
		return c.analyzeWithOpenAI(ctx, input)
	}
	if c.url == "" {
		return nil, ErrAnalyzerUnavailable
	}

	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ErrAnalyzerUnavailable
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, ErrAnalyzerUnavailable
	}

	var parsed analyzerResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		// Fallback for plain text analyzer responses.
		return &AnalyzeLastSubmissionResult{
			Summary: string(respBody),
		}, nil
	}
	if parsed.Summary == "" {
		parsed.Summary = parsed.Analysis
	}

	return &AnalyzeLastSubmissionResult{
		Summary:     parsed.Summary,
		Strengths:   parsed.Strengths,
		Issues:      parsed.Issues,
		Suggestions: parsed.Suggestions,
		Score:       parsed.Score,
	}, nil
}

type openAIChatCompletionsRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Temperature float64 `json:"temperature"`
}

type openAIChatCompletionsResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *HTTPAnalyzerClient) analyzeWithOpenAI(ctx context.Context, input AnalyzerInput) (*AnalyzeLastSubmissionResult, error) {
	systemPrompt := "You are a competitive programming coach. Return ONLY valid JSON with keys: summary (string), strengths (string[]), issues (string[]), suggestions (string[]), score (number 0-10, can include decimals). Be concise and specific."
	if strings.TrimSpace(input.Code) == "" {
		systemPrompt += " If code is empty, explain that no submission exists yet and provide actionable guidance to start solving the problem."
	}
	userPrompt := "Analyze this latest submission.\n\n" +
		"Problem title: " + input.ProblemTitle + "\n" +
		"Problem description: " + input.ProblemDescription + "\n" +
		"Language: " + input.Language + "\n" +
		"Passed: " + intToString(input.PassedCount) + "/" + intToString(input.TotalCount) + "\n\n" +
		"Code:\n" + input.Code

	reqBody := openAIChatCompletionsRequest{
		Model:       c.openAIModel,
		Temperature: 0.2,
	}
	reqBody.Messages = append(reqBody.Messages,
		struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{Role: "system", Content: systemPrompt},
		struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{Role: "user", Content: userPrompt},
	)

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.openAIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ErrAnalyzerUnavailable
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, ErrAnalyzerUnavailable
	}

	var raw openAIChatCompletionsResponse
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, err
	}
	if len(raw.Choices) == 0 {
		return nil, ErrAnalyzerUnavailable
	}
	content := strings.TrimSpace(raw.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var parsed analyzerResponse
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return &AnalyzeLastSubmissionResult{Summary: content}, nil
	}
	if parsed.Summary == "" {
		parsed.Summary = parsed.Analysis
	}
	if parsed.Score != nil {
		v := *parsed.Score
		// Backward compatibility: if model/legacy cache emits 0..1, convert to 0..10.
		if v >= 0 && v <= 1 {
			v = v * 10
		}
		if v < 0 {
			v = 0
		}
		if v > 10 {
			v = 10
		}
		parsed.Score = &v
	}
	return &AnalyzeLastSubmissionResult{
		Summary:     parsed.Summary,
		Strengths:   parsed.Strengths,
		Issues:      parsed.Issues,
		Suggestions: parsed.Suggestions,
		Score:       parsed.Score,
	}, nil
}

func intToString(v int) string {
	return strconv.Itoa(v)
}
