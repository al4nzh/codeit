package problems

import "time"

type TestCase struct {
	ID        int64
	ProblemID int64
	Input     string
	Expected  string
	IsSample  bool
}

type Problem struct {
	ID            int64     `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Difficulty    string    `json:"difficulty"`
	TimeLimitMs   int       `json:"time_limit_ms"`
	MemoryLimitMb int       `json:"memory_limit_mb"`
	CreatedAt     time.Time `json:"created_at"`
}

type SampleTestCase struct {
	Input    string `json:"input"`
	Expected string `json:"expected"`
}

// ProblemResponse is safe to expose to clients because it only includes sample test cases.
type ProblemResponse struct {
	ID              int64            `json:"id"`
	Title           string           `json:"title"`
	Description     string           `json:"description"`
	Difficulty      string           `json:"difficulty"`
	TimeLimitMs     int              `json:"time_limit_ms"`
	MemoryLimitMb   int              `json:"memory_limit_mb"`
	CreatedAt       time.Time        `json:"created_at"`
	SampleTestCases []SampleTestCase `json:"sample_test_cases"`
}

func NewProblemResponse(problem *Problem, samples []TestCase) *ProblemResponse {
	response := &ProblemResponse{
		ID:            problem.ID,
		Title:         problem.Title,
		Description:   problem.Description,
		Difficulty:    problem.Difficulty,
		TimeLimitMs:   problem.TimeLimitMs,
		MemoryLimitMb: problem.MemoryLimitMb,
		CreatedAt:     problem.CreatedAt,
	}

	for _, sample := range samples {
		response.SampleTestCases = append(response.SampleTestCases, SampleTestCase{
			Input:    sample.Input,
			Expected: sample.Expected,
		})
	}

	return response
}
