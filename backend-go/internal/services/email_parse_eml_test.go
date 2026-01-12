package services

import (
	"strings"
	"testing"

	"github.com/emersion/go-message/mail"
)

func TestExtractInvoiceArtifactsFromEmail_RecursesIntoEMLAttachment(t *testing.T) {
	raw := strings.ReplaceAll(`From: a@example.com
To: b@example.com
Subject: outer
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="outer"

--outer
Content-Type: application/octet-stream
Content-Disposition: attachment; filename="forwarded.eml"

From: invoice@info.nuonuo.com
To: b@example.com
Subject: inner
MIME-Version: 1.0
Content-Type: text/html; charset=UTF-8

<div><a href="https://nnfp.jss.com.cn/8_CszRwjaw-FBnv">下载发票</a></div>

--outer--
`, "\n", "\r\n")

	mr, err := mail.CreateReader(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}

	_, _, _, _, bodyText, err := extractInvoiceArtifactsFromEmail(mr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bodyText, "https://nnfp.jss.com.cn/8_CszRwjaw-FBnv") {
		t.Fatalf("expected .eml link to be extracted, got: %q", bodyText)
	}
}

