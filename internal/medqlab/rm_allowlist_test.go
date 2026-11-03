package medqlab

import "testing"

func TestRMInAllowlist(t *testing.T) {
	t.Parallel()

	if !rmInAllowlist("000022", nil) {
		t.Fatal("empty allowlist must accept any RM")
	}
	if !rmInAllowlist("000022", []string{}) {
		t.Fatal("empty slice must accept any RM")
	}
	if !rmInAllowlist("000022", []string{"000022", "000023"}) {
		t.Fatal("expected 000022 to be allowed")
	}
	if rmInAllowlist("000099", []string{"000022", "000023"}) {
		t.Fatal("expected 000099 to be refused")
	}
	if rmInAllowlist("000022 ", []string{"000022"}) {
		t.Fatal("exact match required (caller trims RM)")
	}
}
