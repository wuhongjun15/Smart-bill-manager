package services

import (
	"testing"
	"time"

	"smart-bill-manager/internal/models"
)

func TestPickSuggestedInvoices_RespectsMinScore_NoFallback(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	p := &models.Payment{
		ID:              "p1",
		Amount:          1000,
		TransactionTime: now.Format(time.RFC3339),
	}

	// This invoice has no amount and a far date => score stays below minScore(0.15).
	distant := now.AddDate(0, 0, -14).Format("2006-01-02")
	inv := models.Invoice{
		ID:          "i1",
		Amount:      nil,
		InvoiceDate: &distant,
		CreatedAt:   now,
	}

	scored := scoreInvoiceCandidates(p, []models.Invoice{inv}, map[string]struct{}{})
	out := pickSuggestedInvoices(p, scored, 10)
	if len(out) != 0 {
		t.Fatalf("expected 0 suggestions, got %d", len(out))
	}
}

func TestPickSuggestedInvoices_AmountMissing_AllowsMerchantDateSignals(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	merchant := "Apple Store"
	seller := "Apple Store"

	p := &models.Payment{
		ID:              "p1",
		Amount:          0,
		Merchant:        &merchant,
		TransactionTime: now.Format(time.RFC3339),
	}

	date := now.Format("2006-01-02")
	inv := models.Invoice{
		ID:          "i1",
		SellerName:  &seller,
		InvoiceDate: &date,
		CreatedAt:   now,
	}

	scored := scoreInvoiceCandidates(p, []models.Invoice{inv}, map[string]struct{}{})
	out := pickSuggestedInvoices(p, scored, 10)
	if len(out) != 1 || out[0].ID != "i1" {
		t.Fatalf("expected 1 suggestion i1, got %+v", out)
	}
}

func TestScoreInvoiceCandidates_ExcludesLinkedInvoices(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	p := &models.Payment{ID: "p1", Amount: 1000, TransactionTime: now.Format(time.RFC3339)}

	a1 := 1000.0
	a2 := 1000.0
	inv1 := models.Invoice{ID: "i1", Amount: &a1, CreatedAt: now.Add(-time.Minute)}
	inv2 := models.Invoice{ID: "i2", Amount: &a2, CreatedAt: now}

	linked := map[string]struct{}{"i1": {}}
	scored := scoreInvoiceCandidates(p, []models.Invoice{inv1, inv2}, linked)
	if len(scored) != 1 || scored[0].invoice.ID != "i2" {
		t.Fatalf("expected only i2 scored, got %+v", scored)
	}
}
