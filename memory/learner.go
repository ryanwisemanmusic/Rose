package memory

import (
	"fmt"
	"strings"
)

type Learner struct {
	store *Store
}

func NewLearner(store *Store) *Learner {
	return &Learner{store: store}
}

type LearningContext struct {
	PastSuccesses []string
	PastErrors    []string
	Patterns      []string
}

func (l *Learner) BuildContext(prompt string) *LearningContext {
	ctx := &LearningContext{}
	if l == nil || l.store == nil {
		return ctx
	}

	related, err := l.store.FindSimilar(prompt, 5)
	if err != nil || len(related) == 0 {
		return ctx
	}

	for _, exp := range related {
		if exp.Success {
			ctx.PastSuccesses = append(ctx.PastSuccesses,
				fmt.Sprintf("Previously solved similar problem using %s:\n%s",
					exp.Language, truncate(exp.Response, 300)))
		} else {
			ctx.PastErrors = append(ctx.PastErrors,
				fmt.Sprintf("Previously failed with exit code %d in %s:\nstderr: %s",
					exp.ExitCode, exp.Language, truncate(exp.Stderr, 200)))
		}
	}

	return ctx
}

func (l *Learner) GetPatterns(language string) []string {
	if l == nil || l.store == nil {
		return nil
	}

	successes, err := l.store.FindByExitCode(0, 10)
	if err != nil {
		return nil
	}

	var patterns []string
	for _, s := range successes {
		if s.Language == language || language == "" {
			patterns = append(patterns, s.Prompt+": "+truncate(s.Response, 100))
		}
	}
	return patterns
}

func (l *Learner) BuildPrompt(prompt, language string) string {
	ctx := l.BuildContext(prompt)
	patterns := l.GetPatterns(language)

	var parts []string
	parts = append(parts, prompt)

	if len(ctx.PastSuccesses) > 0 {
		parts = append(parts,
			"\n[Relevant past experiences with similar problems:]")
		for _, s := range ctx.PastSuccesses {
			parts = append(parts, s)
		}
	}

	if len(ctx.PastErrors) > 0 {
		parts = append(parts,
			"\n[Previously seen errors to avoid:]")
		for _, e := range ctx.PastErrors {
			parts = append(parts, e)
		}
	}

	if len(patterns) > 0 {
		parts = append(parts,
			fmt.Sprintf("\n[Known working patterns in %s:]", language))
		for _, p := range patterns {
			parts = append(parts, p)
		}
	}

	return strings.Join(parts, "\n")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
