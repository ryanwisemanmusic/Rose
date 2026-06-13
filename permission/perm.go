package permission

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

type State int

const (
	Unset State = iota
	AllowedOnce
	AllowedSession
	Denied
)

type Rule struct {
	Resource string
	State    State
	Label    string
}

type Manager struct {
	mu          sync.RWMutex
	rules       []Rule
	projectRoot string
}

func NewManager(projectRoot string) *Manager {
	return &Manager{
		projectRoot: projectRoot,
	}
}

func (pm *Manager) CheckAccess(path string) (allowed bool, reason string) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	abs, err := filepath.Abs(path)
	if err != nil {
		return false, "invalid path"
	}

	if !pm.contains(abs) {
		for _, rule := range pm.rules {
			if rule.Resource == abs && rule.State == AllowedSession {
				return true, ""
			}
		}
		return false, fmt.Sprintf("outside project boundary (%s)", pm.projectRoot)
	}

	rel, _ := filepath.Rel(pm.projectRoot, abs)
	if strings.HasPrefix(rel, "..") {
		return false, "path traversal detected"
	}

	return true, ""
}

func (pm *Manager) RequestAccess(path string) (needsPrompt bool, label string) {
	abs, _ := filepath.Abs(path)
	label = fmt.Sprintf("access %s", abs)

	if !pm.contains(abs) {
		return true, label
	}
	return false, ""
}

func (pm *Manager) Grant(path string, session bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	abs, _ := filepath.Abs(path)
	state := AllowedOnce
	if session {
		state = AllowedSession
	}

	for i, rule := range pm.rules {
		if rule.Resource == abs {
			pm.rules[i].State = state
			return
		}
	}

	pm.rules = append(pm.rules, Rule{
		Resource: abs,
		State:    state,
		Label:    fmt.Sprintf("access %s", filepath.Base(abs)),
	})
}

func (pm *Manager) Deny(path string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	abs, _ := filepath.Abs(path)
	for i, rule := range pm.rules {
		if rule.Resource == abs {
			pm.rules[i].State = Denied
			return
		}
	}
	pm.rules = append(pm.rules, Rule{
		Resource: abs,
		State:    Denied,
		Label:    fmt.Sprintf("deny %s", filepath.Base(abs)),
	})
}

func (pm *Manager) PendingRequests() []Rule {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var pending []Rule
	for _, r := range pm.rules {
		if r.State == Unset {
			pending = append(pending, r)
		}
	}
	return pending
}

func (pm *Manager) IsAllowedOnce(path string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	abs, _ := filepath.Abs(path)
	for _, r := range pm.rules {
		if r.Resource == abs && r.State == AllowedOnce {
			return true
		}
	}
	return false
}

func (pm *Manager) ConsumeOnce(path string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	abs, _ := filepath.Abs(path)
	for i, r := range pm.rules {
		if r.Resource == abs && r.State == AllowedOnce {
			pm.rules = append(pm.rules[:i], pm.rules[i+1:]...)
			return
		}
	}
}

func (pm *Manager) contains(path string) bool {
	root, err := filepath.Abs(pm.projectRoot)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}
