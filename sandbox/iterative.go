package sandbox

import (
	"fmt"
	"strings"
	"time"
)

type IterationResult struct {
	Code      string
	Language  string
	Attempts  int
	Success   bool
	Results   []*Result
	ErrorLog  string
	Duration  time.Duration
}

func (e *Executor) RunIterative(initialCode, language string, maxAttempts int, fixFunc func(code, stderr string, attempt int) (string, error)) (*IterationResult, error) {
	start := time.Now()
	result := &IterationResult{
		Code:     initialCode,
		Language: language,
		Attempts: 0,
	}

	currentCode := initialCode

	for attempt := 0; attempt < maxAttempts; attempt++ {
		result.Attempts++

		runResult, err := e.RunShell(currentCode, language)
		if err != nil {
			runResult = &Result{
				Stderr:   err.Error(),
				ExitCode: -1,
			}
		}
		result.Results = append(result.Results, runResult)

		if runResult.ExitCode == 0 {
			result.Success = true
			result.Code = currentCode
			result.Duration = time.Since(start)
			return result, nil
		}

		errorLog := runResult.Stderr
		if errorLog == "" {
			errorLog = runResult.Stdout
		}
		if result.ErrorLog != "" {
			result.ErrorLog += "\n---\n"
		}
		result.ErrorLog += fmt.Sprintf("Attempt %d (exit %d):\n%s", attempt+1, runResult.ExitCode, errorLog)

		if fixFunc != nil {
			newCode, err := fixFunc(currentCode, errorLog, attempt)
			if err != nil {
				return result, fmt.Errorf("fix function error: %w", err)
			}
			if strings.TrimSpace(newCode) == strings.TrimSpace(currentCode) {
				return result, fmt.Errorf("fix produced identical code, stopping")
			}
			currentCode = newCode
			e.Cleanup()
			e2, err := NewExecutor(30)
			if err != nil {
				return result, err
			}
			*e = *e2
		} else {
			break
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

func (r *IterationResult) Summary() string {
	if r.Success {
		return fmt.Sprintf("✓ solved in %d attempts (%s)", r.Attempts, r.Duration)
	}
	return fmt.Sprintf("✗ failed after %d attempts (%s): %s", r.Attempts, r.Duration, truncate(r.ErrorLog, 200))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
