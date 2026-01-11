package services

import "testing"

func TestShouldShortCircuitEmailLogParse(t *testing.T) {
	id := "inv123"
	blank := "   "

	if shouldShortCircuitEmailLogParse("error", &id) {
		t.Fatalf("expected false when status!=parsed")
	}
	if shouldShortCircuitEmailLogParse("parsed", nil) {
		t.Fatalf("expected false when id is nil")
	}
	if shouldShortCircuitEmailLogParse("parsed", &blank) {
		t.Fatalf("expected false when id is blank")
	}
	if !shouldShortCircuitEmailLogParse("parsed", &id) {
		t.Fatalf("expected true when status=parsed and id set")
	}
	// Case-insensitive status handling
	if !shouldShortCircuitEmailLogParse("PaRsEd", &id) {
		t.Fatalf("expected true when status is case-insensitive parsed")
	}
}

