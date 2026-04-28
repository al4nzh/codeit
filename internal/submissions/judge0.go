package submissions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	ErrJudgeUnavailable = errors.New("judge service is unavailable")
	ErrUnsupportedLang  = errors.New("unsupported language")
)

type Judge interface {
	Evaluate(ctx context.Context, language string, code string, input string, expectedOutput string) (bool, error)
}

type Judge0Client struct {
	baseURL      string
	apiKey       string
	rapidAPIHost string
	httpClient   *http.Client
}

type judge0SubmissionRequest struct {
	SourceCode     string `json:"source_code"`
	LanguageID     int    `json:"language_id"`
	Stdin          string `json:"stdin,omitempty"`
	ExpectedOutput string `json:"expected_output,omitempty"`
}

type judge0SubmissionResponse struct {
	Status struct {
		ID int `json:"id"`
	} `json:"status"`
}

func NewJudge0Client(baseURL, apiKey, rapidAPIHost string) (*Judge0Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, errors.New("JUDGE0_BASE_URL is required")
	}

	return &Judge0Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       strings.TrimSpace(apiKey),
		rapidAPIHost: strings.TrimSpace(rapidAPIHost),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *Judge0Client) Evaluate(ctx context.Context, language string, code string, input string, expectedOutput string) (bool, error) {
	languageID, err := judge0LanguageID(language)
	if err != nil {
		return false, err
	}

	payload := judge0SubmissionRequest{
		SourceCode:     code,
		LanguageID:     languageID,
		Stdin:          input,
		ExpectedOutput: expectedOutput,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}

	url := c.baseURL + "/submissions?base64_encoded=false&wait=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	// RapidAPI Judge0 uses X-RapidAPI-* headers.
	if c.rapidAPIHost != "" {
		req.Header.Set("X-RapidAPI-Host", c.rapidAPIHost)
		if c.apiKey != "" {
			req.Header.Set("X-RapidAPI-Key", c.apiKey)
		}
	} else if c.apiKey != "" {
		// Direct/self-hosted Judge0 deployments may use X-Auth-Token.
		req.Header.Set("X-Auth-Token", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, ErrJudgeUnavailable
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("%w: status %d", ErrJudgeUnavailable, resp.StatusCode)
	}

	var judged judge0SubmissionResponse
	if err := json.Unmarshal(respBody, &judged); err != nil {
		return false, err
	}

	// Judge0 status id 3 is "Accepted".
	return judged.Status.ID == 3, nil
}

func judge0LanguageID(language string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "go", "golang":
		return 95, nil
	case "python", "python3":
		return 71, nil
	case "javascript", "node", "nodejs":
		return 63, nil
	case "java":
		return 62, nil
	case "cpp", "c++":
		return 54, nil
	default:
		return 0, ErrUnsupportedLang
	}
}
