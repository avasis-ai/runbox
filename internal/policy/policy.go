package policy

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Engine struct {
	requireApproval []string
	deny            []string
}

func NewEngine(requireApproval, deny []string) *Engine {
	return &Engine{
		requireApproval: requireApproval,
		deny:            deny,
	}
}

type Decision struct {
	Allowed  bool
	Reason   string
	Pattern  string
	Requires bool
}

func (e *Engine) Check(command string) Decision {
	for _, pattern := range e.deny {
		if matchPattern(pattern, command) {
			return Decision{
				Allowed:  false,
				Reason:   "denied by policy",
				Pattern:  pattern,
				Requires: false,
			}
		}
	}

	for _, pattern := range e.requireApproval {
		if matchPattern(pattern, command) {
			return Decision{
				Allowed:  true,
				Reason:   "requires approval",
				Pattern:  pattern,
				Requires: true,
			}
		}
	}

	return Decision{Allowed: true}
}

func matchPattern(pattern, command string) bool {
	matched, err := filepath.Match(pattern, command)
	if err != nil {
		return strings.Contains(command, strings.TrimSuffix(pattern, "*"))
	}
	return matched
}

func (d Decision) String() string {
	if !d.Allowed {
		return fmt.Sprintf("Denied: %q (matches policy: %s)", d.Reason, d.Pattern)
	}
	if d.Requires {
		return fmt.Sprintf("Approval required (matches policy: %s)", d.Pattern)
	}
	return ""
}
