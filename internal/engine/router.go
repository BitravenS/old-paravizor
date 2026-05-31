package engine

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/bitravens/paravizor/v1/internal/items"
)

// itemAttrs extracts a flat string-keyed attribute map from an item.
// Keys match common field names referenced in route conditions.
func itemAttrs(item items.Item) map[string]string {
	attrs := map[string]string{
		"type": string(item.Type()),
	}
	switch v := item.(type) {
	case *items.DomainItem:
		attrs["name"] = v.Name
		attrs["source"] = v.SourceName
	case items.DomainItem:
		attrs["name"] = v.Name
		attrs["source"] = v.SourceName
	case *items.URLItem:
		attrs["full_url"] = v.FullURL
		attrs["source"] = v.SourceName
	case items.URLItem:
		attrs["full_url"] = v.FullURL
		attrs["source"] = v.SourceName
	case *items.IPItem:
		attrs["address"] = v.Address
		attrs["source"] = v.SourceName
	case items.IPItem:
		attrs["address"] = v.Address
		attrs["source"] = v.SourceName
	case *items.PortItem:
		attrs["port"] = strconv.Itoa(v.Port)
		attrs["protocol"] = v.Protocol
		attrs["source"] = v.SourceName
	case items.PortItem:
		attrs["port"] = strconv.Itoa(v.Port)
		attrs["protocol"] = v.Protocol
		attrs["source"] = v.SourceName
	case *items.FindingItem:
		attrs["severity"] = v.Severity
		attrs["title"] = v.Title
		attrs["source"] = v.SourceName
	case items.FindingItem:
		attrs["severity"] = v.Severity
		attrs["title"] = v.Title
		attrs["source"] = v.SourceName
	case *items.FileItem:
		attrs["file_path"] = v.Path
		attrs["source"] = v.SourceName
	case items.FileItem:
		attrs["file_path"] = v.Path
		attrs["source"] = v.SourceName
	case *items.DNSRecordItem:
		attrs["record_type"] = v.RecordType
		attrs["value"] = v.RecordValue
		attrs["source"] = v.SourceName
	case items.DNSRecordItem:
		attrs["record_type"] = v.RecordType
		attrs["value"] = v.RecordValue
		attrs["source"] = v.SourceName
	}
	return attrs
}

// EvalCondition evaluates a route condition string against an item.
// Returns true if the condition matches (or if condition is empty).
//
// Supported syntax:
//   - empty / "true"            → always match
//   - `field == 'value'`
//   - `field != 'value'`
//   - `field > N`  / `field < N`
//   - `'value' in field`        → field contains value as substring / set member
//   - `field matches 'regex'`
//   - `is_wildcard == true`     → special: checks DomainItem name starts with "*."
//   - conditions joined by `and` / `or` (evaluated left-to-right, no precedence)
func EvalCondition(cond string, item items.Item) bool {
	cond = strings.TrimSpace(cond)
	if cond == "" || cond == "true" {
		return true
	}

	attrs := itemAttrs(item)

	// Handle 'or' (lowest precedence) by splitting first.
	orParts := splitByKeyword(cond, " or ")
	if len(orParts) > 1 {
		for _, part := range orParts {
			if EvalCondition(strings.TrimSpace(part), item) {
				return true
			}
		}
		return false
	}

	// Handle 'and'.
	andParts := splitByKeyword(cond, " and ")
	if len(andParts) > 1 {
		for _, part := range andParts {
			if !EvalCondition(strings.TrimSpace(part), item) {
				return false
			}
		}
		return true
	}

	// Single predicate.
	return evalPredicate(cond, attrs, item)
}

// evalPredicate evaluates a single predicate expression.
func evalPredicate(expr string, attrs map[string]string, item items.Item) bool {
	expr = strings.TrimSpace(expr)

	// 'value' in field
	if idx := strings.Index(expr, "' in "); idx != -1 {
		value := strings.Trim(expr[:idx], "' ")
		field := strings.TrimSpace(expr[idx+5:])
		if fieldVal, ok := attrs[field]; ok {
			return strings.Contains(fieldVal, value)
		}
		return false
	}

	// field matches 'regex'
	if strings.Contains(expr, " matches ") {
		parts := strings.SplitN(expr, " matches ", 2)
		field := strings.TrimSpace(parts[0])
		pattern := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
		fieldVal := attrs[field]
		matched, err := regexp.MatchString(pattern, fieldVal)
		if err != nil {
			return false
		}
		return matched
	}

	// is_wildcard == true / false (special semantic)
	if strings.HasPrefix(expr, "is_wildcard") {
		var name string
		switch v := item.(type) {
		case *items.DomainItem:
			name = v.Name
		case items.DomainItem:
			name = v.Name
		default:
			return false
		}
		isWildcard := strings.HasPrefix(name, "*.")
		rhs := strings.TrimSpace(strings.TrimPrefix(expr, "is_wildcard"))
		rhs = strings.TrimSpace(strings.TrimPrefix(rhs, "=="))
		rhs = strings.Trim(rhs, " '\"")
		want := rhs == "true"
		return isWildcard == want
	}

	// status_code in 200,206 (comma-separated integer set check)
	// (Note: URLItem no longer has StatusCode, but keeping for compatibility if attrs are added back)
	if strings.Contains(expr, " in ") {
		parts := strings.SplitN(expr, " in ", 2)
		field := strings.TrimSpace(parts[0])
		setStr := strings.TrimSpace(parts[1])
		fieldVal := attrs[field]
		for _, s := range strings.Split(setStr, ",") {
			if strings.TrimSpace(s) == fieldVal {
				return true
			}
		}
		return false
	}

	// field == 'value'
	if strings.Contains(expr, " == ") {
		parts := strings.SplitN(expr, " == ", 2)
		field := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
		return attrs[field] == value
	}

	// field != 'value'
	if strings.Contains(expr, " != ") {
		parts := strings.SplitN(expr, " != ", 2)
		field := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
		return attrs[field] != value
	}

	// field > N
	if strings.Contains(expr, " > ") {
		parts := strings.SplitN(expr, " > ", 2)
		field := strings.TrimSpace(parts[0])
		n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return false
		}
		v, err := strconv.Atoi(attrs[field])
		if err != nil {
			return false
		}
		return v > n
	}

	// field < N
	if strings.Contains(expr, " < ") {
		parts := strings.SplitN(expr, " < ", 2)
		field := strings.TrimSpace(parts[0])
		n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return false
		}
		v, err := strconv.Atoi(attrs[field])
		if err != nil {
			return false
		}
		return v < n
	}

	return false
}

// splitByKeyword splits s by the first occurrence of each keyword occurrence,
// preserving quoted strings. Simple implementation without full parser.
func splitByKeyword(s, kw string) []string {
	var parts []string
	remaining := s
	for {
		idx := strings.Index(remaining, kw)
		if idx == -1 {
			parts = append(parts, remaining)
			break
		}
		parts = append(parts, remaining[:idx])
		remaining = remaining[idx+len(kw):]
	}
	return parts
}
