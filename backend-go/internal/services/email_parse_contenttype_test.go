package services

import "testing"

type headerGetOnly map[string]string

func (h headerGetOnly) Get(key string) string {
	return h[key]
}

func TestContentTypeLowerFromHeader_GetOnly(t *testing.T) {
	h := headerGetOnly{
		"Content-Type": "Text/HTML; charset=UTF-8",
	}
	got := contentTypeLowerFromHeader(h)
	if got != "text/html" {
		t.Fatalf("unexpected content type: %q", got)
	}
}

func TestLooksLikeTextBytes(t *testing.T) {
	if !looksLikeTextBytes([]byte("<div>点击链接查看发票：<a href=\"https://nnfp.jss.com.cn/8_abc\">下载发票</a></div>")) {
		t.Fatalf("expected html bytes to look like text")
	}

	// BMP header: "BM" + zeros.
	if looksLikeTextBytes([]byte{0x42, 0x4D, 0x3A, 0x00, 0x00, 0x00, 0x00}) {
		t.Fatalf("expected bmp-like bytes to not look like text")
	}
}
