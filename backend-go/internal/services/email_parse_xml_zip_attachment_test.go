package services

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/emersion/go-message/mail"
)

func TestExtractInvoiceArtifactsFromEmail_ZipAttachmentWithXML(t *testing.T) {
	xmlPayload := `<?xml version="1.0" encoding="UTF-8"?>
<Invoice>
  <fphm>25317000003387982028</fphm>
  <kprq>2025-12-30</kprq>
  <jshj>88.00</jshj>
  <hjse>4.99</hjse>
  <xfmc>上海星巴克咖啡经营有限公司</xfmc>
  <gfmc>个人（个人）</gfmc>
</Invoice>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("invoice.xml")
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := w.Write([]byte(xmlPayload)); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	zipB64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	raw := strings.Join([]string{
		"From: test@example.com",
		"To: you@example.com",
		"Subject: zip xml",
		"MIME-Version: 1.0",
		"Content-Type: multipart/mixed; boundary=\"z\"",
		"",
		"--z",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"hello",
		"",
		"--z",
		"Content-Type: application/zip",
		"Content-Disposition: attachment; filename=\"25317000003387982028.zip\"",
		"Content-Transfer-Encoding: base64",
		"",
		zipB64,
		"--z--",
		"",
	}, "\r\n")

	mr, err := mail.CreateReader(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("CreateReader: %v", err)
	}

	_, _, xmlBytes, _, _, err := extractInvoiceArtifactsFromEmail(mr)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(xmlBytes) == 0 || !bytes.HasPrefix(bytes.TrimSpace(xmlBytes), []byte("<?xml")) {
		head := xmlBytes
		if len(head) > 32 {
			head = head[:32]
		}
		t.Fatalf("expected xml bytes extracted, got len=%d head=%q", len(xmlBytes), string(head))
	}

	extracted, err := parseInvoiceXMLToExtracted(xmlBytes)
	if err != nil {
		t.Fatalf("parseInvoiceXMLToExtracted: %v", err)
	}
	if extracted.Amount == nil || *extracted.Amount != 88.00 {
		t.Fatalf("expected amount 88.00, got %+v", extracted.Amount)
	}
	if extracted.TaxAmount == nil || *extracted.TaxAmount != 4.99 {
		t.Fatalf("expected tax 4.99, got %+v", extracted.TaxAmount)
	}
	if extracted.InvoiceNumber == nil || strings.TrimSpace(*extracted.InvoiceNumber) != "25317000003387982028" {
		t.Fatalf("expected invoice number, got %+v", extracted.InvoiceNumber)
	}
}

func TestNormalizeInvoiceXMLBytes_ZipBombRejected(t *testing.T) {
	// Create an XML larger than the allowed limit so it is skipped during ZIP scan.
	oversize := bytes.Repeat([]byte("a"), emailParseMaxXMLBytes+16)
	xmlPayload := append([]byte("<Invoice>"), oversize...)
	xmlPayload = append(xmlPayload, []byte("</Invoice>")...)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("invoice.xml")
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := w.Write(xmlPayload); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}

	if _, _, err := normalizeInvoiceXMLBytes(buf.Bytes()); err == nil {
		t.Fatalf("expected oversize zip xml rejected, got err=nil")
	}
}
