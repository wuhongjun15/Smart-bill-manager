package services

import "testing"

func TestParseInvoiceDataWithMeta_DidiHeaderInvoiceNumber(t *testing.T) {
	svc := &OCRService{}

	text := `
【第1页-分区】
【发票信息】
发票号码: 开票日期: 25117000000677781736 2025年05月12日
旅客运输服务 电子发票（普通发票）
`

	got, err := svc.ParseInvoiceDataWithMeta(text, nil)
	if err != nil {
		t.Fatalf("ParseInvoiceDataWithMeta error: %v", err)
	}
	if got.InvoiceNumber == nil || *got.InvoiceNumber != "25117000000677781736" {
		t.Fatalf("expected invoice number=%q got=%+v (src=%q conf=%v)", "25117000000677781736", got.InvoiceNumber, got.InvoiceNumberSource, got.InvoiceNumberConfidence)
	}
}

func TestExtractInvoiceLineItemsFromPDFZones_DidiStripsAmountAndSkipsBuyer(t *testing.T) {
	pages := []PDFTextZonesPage{
		{
			Page:   1,
			Width:  1000,
			Height: 1000,
			Rows: []PDFTextZonesRow{
				{
					Region: "buyer",
					Y0:     220,
					Y1:     250,
					Text:   "购买方信息 名称： 统一社会信用代码/纳税人识别号： 个人（个人）",
					Spans: []PDFTextZonesSpan{
						{X0: 80, Y0: 220, X1: 260, Y1: 250, T: "购买方信息"},
						{X0: 280, Y0: 220, X1: 520, Y1: 250, T: "名称：个人（个人）"},
					},
				},
				{
					Region: "items",
					Y0:     600,
					Y1:     620,
					Text:   "*运输服务*客运服务费 68.34 1",
					Spans: []PDFTextZonesSpan{
						{X0: 80, Y0: 600, X1: 360, Y1: 620, T: "*运输服务*客运服务费"},
						// Didi invoices often render the amount before the qty; it can land in the "name" column.
						{X0: 450, Y0: 600, X1: 520, Y1: 620, T: "68.34"},
						{X0: 730, Y0: 600, X1: 745, Y1: 620, T: "1"},
					},
				},
			},
		},
	}

	items := extractInvoiceLineItemsFromPDFZones(pages)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %+v", items)
	}
	if items[0].Name != "*运输服务*客运服务费" {
		t.Fatalf("expected name parsed, got %q", items[0].Name)
	}
	if items[0].Quantity == nil || *items[0].Quantity != 1 {
		t.Fatalf("expected qty 1, got %+v", items[0].Quantity)
	}
}

func TestExtractInvoiceLineItemsFromPDFZones_DidiInfersQtyWhenMiscolumned(t *testing.T) {
	pages := []PDFTextZonesPage{
		{
			Page:   1,
			Width:  1000,
			Height: 1000,
			Rows: []PDFTextZonesRow{
				{
					Region: "items",
					Y0:     600,
					Y1:     620,
					Text:   "*运输服务*客运服务费68.34 1",
					Spans: []PDFTextZonesSpan{
						// Amount glued into the name span.
						{X0: 80, Y0: 600, X1: 420, Y1: 620, T: "*运输服务*客运服务费68.34"},
						// Qty token lands left of the qty column boundary (mis-columned).
						{X0: 520, Y0: 600, X1: 535, Y1: 620, T: "1"},
					},
				},
			},
		},
	}

	items := extractInvoiceLineItemsFromPDFZones(pages)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %+v", items)
	}
	if items[0].Name != "*运输服务*客运服务费" {
		t.Fatalf("expected name parsed, got %q", items[0].Name)
	}
	if items[0].Spec != "" {
		t.Fatalf("expected empty spec, got %q", items[0].Spec)
	}
	if items[0].Quantity == nil || *items[0].Quantity != 1 {
		t.Fatalf("expected qty 1, got %+v", items[0].Quantity)
	}
}
