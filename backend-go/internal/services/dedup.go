package services

import (
	"errors"
	"fmt"
	"time"
)

const (
	DedupStatusOK        = "ok"
	DedupStatusSuspected = "suspected_duplicate"
	DedupStatusForced    = "forced_saved"
)

type DedupCandidate struct {
	ID              string    `json:"id"`
	IsDraft         bool      `json:"is_draft"`
	Amount          *float64  `json:"amount,omitempty"`
	TransactionTime *string   `json:"transaction_time,omitempty"`
	Merchant        *string   `json:"merchant,omitempty"`
	InvoiceNumber   *string   `json:"invoice_number,omitempty"`
	InvoiceDate     *string   `json:"invoice_date,omitempty"`
	SellerName      *string   `json:"seller_name,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type DuplicateError struct {
	// Kind: "hash_duplicate" | "suspected_duplicate"
	Kind string `json:"kind"`
	// Reason: "file_sha256" | "amount_time" | "invoice_number"
	Reason string `json:"reason"`
	// Entity: "payment" | "invoice"
	Entity string `json:"entity"`

	ExistingID      string           `json:"existing_id,omitempty"`
	ExistingIsDraft bool             `json:"existing_is_draft,omitempty"`
	Candidates      []DedupCandidate `json:"candidates,omitempty"`
}

func (e *DuplicateError) Error() string {
	switch e.Kind {
	case "hash_duplicate":
		return fmt.Sprintf("%s hash duplicate: %s", e.Entity, e.ExistingID)
	case "suspected_duplicate":
		return fmt.Sprintf("%s suspected duplicate (%s)", e.Entity, e.Reason)
	default:
		return "duplicate detected"
	}
}

func AsDuplicateError(err error) (*DuplicateError, bool) {
	var de *DuplicateError
	if errors.As(err, &de) {
		return de, true
	}
	return nil, false
}
