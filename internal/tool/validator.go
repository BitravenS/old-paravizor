package tool

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// validItemTypes is the set of item type strings accepted by the engine.
var validItemTypes = map[string]bool{
	"domain":     true,
	"url":        true,
	"ip":         true,
	"port":       true,
	"dns_record": true,
	"finding":    true,
	"file":       true,
}

// ValidateTool performs semantic validation of a ToolConfig beyond struct tags.
// It checks:
//   - consumes/produces are known item types
//   - input delivery is coherent (arg/stdin/file/bulk combos)
//   - output regex format: pattern has named groups and fields map to them
//
// Returns a joined non-nil error if any check fails, nil otherwise.
func ValidateTool(def *ToolConfig) error {
	var errs []error

	// --- Item types ---
	if !validItemTypes[def.Consumes] {
		errs = append(errs, fmt.Errorf("consumes: unknown item type %q (valid: %s)", def.Consumes, itemTypeList()))
	}
	if !validItemTypes[def.Produces] {
		errs = append(errs, fmt.Errorf("produces: unknown item type %q (valid: %s)", def.Produces, itemTypeList()))
	}

	// --- Input delivery coherence ---
	errs = append(errs, validateInput(def)...)

	// --- Output parsing coherence ---
	errs = append(errs, validateOutput(def)...)

	if len(errs) == 0 {
		return nil
	}
	// Prefix every error with the tool name so callers get useful context.
	var named []error
	for _, e := range errs {
		named = append(named, fmt.Errorf("tool %q: %w", def.Name, e))
	}
	return errors.Join(named...)
}

// validateInput checks that the input configuration can actually deliver values
// to the tool binary.
func validateInput(def *ToolConfig) []error {
	var errs []error
	inp := def.Input

	switch inp.Type {
	case "arg":
		// Single-item path: flag is optional (positional args are fine).
		// Bulk path: must declare a bulk delivery method.
		if inp.Bulk.Type == "" {
			// Only single-item delivery available — acceptable.
		} else if inp.Bulk.Type != "file" && inp.Bulk.Type != "stdin" {
			errs = append(errs, fmt.Errorf("input.bulk.type %q is not valid (use stdin or file)", inp.Bulk.Type))
		}
	case "stdin":
		// Nothing extra required; separator defaults to newline.
	case "file":
		if inp.Flag == "" {
			errs = append(errs, fmt.Errorf("input.type is file but input.flag is empty"))
		}
	case "none":
		// Tool takes no input (e.g. asset discovery seeded internally). Valid.
	}

	return errs
}

// validateOutput checks that output parsing config is internally consistent.
func validateOutput(def *ToolConfig) []error {
	var errs []error
	out := def.Output

	switch out.Format {
	case "regex":
		if out.Pattern == "" {
			errs = append(errs, fmt.Errorf("output.format is regex but output.pattern is empty"))
			break
		}
		re, err := regexp.Compile(out.Pattern)
		if err != nil {
			errs = append(errs, fmt.Errorf("output.pattern is not a valid regex: %w", err))
			break
		}
		// Build set of named capture groups in the pattern.
		groups := make(map[string]bool)
		for _, name := range re.SubexpNames() {
			if name != "" {
				groups[name] = true
			}
		}
		if len(groups) == 0 {
			errs = append(errs, fmt.Errorf("output.pattern has no named capture groups (use (?P<name>...) syntax)"))
		}
		// Every declared field must map to an existing named group.
		for fieldKey, groupRef := range out.Fields {
			groupName := strings.TrimPrefix(groupRef, "$.")
			if !groups[groupName] {
				errs = append(errs, fmt.Errorf("output.fields[%q] references capture group %q which is not in the pattern", fieldKey, groupName))
			}
		}

	case "json", "jsonl", "csv":
		// If fields are declared, no further structural check needed here —
		// missing fields at parse time just silently produce empty values,
		// which is acceptable for optional enrichment.

	case "line":
		// Line format is always valid as long as produces is set (checked above).
		if len(out.Fields) > 0 {
			errs = append(errs, fmt.Errorf("output.format is line but output.fields is set (fields are only used for structured formats)"))
		}

	case "xml":
		// XML parsing is format-valid; no extra checks here.
	}

	// File output requires a flag and path.
	if out.Type == "file" {
		if out.Flag == "" {
			errs = append(errs, fmt.Errorf("output.type is file but output.flag is empty"))
		}
		if out.Path == "" {
			errs = append(errs, fmt.Errorf("output.type is file but output.path is empty"))
		}
	}

	return errs
}

func itemTypeList() string {
	return "domain, url, ip, port, dns_record, finding, file"
}
