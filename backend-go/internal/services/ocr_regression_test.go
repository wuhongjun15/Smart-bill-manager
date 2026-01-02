package services

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type regressionSample struct {
	Kind     string          `json:"kind"`
	Name     string          `json:"name"`
	RawText  string          `json:"raw_text"`
	Expected json.RawMessage `json:"expected"`
}

type paymentExpected struct {
	Amount          *float64 `json:"amount,omitempty"`
	Merchant        *string  `json:"merchant,omitempty"`
	TransactionTime *string  `json:"transaction_time,omitempty"`
	PaymentMethod   *string  `json:"payment_method,omitempty"`
	OrderNumber     *string  `json:"order_number,omitempty"`
}

type invoiceExpected struct {
	InvoiceNumber *string  `json:"invoice_number,omitempty"`
	InvoiceDate   *string  `json:"invoice_date,omitempty"`
	Amount        *float64 `json:"amount,omitempty"`
	TaxAmount     *float64 `json:"tax_amount,omitempty"`
	SellerName    *string  `json:"seller_name,omitempty"`
	BuyerName     *string  `json:"buyer_name,omitempty"`
}

func TestRegressionSamples(t *testing.T) {
	root := filepath.Join("testdata", "regression")
	if _, err := os.Stat(root); err != nil {
		t.Skip("no regression samples found")
		return
	}

	var files []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk regression samples: %v", err)
	}
	if len(files) == 0 {
		t.Skip("no regression samples found")
		return
	}

	svc := NewOCRService()

	for _, path := range files {
		t.Run(path, func(t *testing.T) {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read sample: %v", err)
			}

			var s regressionSample
			if err := json.Unmarshal(b, &s); err != nil {
				t.Fatalf("unmarshal sample: %v", err)
			}
			if strings.TrimSpace(s.Kind) == "" || strings.TrimSpace(s.Name) == "" {
				t.Fatalf("invalid sample metadata (kind/name required)")
			}
			if strings.TrimSpace(s.RawText) == "" {
				t.Fatalf("invalid sample (raw_text required)")
			}

			switch s.Kind {
			case "payment_screenshot":
				var exp paymentExpected
				if err := json.Unmarshal(s.Expected, &exp); err != nil {
					t.Fatalf("unmarshal expected: %v", err)
				}
				got, err := svc.ParsePaymentScreenshot(s.RawText)
				if err != nil {
					t.Fatalf("parse payment screenshot: %v", err)
				}
				if diffs := diffPaymentExpected(exp, got); len(diffs) > 0 {
					t.Fatalf("regression mismatch\nsample=%s\nkind=%s name=%s\n%s\nraw_text(head):\n%s",
						path,
						s.Kind,
						s.Name,
						strings.Join(diffs, "\n"),
						headLines(s.RawText, 12, 1200),
					)
				}
			case "invoice":
				var exp invoiceExpected
				if err := json.Unmarshal(s.Expected, &exp); err != nil {
					t.Fatalf("unmarshal expected: %v", err)
				}
				got, err := svc.ParseInvoiceData(s.RawText)
				if err != nil {
					t.Fatalf("parse invoice: %v", err)
				}
				if diffs := diffInvoiceExpected(exp, got); len(diffs) > 0 {
					t.Fatalf("regression mismatch\nsample=%s\nkind=%s name=%s\n%s\nraw_text(head):\n%s",
						path,
						s.Kind,
						s.Name,
						strings.Join(diffs, "\n"),
						headLines(s.RawText, 12, 1200),
					)
				}
			default:
				t.Fatalf("unknown kind: %s", s.Kind)
			}
		})
	}
}

func diffPaymentExpected(exp paymentExpected, got *PaymentExtractedData) []string {
	if got == nil {
		return []string{"diff: parsed payment is nil"}
	}

	diffs := make([]string, 0, 8)

	if exp.Merchant != nil {
		if got.Merchant == nil {
			diffs = append(diffs, fmt.Sprintf("diff: merchant expected=%q got=<nil>", strings.TrimSpace(*exp.Merchant)))
		} else if strings.TrimSpace(*got.Merchant) != strings.TrimSpace(*exp.Merchant) {
			diffs = append(diffs, fmt.Sprintf("diff: merchant expected=%q got=%q", strings.TrimSpace(*exp.Merchant), strings.TrimSpace(*got.Merchant)))
		}
	}

	if exp.TransactionTime != nil {
		if got.TransactionTime == nil {
			diffs = append(diffs, fmt.Sprintf("diff: transaction_time expected=%q got=<nil>", strings.TrimSpace(*exp.TransactionTime)))
		} else {
			expT, expErr := parseAnyPaymentTimeToUTC(*exp.TransactionTime)
			gotT, gotErr := parseAnyPaymentTimeToUTC(*got.TransactionTime)
			if expErr == nil && gotErr == nil {
				if !expT.Equal(gotT) {
					diffs = append(diffs, fmt.Sprintf(
						"diff: transaction_time expected(utc)=%s got(utc)=%s (raw expected=%q got=%q)",
						expT.Format(time.RFC3339),
						gotT.Format(time.RFC3339),
						strings.TrimSpace(*exp.TransactionTime),
						strings.TrimSpace(*got.TransactionTime),
					))
				}
			} else if strings.TrimSpace(*got.TransactionTime) != strings.TrimSpace(*exp.TransactionTime) {
				diffs = append(diffs, fmt.Sprintf("diff: transaction_time expected=%q got=%q", strings.TrimSpace(*exp.TransactionTime), strings.TrimSpace(*got.TransactionTime)))
			}
		}
	}

	if exp.PaymentMethod != nil {
		if got.PaymentMethod == nil {
			diffs = append(diffs, fmt.Sprintf("diff: payment_method expected=%q got=<nil>", strings.TrimSpace(*exp.PaymentMethod)))
		} else if strings.TrimSpace(*got.PaymentMethod) != strings.TrimSpace(*exp.PaymentMethod) {
			diffs = append(diffs, fmt.Sprintf("diff: payment_method expected=%q got=%q", strings.TrimSpace(*exp.PaymentMethod), strings.TrimSpace(*got.PaymentMethod)))
		}
	}

	if exp.OrderNumber != nil {
		if got.OrderNumber == nil {
			diffs = append(diffs, fmt.Sprintf("diff: order_number expected=%q got=<nil>", strings.TrimSpace(*exp.OrderNumber)))
		} else if strings.TrimSpace(*got.OrderNumber) != strings.TrimSpace(*exp.OrderNumber) {
			diffs = append(diffs, fmt.Sprintf("diff: order_number expected=%q got=%q", strings.TrimSpace(*exp.OrderNumber), strings.TrimSpace(*got.OrderNumber)))
		}
	}

	if exp.Amount != nil {
		if got.Amount == nil {
			diffs = append(diffs, fmt.Sprintf("diff: amount expected=%v got=<nil>", *exp.Amount))
		} else {
			expV := math.Abs(*exp.Amount)
			gotV := math.Abs(*got.Amount)
			if !approxEqMoney(expV, gotV) {
				diffs = append(diffs, fmt.Sprintf("diff: amount expected≈%v got=%v", expV, gotV))
			}
		}
	}

	return diffs
}

func diffInvoiceExpected(exp invoiceExpected, got *InvoiceExtractedData) []string {
	if got == nil {
		return []string{"diff: parsed invoice is nil"}
	}

	diffs := make([]string, 0, 8)

	if exp.InvoiceNumber != nil {
		if got.InvoiceNumber == nil {
			diffs = append(diffs, fmt.Sprintf("diff: invoice_number expected=%q got=<nil>", strings.TrimSpace(*exp.InvoiceNumber)))
		} else if strings.TrimSpace(*got.InvoiceNumber) != strings.TrimSpace(*exp.InvoiceNumber) {
			diffs = append(diffs, fmt.Sprintf("diff: invoice_number expected=%q got=%q", strings.TrimSpace(*exp.InvoiceNumber), strings.TrimSpace(*got.InvoiceNumber)))
		}
	}

	if exp.InvoiceDate != nil {
		if got.InvoiceDate == nil {
			diffs = append(diffs, fmt.Sprintf("diff: invoice_date expected=%q got=<nil>", strings.TrimSpace(*exp.InvoiceDate)))
		} else {
			expD, expErr := normalizeAnyInvoiceDate(*exp.InvoiceDate)
			gotD, gotErr := normalizeAnyInvoiceDate(*got.InvoiceDate)
			if expErr == nil && gotErr == nil {
				if expD != gotD {
					diffs = append(diffs, fmt.Sprintf(
						"diff: invoice_date expected=%q got=%q (raw expected=%q got=%q)",
						expD,
						gotD,
						strings.TrimSpace(*exp.InvoiceDate),
						strings.TrimSpace(*got.InvoiceDate),
					))
				}
			} else if strings.TrimSpace(*got.InvoiceDate) != strings.TrimSpace(*exp.InvoiceDate) {
				diffs = append(diffs, fmt.Sprintf("diff: invoice_date expected=%q got=%q", strings.TrimSpace(*exp.InvoiceDate), strings.TrimSpace(*got.InvoiceDate)))
			}
		}
	}

	if exp.SellerName != nil {
		if got.SellerName == nil {
			diffs = append(diffs, fmt.Sprintf("diff: seller_name expected=%q got=<nil>", strings.TrimSpace(*exp.SellerName)))
		} else if strings.TrimSpace(*got.SellerName) != strings.TrimSpace(*exp.SellerName) {
			diffs = append(diffs, fmt.Sprintf("diff: seller_name expected=%q got=%q", strings.TrimSpace(*exp.SellerName), strings.TrimSpace(*got.SellerName)))
		}
	}

	if exp.BuyerName != nil {
		if got.BuyerName == nil {
			diffs = append(diffs, fmt.Sprintf("diff: buyer_name expected=%q got=<nil>", strings.TrimSpace(*exp.BuyerName)))
		} else if strings.TrimSpace(*got.BuyerName) != strings.TrimSpace(*exp.BuyerName) {
			diffs = append(diffs, fmt.Sprintf("diff: buyer_name expected=%q got=%q", strings.TrimSpace(*exp.BuyerName), strings.TrimSpace(*got.BuyerName)))
		}
	}

	if exp.Amount != nil {
		if got.Amount == nil {
			diffs = append(diffs, fmt.Sprintf("diff: amount expected=%v got=<nil>", *exp.Amount))
		} else if !approxEqMoney(*exp.Amount, *got.Amount) {
			diffs = append(diffs, fmt.Sprintf("diff: amount expected=%v got=%v", *exp.Amount, *got.Amount))
		}
	}

	if exp.TaxAmount != nil {
		if got.TaxAmount == nil {
			diffs = append(diffs, fmt.Sprintf("diff: tax_amount expected=%v got=<nil>", *exp.TaxAmount))
		} else if !approxEqMoney(*exp.TaxAmount, *got.TaxAmount) {
			diffs = append(diffs, fmt.Sprintf("diff: tax_amount expected=%v got=%v", *exp.TaxAmount, *got.TaxAmount))
		}
	}

	return diffs
}

func approxEqMoney(a float64, b float64) bool {
	return math.Abs(a-b) <= 0.01
}

func parseAnyPaymentTimeToUTC(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Truncate(time.Second), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC().Truncate(time.Second), nil
	}
	loc := loadLocationOrUTC("Asia/Shanghai")
	if t, err := parsePaymentTimeToUTC(s, loc); err == nil {
		return t.UTC().Truncate(time.Second), nil
	}
	return time.Time{}, fmt.Errorf("unsupported time format: %q", s)
}

func normalizeAnyInvoiceDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty date")
	}
	if len(s) == 10 && s[4] == '-' && s[7] == '-' {
		return s, nil
	}

	parts := make([]string, 0, 3)
	var cur strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			cur.WriteRune(r)
			continue
		}
		if cur.Len() > 0 {
			parts = append(parts, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}

	// YYYYMMDD
	if len(parts) == 1 && len(parts[0]) == 8 {
		ds := parts[0]
		return fmt.Sprintf("%s-%s-%s", ds[0:4], ds[4:6], ds[6:8]), nil
	}

	if len(parts) >= 3 && len(parts[0]) == 4 {
		yyyy := parts[0]
		mm := parts[1]
		dd := parts[2]
		if len(mm) == 1 {
			mm = "0" + mm
		}
		if len(dd) == 1 {
			dd = "0" + dd
		}
		return fmt.Sprintf("%s-%s-%s", yyyy, mm, dd), nil
	}

	return "", fmt.Errorf("unsupported date format: %q", s)
}

func headLines(s string, maxLines int, maxChars int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	out := strings.Join(lines, "\n")
	out = strings.TrimSpace(out)
	if maxChars > 0 && len(out) > maxChars {
		return out[:maxChars] + "\n...(truncated)"
	}
	return out
}
