package runtime

import "testing"

func TestNormalizeScopeTargetAcceptsLeadingStarWithoutDot(t *testing.T) {
	scopeType, normalized, expanded := normalizeScopeTarget("*insat.rnu.tn")
	if scopeType != "wildcard" {
		t.Fatalf("scopeType = %q, want wildcard", scopeType)
	}
	if normalized != "insat.rnu.tn" {
		t.Fatalf("normalized = %q, want insat.rnu.tn", normalized)
	}
	if len(expanded) != 1 || expanded[0] != "insat.rnu.tn" {
		t.Fatalf("expanded = %#v, want [insat.rnu.tn]", expanded)
	}
}

func TestNormalizeScopeTargetAcceptsStandardWildcard(t *testing.T) {
	scopeType, normalized, expanded := normalizeScopeTarget("*.insat.rnu.tn")
	if scopeType != "wildcard" || normalized != "insat.rnu.tn" || len(expanded) != 1 || expanded[0] != "insat.rnu.tn" {
		t.Fatalf("got scopeType=%q normalized=%q expanded=%#v", scopeType, normalized, expanded)
	}
}
