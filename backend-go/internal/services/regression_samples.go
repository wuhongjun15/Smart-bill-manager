package services

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"smart-bill-manager/internal/models"
	"smart-bill-manager/internal/utils"
	"smart-bill-manager/pkg/database"
)

var ErrSampleNotFound = errors.New("regression sample not found")

type RegressionSampleService struct{}

func NewRegressionSampleService() *RegressionSampleService { return &RegressionSampleService{} }

type regressionSampleExpectedPayment struct {
	Amount          *float64 `json:"amount,omitempty"`
	Merchant        *string  `json:"merchant,omitempty"`
	TransactionTime *string  `json:"transaction_time,omitempty"`
	PaymentMethod   *string  `json:"payment_method,omitempty"`
	OrderNumber     *string  `json:"order_number,omitempty"`
}

type regressionSampleExpectedInvoice struct {
	InvoiceNumber *string  `json:"invoice_number,omitempty"`
	InvoiceDate   *string  `json:"invoice_date,omitempty"`
	Amount        *float64 `json:"amount,omitempty"`
	TaxAmount     *float64 `json:"tax_amount,omitempty"`
	SellerName    *string  `json:"seller_name,omitempty"`
	BuyerName     *string  `json:"buyer_name,omitempty"`
}

func normalizeSampleName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	s = regexp.MustCompile(`[^a-z0-9_\-]+`).ReplaceAllString(s, "_")
	s = strings.Trim(s, "_-")
	if s == "" {
		return ""
	}
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

func paymentTimeForSampleUTCToShanghai(transactionTime string) *string {
	transactionTime = strings.TrimSpace(transactionTime)
	if transactionTime == "" {
		return nil
	}

	tt, err := time.Parse(time.RFC3339Nano, transactionTime)
	if err != nil {
		if t2, err2 := time.Parse(time.RFC3339, transactionTime); err2 == nil {
			tt = t2
		} else {
			return nil
		}
	}
	loc := loadLocationOrUTC("Asia/Shanghai")
	v := tt.In(loc).Format("2006-01-02 15:04:05")
	return &v
}

func (s *RegressionSampleService) CreateOrUpdateFromPayment(paymentID string, createdBy string, name string) (*models.RegressionSample, error) {
	paymentID = strings.TrimSpace(paymentID)
	createdBy = strings.TrimSpace(createdBy)
	name = normalizeSampleName(name)
	if paymentID == "" || createdBy == "" {
		return nil, fmt.Errorf("missing fields")
	}

	db := database.GetDB()
	var p models.Payment
	res := db.Where("id = ?", paymentID).Limit(1).Find(&p)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	if p.IsDraft {
		return nil, fmt.Errorf("cannot create regression sample from draft payment")
	}
	if p.ExtractedData == nil || strings.TrimSpace(*p.ExtractedData) == "" {
		return nil, fmt.Errorf("payment has no extracted_data")
	}

	var ed PaymentExtractedData
	if err := json.Unmarshal([]byte(*p.ExtractedData), &ed); err != nil {
		return nil, fmt.Errorf("invalid payment extracted_data: %w", err)
	}
	raw := strings.TrimSpace(ed.RawText)
	if raw == "" {
		return nil, fmt.Errorf("payment extracted_data has empty raw_text")
	}

	exp := regressionSampleExpectedPayment{
		Amount:        nil,
		Merchant:      p.Merchant,
		PaymentMethod: p.PaymentMethod,
		OrderNumber:   ed.OrderNumber,
	}

	// Store as positive money (parser may output negative; regression compares abs).
	amt := math.Abs(p.Amount)
	exp.Amount = &amt
	exp.TransactionTime = paymentTimeForSampleUTCToShanghai(p.TransactionTime)

	expB, _ := json.Marshal(exp)

	if name == "" {
		short := paymentID
		if len(short) > 8 {
			short = short[:8]
		}
		name = "payment_" + short
	}

	sample := &models.RegressionSample{
		ID:           utils.GenerateUUID(),
		Kind:         "payment_screenshot",
		Name:         name,
		SourceType:   "payment",
		SourceID:     paymentID,
		CreatedBy:    createdBy,
		RawText:      raw,
		ExpectedJSON: string(expB),
	}

	// Upsert by (source_type, source_id, kind).
	var existing models.RegressionSample
	existingRes := db.Where("source_type = ? AND source_id = ? AND kind = ?", sample.SourceType, sample.SourceID, sample.Kind).
		Limit(1).
		Find(&existing)
	if existingRes.Error != nil {
		return nil, existingRes.Error
	}
	if existingRes.RowsAffected > 0 {
		update := map[string]any{
			"name":          sample.Name,
			"raw_text":      sample.RawText,
			"expected_json": sample.ExpectedJSON,
			"created_by":    sample.CreatedBy,
			"updated_at":    time.Now(),
		}
		if err := db.Model(&models.RegressionSample{}).Where("id = ?", existing.ID).Updates(update).Error; err != nil {
			return nil, err
		}
		out := existing
		out.Name = sample.Name
		out.RawText = sample.RawText
		out.ExpectedJSON = sample.ExpectedJSON
		out.CreatedBy = sample.CreatedBy
		return &out, nil
	}

	if err := db.Create(sample).Error; err != nil {
		return nil, err
	}
	return sample, nil
}

func (s *RegressionSampleService) CreateOrUpdateFromInvoice(invoiceID string, createdBy string, name string) (*models.RegressionSample, error) {
	invoiceID = strings.TrimSpace(invoiceID)
	createdBy = strings.TrimSpace(createdBy)
	name = normalizeSampleName(name)
	if invoiceID == "" || createdBy == "" {
		return nil, fmt.Errorf("missing fields")
	}

	db := database.GetDB()
	var inv models.Invoice
	res := db.Where("id = ?", invoiceID).Limit(1).Find(&inv)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	if inv.IsDraft {
		return nil, fmt.Errorf("cannot create regression sample from draft invoice")
	}

	raw := ""
	if inv.RawText != nil {
		raw = strings.TrimSpace(*inv.RawText)
	}
	if raw == "" && inv.ExtractedData != nil && strings.TrimSpace(*inv.ExtractedData) != "" {
		var ed InvoiceExtractedData
		if err := json.Unmarshal([]byte(*inv.ExtractedData), &ed); err == nil {
			raw = strings.TrimSpace(ed.RawText)
		}
	}
	if raw == "" {
		return nil, fmt.Errorf("invoice has no raw_text")
	}

	exp := regressionSampleExpectedInvoice{
		InvoiceNumber: inv.InvoiceNumber,
		InvoiceDate:   inv.InvoiceDate,
		Amount:        inv.Amount,
		TaxAmount:     inv.TaxAmount,
		SellerName:    inv.SellerName,
		BuyerName:     inv.BuyerName,
	}
	expB, _ := json.Marshal(exp)

	if name == "" {
		short := invoiceID
		if len(short) > 8 {
			short = short[:8]
		}
		name = "invoice_" + short
	}

	sample := &models.RegressionSample{
		ID:           utils.GenerateUUID(),
		Kind:         "invoice",
		Name:         name,
		SourceType:   "invoice",
		SourceID:     invoiceID,
		CreatedBy:    createdBy,
		RawText:      raw,
		ExpectedJSON: string(expB),
	}

	var existing models.RegressionSample
	existingRes := db.Where("source_type = ? AND source_id = ? AND kind = ?", sample.SourceType, sample.SourceID, sample.Kind).
		Limit(1).
		Find(&existing)
	if existingRes.Error != nil {
		return nil, existingRes.Error
	}
	if existingRes.RowsAffected > 0 {
		update := map[string]any{
			"name":          sample.Name,
			"raw_text":      sample.RawText,
			"expected_json": sample.ExpectedJSON,
			"created_by":    sample.CreatedBy,
			"updated_at":    time.Now(),
		}
		if err := db.Model(&models.RegressionSample{}).Where("id = ?", existing.ID).Updates(update).Error; err != nil {
			return nil, err
		}
		out := existing
		out.Name = sample.Name
		out.RawText = sample.RawText
		out.ExpectedJSON = sample.ExpectedJSON
		out.CreatedBy = sample.CreatedBy
		return &out, nil
	}

	if err := db.Create(sample).Error; err != nil {
		return nil, err
	}
	return sample, nil
}

type ListRegressionSamplesParams struct {
	Kind   string
	Search string
	Limit  int
	Offset int
}

func (s *RegressionSampleService) List(params ListRegressionSamplesParams) ([]models.RegressionSample, int64, error) {
	db := database.GetDB()
	q := db.Model(&models.RegressionSample{})

	kind := strings.TrimSpace(params.Kind)
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	search := strings.TrimSpace(params.Search)
	if search != "" {
		like := "%" + search + "%"
		q = q.Where("name LIKE ? OR source_id LIKE ?", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := params.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	var rows []models.RegressionSample
	if err := q.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *RegressionSampleService) Delete(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrSampleNotFound
	}
	db := database.GetDB()
	res := db.Where("id = ?", id).Delete(&models.RegressionSample{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrSampleNotFound
	}
	return nil
}

func (s *RegressionSampleService) BulkDelete(ids []string) (deleted int, err error) {
	if len(ids) == 0 {
		return 0, nil
	}
	clean := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		clean = append(clean, id)
	}
	if len(clean) == 0 {
		return 0, nil
	}
	db := database.GetDB()
	res := db.Where("id IN ?", clean).Delete(&models.RegressionSample{})
	if res.Error != nil {
		return 0, res.Error
	}
	return int(res.RowsAffected), nil
}

func (s *RegressionSampleService) ExportZip(kind string) ([]byte, string, error) {
	db := database.GetDB()
	q := db.Model(&models.RegressionSample{})
	kind = strings.TrimSpace(kind)
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}

	var rows []models.RegressionSample
	if err := q.Find(&rows).Error; err != nil {
		return nil, "", err
	}
	if len(rows) == 0 {
		return nil, "", fmt.Errorf("no samples to export")
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Kind != rows[j].Kind {
			return rows[i].Kind < rows[j].Kind
		}
		if rows[i].Name != rows[j].Name {
			return rows[i].Name < rows[j].Name
		}
		return rows[i].ID < rows[j].ID
	})

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	now := time.Now().UTC().Format("20060102_150405")
	zipName := "regression_samples_" + now + ".zip"
	base := filepath.Join("backend-go", "internal", "services", "testdata", "regression")

	for _, r := range rows {
		dir := "misc"
		switch r.Kind {
		case "payment_screenshot":
			dir = "payments"
		case "invoice":
			dir = "invoices"
		}

		filename := normalizeSampleName(r.Name)
		if filename == "" {
			filename = r.SourceType + "_" + r.SourceID
		}
		path := filepath.ToSlash(filepath.Join(base, dir, filename+".json"))

		var expected any
		_ = json.Unmarshal([]byte(r.ExpectedJSON), &expected)
		payload := map[string]any{
			"kind":     r.Kind,
			"name":     r.Name,
			"raw_text": r.RawText,
			"expected": expected,
		}

		b, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			continue
		}
		b = append(b, '\n')

		f, err := zw.Create(path)
		if err != nil {
			continue
		}
		_, _ = f.Write(b)
	}

	_ = zw.Close()
	return buf.Bytes(), zipName, nil
}
