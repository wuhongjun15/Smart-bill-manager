package services

import "testing"

func TestItineraryInvoiceWhitelist(t *testing.T) {
	cases := []struct {
		name     string
		fn       string
		isItin   bool
		allowed  bool
		asInvoice bool
	}{
		{
			name:      "didi itinerary is not invoice",
			fn:        "滴滴出行行程报销单.pdf",
			isItin:    true,
			allowed:   false,
			asInvoice: false,
		},
		{
			name:      "air ticket itinerary is invoice",
			fn:        "航空运输电子客票行程单.pdf",
			isItin:    true,
			allowed:   true,
			asInvoice: true,
		},
		{
			name:      "generic itinerary with flight marker is invoice",
			fn:        "电子行程单_机票.pdf",
			isItin:    true,
			allowed:   true,
			asInvoice: true,
		},
		{
			name:      "generic itinerary with rail marker is invoice",
			fn:        "行程单_高铁.pdf",
			isItin:    true,
			allowed:   true,
			asInvoice: true,
		},
		{
			name:      "rail e-ticket is not itinerary but should parse as invoice",
			fn:        "电子发票(铁路电子客票).pdf",
			isItin:    false,
			allowed:   false,
			asInvoice: true,
		},
		{
			name:      "other pdf should still be attempted as invoice (multi-invoice emails)",
			fn:        "other.pdf",
			isItin:    false,
			allowed:   false,
			asInvoice: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isItineraryPDFName(tc.fn); got != tc.isItin {
				t.Fatalf("isItineraryPDFName(%q)=%v, want %v", tc.fn, got, tc.isItin)
			}
			if got := isAllowedInvoiceItineraryPDFName(tc.fn); got != tc.allowed {
				t.Fatalf("isAllowedInvoiceItineraryPDFName(%q)=%v, want %v", tc.fn, got, tc.allowed)
			}
			if got := shouldParseExtraPDFAsInvoice(tc.fn); got != tc.asInvoice {
				t.Fatalf("shouldParseExtraPDFAsInvoice(%q)=%v, want %v", tc.fn, got, tc.asInvoice)
			}
		})
	}
}

