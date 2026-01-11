package services

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/emersion/go-message/mail"
)

func TestExtractInvoicePDFAndBodyTextFromEmail_InlinePDF(t *testing.T) {
	pdfRaw := []byte("%PDF-1.4\n%test\n")
	pdfB64 := base64.StdEncoding.EncodeToString(pdfRaw)

	raw := strings.Join([]string{
		"From: test@example.com",
		"To: you@example.com",
		"Subject: test",
		"MIME-Version: 1.0",
		"Content-Type: multipart/mixed; boundary=\"abc\"",
		"",
		"--abc",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"这里有个链接 https://example.com/invoice",
		"",
		"--abc",
		"Content-Type: application/pdf",
		"Content-Disposition: inline; filename=\"invoice.pdf\"",
		"Content-Transfer-Encoding: base64",
		"",
		pdfB64,
		"--abc--",
		"",
	}, "\r\n")

	mr, err := mail.CreateReader(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("CreateReader: %v", err)
	}

	name, b, body, err := extractInvoicePDFAndBodyTextFromEmail(mr)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(b) == 0 || !strings.HasPrefix(string(b), "%PDF-") {
		t.Fatalf("expected pdf bytes, got len=%d head=%q", len(b), string(b))
	}
	if strings.TrimSpace(name) != "invoice.pdf" {
		t.Fatalf("expected filename invoice.pdf, got %q", name)
	}
	if !strings.Contains(body, "https://example.com/invoice") {
		t.Fatalf("expected body text collected, got:\n%s", body)
	}
}

func TestExtractInvoicePDFAndBodyTextFromEmail_AttachmentPDF_NoFilenameButMime(t *testing.T) {
	pdfRaw := []byte("%PDF-1.7\n%test\n")
	pdfB64 := base64.StdEncoding.EncodeToString(pdfRaw)

	raw := strings.Join([]string{
		"From: test@example.com",
		"To: you@example.com",
		"Subject: test",
		"MIME-Version: 1.0",
		"Content-Type: multipart/mixed; boundary=\"xyz\"",
		"",
		"--xyz",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"hello",
		"",
		"--xyz",
		"Content-Type: application/pdf",
		"Content-Disposition: attachment",
		"Content-Transfer-Encoding: base64",
		"",
		pdfB64,
		"--xyz--",
		"",
	}, "\r\n")

	mr, err := mail.CreateReader(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("CreateReader: %v", err)
	}

	name, b, _, err := extractInvoicePDFAndBodyTextFromEmail(mr)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(b) == 0 || !strings.HasPrefix(string(b), "%PDF-") {
		t.Fatalf("expected pdf bytes, got len=%d head=%q", len(b), string(b))
	}
	// Filename may be empty (no filename/name parameter); caller will fallback to invoice.pdf later.
	if strings.TrimSpace(name) != "" {
		t.Fatalf("expected empty filename when missing params, got %q", name)
	}
}

