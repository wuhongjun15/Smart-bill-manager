package services

import "testing"

func TestParseInvoiceXMLToExtracted_EInvoice(t *testing.T) {
	xmlStr := `
<EInvoice>
  <Header>
    <EIid>25327000001739485410</EIid>
    <EInvoiceTag>SWEI3200</EInvoiceTag>
    <Version>0.2</Version>
  </Header>
  <EInvoiceData>
    <SellerInformation>
      <SellerIdNum>913205830880018839</SellerIdNum>
      <SellerName>昆山京东尚信贸易有限公司</SellerName>
    </SellerInformation>
    <BuyerInformation>
      <BuyerName>乌洪军</BuyerName>
    </BuyerInformation>
    <BasicInformation>
      <TotalAmWithoutTax>5838.94</TotalAmWithoutTax>
      <TotalTaxAm>759.06</TotalTaxAm>
      <TotalTax-includedAmount>6598.00</TotalTax-includedAmount>
      <RequestTime>2025-12-29 13:02:50</RequestTime>
    </BasicInformation>
    <IssuItemInformation>
      <ItemName>*制冷空调设备*格力空调 云锦Ⅲ 1.5匹新一级能效变频 纯铜管省电舒适风搭载冷酷外机挂机国家补贴 KFR-35GW/NhAe1BAj</ItemName>
      <SpecMod>KFR-35GW/NhAe1BAj</SpecMod>
      <MeaUnits>套</MeaUnits>
      <Quantity>2.00000000</Quantity>
    </IssuItemInformation>
  </EInvoiceData>
  <TaxSupervisionInfo>
    <InvoiceNumber>25327000001739485410</InvoiceNumber>
    <IssueTime>2025-12-29 13:02:50</IssueTime>
  </TaxSupervisionInfo>
</EInvoice>
`

	extracted, err := parseInvoiceXMLToExtracted([]byte(xmlStr))
	if err != nil {
		t.Fatalf("parseInvoiceXMLToExtracted err: %v", err)
	}
	if extracted.InvoiceNumber == nil || *extracted.InvoiceNumber != "25327000001739485410" {
		t.Fatalf("invoice number mismatch: %#v", extracted.InvoiceNumber)
	}
	if extracted.InvoiceDate == nil || *extracted.InvoiceDate != "2025-12-29" {
		t.Fatalf("invoice date mismatch: %#v", extracted.InvoiceDate)
	}
	if extracted.Amount == nil || (*extracted.Amount < 6597.999 || *extracted.Amount > 6598.001) {
		t.Fatalf("amount mismatch: %#v", extracted.Amount)
	}
	if extracted.TaxAmount == nil || (*extracted.TaxAmount < 759.059 || *extracted.TaxAmount > 759.061) {
		t.Fatalf("tax amount mismatch: %#v", extracted.TaxAmount)
	}
	if extracted.SellerName == nil || *extracted.SellerName != "昆山京东尚信贸易有限公司" {
		t.Fatalf("seller mismatch: %#v", extracted.SellerName)
	}
	if extracted.BuyerName == nil || *extracted.BuyerName != "乌洪军" {
		t.Fatalf("buyer mismatch: %#v", extracted.BuyerName)
	}
	if len(extracted.Items) != 1 {
		t.Fatalf("items count mismatch: %d", len(extracted.Items))
	}
	item := extracted.Items[0]
	if item.Name == "" || item.Spec != "KFR-35GW/NhAe1BAj" || item.Unit != "套" || item.Quantity == nil || (*item.Quantity < 1.999 || *item.Quantity > 2.001) {
		t.Fatalf("item mismatch: %+v", item)
	}
}

