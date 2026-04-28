package problems

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

type ProblemRepository struct {
	db *pgxpool.Pool
}

func NewProblemRepository(db *pgxpool.Pool) *ProblemRepository {
	return &ProblemRepository{db: db}
}

func (r *ProblemRepository) ListProblems(ctx context.Context) ([]Problem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, title, description, difficulty, time_limit_ms, memory_limit_mb, created_at
		FROM problems
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	problems := make([]Problem, 0)
	for rows.Next() {
		var problem Problem
		if err := rows.Scan(
			&problem.ID,
			&problem.Title,
			&problem.Description,
			&problem.Difficulty,
			&problem.TimeLimitMs,
			&problem.MemoryLimitMb,
			&problem.CreatedAt,
		); err != nil {
			return nil, err
		}
		problems = append(problems, problem)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return problems, nil
}

func (r *ProblemRepository) GetProblemByID(ctx context.Context, id int64) (*Problem, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, title, description, difficulty, time_limit_ms, memory_limit_mb, created_at
		FROM problems
		WHERE id = $1
	`, id)

	var problem Problem
	if err := row.Scan(
		&problem.ID,
		&problem.Title,
		&problem.Description,
		&problem.Difficulty,
		&problem.TimeLimitMs,
		&problem.MemoryLimitMb,
		&problem.CreatedAt,
	); err != nil {
		return nil, err
	}

	return &problem, nil
}

func (r *ProblemRepository) GetRandomProblemByDifficulty(ctx context.Context, difficulty string) (*Problem, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, title, description, difficulty, time_limit_ms, memory_limit_mb, created_at
		FROM problems
		WHERE difficulty = $1
		ORDER BY RANDOM()
		LIMIT 1
	`, difficulty)

	var problem Problem
	if err := row.Scan(
		&problem.ID,
		&problem.Title,
		&problem.Description,
		&problem.Difficulty,
		&problem.TimeLimitMs,
		&problem.MemoryLimitMb,
		&problem.CreatedAt,
	); err != nil {
		return nil, err
	}

	return &problem, nil
}

func (r *ProblemRepository) GetSampleTestCasesByProblemID(ctx context.Context, problemID int64) ([]TestCase, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, problem_id, input, expected, is_sample
		FROM test_cases
		WHERE problem_id = $1 AND is_sample = true
		ORDER BY id ASC
	`, problemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	testCases := make([]TestCase, 0)
	for rows.Next() {
		var tc TestCase
		if err := rows.Scan(&tc.ID, &tc.ProblemID, &tc.Input, &tc.Expected, &tc.IsSample); err != nil {
			return nil, err
		}
		testCases = append(testCases, tc)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return testCases, nil
}

func (r *ProblemRepository) GetAllTestCasesByProblemID(ctx context.Context, problemID int64) ([]TestCase, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, problem_id, input, expected, is_sample
		FROM test_cases
		WHERE problem_id = $1
		ORDER BY id ASC
	`, problemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	testCases := make([]TestCase, 0)
	for rows.Next() {
		var tc TestCase
		if err := rows.Scan(&tc.ID, &tc.ProblemID, &tc.Input, &tc.Expected, &tc.IsSample); err != nil {
			return nil, err
		}
		testCases = append(testCases, tc)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return testCases, nil
}
