package engine

import (
	"regexp"
	"strings"

	"github.com/bitravens/paravizor/v1/internal/items"
	"github.com/bitravens/paravizor/v1/internal/project"
)

type ScopeEngine struct {
	exactIncludes    map[string]bool
	exactExcludes    map[string]bool
	wildcardIncludes []string // e.g., "*.example.com" -> ".example.com"
	wildcardExcludes []string
	regexIncludes    []*regexp.Regexp
	regexExcludes    []*regexp.Regexp
}

func NewScopeEngine(cfg project.ScopeConfig) *ScopeEngine {
	e := &ScopeEngine{
		exactIncludes: make(map[string]bool),
		exactExcludes: make(map[string]bool),
	}

	for _, p := range cfg.Include {
		e.addRule(p, true)
	}
	for _, p := range cfg.Exclude {
		e.addRule(p, false)
	}

	return e
}

func (e *ScopeEngine) addRule(pattern string, include bool) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return
	}

	if strings.HasPrefix(pattern, "^") || strings.HasSuffix(pattern, "$") {
		// Treat as regex
		if re, err := regexp.Compile(pattern); err == nil {
			if include {
				e.regexIncludes = append(e.regexIncludes, re)
			} else {
				e.regexExcludes = append(e.regexExcludes, re)
			}
		}
		return
	}

	if strings.HasPrefix(pattern, "*.") {
		// Wildcard domain
		suffix := strings.TrimPrefix(pattern, "*")
		if include {
			e.wildcardIncludes = append(e.wildcardIncludes, suffix)
		} else {
			e.wildcardExcludes = append(e.wildcardExcludes, suffix)
		}
		return
	}

	// Exact match
	if include {
		e.exactIncludes[pattern] = true
	} else {
		e.exactExcludes[pattern] = true
	}
}

// IsInScope checks if an item matches the scope rules.
// Rules:
// 1. Exact exclude -> drop
// 2. Exact include -> keep
// 3. Regex exclude -> drop
// 4. Wildcard exclude -> drop
// 5. Regex include -> keep
// 6. Wildcard include -> keep
// 7. Default -> drop
func (e *ScopeEngine) IsInScope(item items.Item) bool {
	target := string(item.ScopeTarget())
	if target == "" {
		return false
	}

	// We might need to extract the domain part if the target is a URL.
	// For simplicity, we can just match the raw target string against the rules first.
	// In a complete implementation, URL items should have their domain part checked against domain rules.
	domainPart := target
	if strings.Contains(target, "://") {
		// Very naive domain extraction for wildcard matching
		parts := strings.Split(target, "://")
		if len(parts) > 1 {
			domainPart = strings.Split(parts[1], "/")[0]
			domainPart = strings.Split(domainPart, ":")[0]
		}
	}

	// 1. Exact exclude
	if e.exactExcludes[target] || e.exactExcludes[domainPart] {
		return false
	}

	// 2. Exact include
	if e.exactIncludes[target] || e.exactIncludes[domainPart] {
		return true
	}

	// 3. Regex exclude
	for _, re := range e.regexExcludes {
		if re.MatchString(target) {
			return false
		}
	}

	// 4. Wildcard exclude
	for _, suffix := range e.wildcardExcludes {
		if strings.HasSuffix(target, suffix) || strings.HasSuffix(domainPart, suffix) {
			return false
		}
	}

	// 5. Regex include
	for _, re := range e.regexIncludes {
		if re.MatchString(target) {
			return true
		}
	}

	// 6. Wildcard include
	for _, suffix := range e.wildcardIncludes {
		if strings.HasSuffix(target, suffix) || strings.HasSuffix(domainPart, suffix) {
			return true
		}
	}

	// Default deny
	// If the scope config is completely empty, should we default allow?
	// The spec says "Default deny when no scope rule matches"
	if len(e.exactIncludes) == 0 && len(e.wildcardIncludes) == 0 && len(e.regexIncludes) == 0 {
		// If NO includes are defined at all, maybe we allow all unless excluded?
		// But let's strictly follow default deny for now. Actually, if no includes are defined,
		// everything would be dropped. The prompt says "default deny when no scope rule matches".
		return false
	}

	return false
}
