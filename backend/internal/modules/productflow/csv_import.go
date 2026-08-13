package productflow

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/importcsv"
)

type CSVImportRequest struct {
	CSV     string            `json:"csv" binding:"required"`
	Mapping map[string]string `json:"mapping" binding:"required"`
}

type CSVImportPreview struct {
	Headers     []string             `json:"headers"`
	Rows        []map[string]string  `json:"rows"`
	TotalRows   int                  `json:"totalRows"`
	ValidRows   int                  `json:"validRows"`
	InvalidRows int                  `json:"invalidRows"`
	Errors      []importcsv.RowError `json:"errors"`
}

var marketCSVFields = map[string]bool{"candidate_id": true, "platform": true, "signal_type": true, "value": true, "unit": true, "observed_at": true, "confidence": true, "source": true}
var performanceCSVFields = map[string]bool{"candidate_id": true, "catalog_product_id": true, "listing_draft_id": true, "platform_listing_id": true, "platform": true, "observed_at": true, "period_start": true, "period_end": true, "impressions": true, "views": true, "clicks": true, "favorites": true, "inquiries": true, "messages": true, "orders": true, "units_sold": true, "gross_revenue": true, "refund_amount": true, "platform_cost": true, "actual_cost": true, "realized_profit": true, "refund_count": true, "return_count": true, "after_sale_count": true}

func validateMapping(mapping map[string]string, headers []string, allowed map[string]bool, required []string) error {
	headerSet := map[string]bool{}
	for _, h := range headers {
		headerSet[h] = true
	}
	targets := map[string]bool{}
	for source, target := range mapping {
		if !headerSet[source] || !allowed[target] || targets[target] {
			return fmt.Errorf("%w: invalid or duplicate field mapping", ErrValidation)
		}
		targets[target] = true
	}
	for _, field := range required {
		if !targets[field] {
			return fmt.Errorf("%w: missing mapping for %s", ErrValidation, field)
		}
	}
	return nil
}

func parseCSVTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date")
}
func int64Ptr(value string) (*int64, error) {
	if value == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n < 0 {
		return nil, fmt.Errorf("invalid non-negative integer")
	}
	return &n, nil
}

func marketItemFromMap(row map[string]string) (CreateMarketSignalBody, error) {
	id, err := uuid.Parse(row["candidate_id"])
	if err != nil {
		return CreateMarketSignalBody{}, fmt.Errorf("candidate_id is invalid")
	}
	value, err := strconv.ParseInt(row["value"], 10, 64)
	if err != nil {
		return CreateMarketSignalBody{}, fmt.Errorf("value is invalid")
	}
	observed, err := parseCSVTime(row["observed_at"])
	if err != nil {
		return CreateMarketSignalBody{}, fmt.Errorf("observed_at is invalid")
	}
	confidence := int64(6000)
	if row["confidence"] != "" {
		v, e := strconv.ParseInt(row["confidence"], 10, 64)
		if e != nil || v < 0 || v > 100 {
			return CreateMarketSignalBody{}, fmt.Errorf("confidence must be 0..100")
		}
		confidence = v * 100
	}
	for _, field := range []string{"platform", "signal_type", "unit", "source"} {
		if importcsv.UnsafeFormula(row[field]) {
			return CreateMarketSignalBody{}, fmt.Errorf("%s contains an unsafe spreadsheet formula", field)
		}
	}
	return CreateMarketSignalBody{CandidateID: id, Platform: row["platform"], SignalType: row["signal_type"], Value: value, Unit: row["unit"], Source: row["source"], Origin: SignalOriginImport, ConfidenceBPS: confidence, ObservedAt: &observed}, nil
}

func performanceItemFromMap(row map[string]string) (ImportPerformanceItem, error) {
	candidateID, err := uuid.Parse(row["candidate_id"])
	if err != nil {
		return ImportPerformanceItem{}, fmt.Errorf("candidate_id is invalid")
	}
	catalogID, err := uuid.Parse(row["catalog_product_id"])
	if err != nil {
		return ImportPerformanceItem{}, fmt.Errorf("catalog_product_id is invalid")
	}
	observed, err := parseCSVTime(row["observed_at"])
	if err != nil {
		return ImportPerformanceItem{}, fmt.Errorf("observed_at is invalid")
	}
	start, err := parseCSVTime(row["period_start"])
	if err != nil {
		return ImportPerformanceItem{}, fmt.Errorf("period_start is invalid")
	}
	end, err := parseCSVTime(row["period_end"])
	if err != nil {
		return ImportPerformanceItem{}, fmt.Errorf("period_end is invalid")
	}
	if importcsv.UnsafeFormula(row["platform"]) {
		return ImportPerformanceItem{}, fmt.Errorf("platform contains an unsafe spreadsheet formula")
	}
	item := ImportPerformanceItem{CandidateID: candidateID, CatalogProductID: catalogID, Platform: row["platform"], Source: SignalOriginImport, ObservedAt: observed, PeriodStart: start, PeriodEnd: end}
	item.PlatformListingID = strings.TrimSpace(row["platform_listing_id"])
	if text := row["listing_draft_id"]; text != "" {
		id, e := uuid.Parse(text)
		if e != nil {
			return item, fmt.Errorf("listing_draft_id is invalid")
		}
		item.ListingDraftID = &id
	}
	counts := map[string]**int64{"impressions": &item.Impressions, "views": &item.Views, "clicks": &item.Clicks, "favorites": &item.Favorites, "inquiries": &item.Inquiries, "messages": &item.Messages, "orders": &item.Orders, "units_sold": &item.UnitsSold, "refund_count": &item.RefundCount, "return_count": &item.ReturnCount, "after_sale_count": &item.AfterSaleCount}
	for field, target := range counts {
		value, e := int64Ptr(row[field])
		if e != nil {
			return item, fmt.Errorf("%s is invalid", field)
		}
		*target = value
	}
	money := map[string]**string{"gross_revenue": &item.GrossRevenue, "refund_amount": &item.RefundAmount, "platform_cost": &item.PlatformCost, "actual_cost": &item.ActualCost, "realized_profit": &item.RealizedProfit}
	for field, target := range money {
		if text := strings.TrimSpace(row[field]); text != "" {
			if _, e := parseOptionalPerformanceMoney(&text); e != nil {
				return item, fmt.Errorf("%s is invalid", field)
			}
			*target = &text
		}
	}
	return item, nil
}

func previewCSV(body CSVImportRequest, allowed map[string]bool, required []string, validate func(map[string]string) error) (*CSVImportPreview, error) {
	doc, err := importcsv.Parse(body.CSV)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	if err := validateMapping(body.Mapping, doc.Headers, allowed, required); err != nil {
		return nil, err
	}
	errorsByRow := map[int]bool{}
	errorsList := append([]importcsv.RowError{}, doc.Errors...)
	for _, e := range doc.Errors {
		errorsByRow[e.Row] = true
	}
	rows := make([]map[string]string, 0, min(len(doc.Rows), 20))
	for i, source := range doc.Rows {
		mapped := importcsv.MapRow(source, body.Mapping)
		if err := validate(mapped); err != nil {
			errorsList = append(errorsList, importcsv.RowError{Row: i + 2, Message: err.Error()})
			errorsByRow[i+2] = true
		}
		if len(rows) < 20 {
			rows = append(rows, mapped)
		}
	}
	return &CSVImportPreview{Headers: doc.Headers, Rows: rows, TotalRows: len(doc.Rows), ValidRows: len(doc.Rows) - len(errorsByRow), InvalidRows: len(errorsByRow), Errors: errorsList}, nil
}

func (s *Service) PreviewMarketSignalCSV(ctx context.Context, tenantID int64, body CSVImportRequest) (*CSVImportPreview, error) {
	return previewCSV(body, marketCSVFields, []string{"candidate_id", "platform", "signal_type", "value", "observed_at", "source"}, func(row map[string]string) error {
		item, err := marketItemFromMap(row)
		if err != nil {
			return err
		}
		var count int64
		if err := s.DB.WithContext(ctx).Model(&Candidate{}).Where("tenant_id = ? AND id = ?", tenantID, item.CandidateID).Count(&count).Error; err != nil || count == 0 {
			return fmt.Errorf("candidate_id does not exist")
		}
		return nil
	})
}
func (s *Service) PreviewPerformanceCSV(ctx context.Context, tenantID int64, body CSVImportRequest) (*CSVImportPreview, error) {
	return previewCSV(body, performanceCSVFields, []string{"candidate_id", "catalog_product_id", "platform", "observed_at", "period_start", "period_end"}, func(row map[string]string) error {
		item, err := performanceItemFromMap(row)
		if err != nil {
			return err
		}
		var count int64
		if err := s.DB.WithContext(ctx).Table("products").Where("tenant_id = ? AND id = ? AND candidate_id = ?", tenantID, item.CatalogProductID, item.CandidateID).Count(&count).Error; err != nil || count == 0 {
			return fmt.Errorf("candidate/catalog relationship does not exist")
		}
		return nil
	})
}

func (s *Service) ImportMappedMarketSignalCSV(ctx context.Context, tenantID int64, body CSVImportRequest) (*MarketSignalImportResult, error) {
	preview, err := s.PreviewMarketSignalCSV(ctx, tenantID, body)
	if err != nil {
		return nil, err
	}
	doc, _ := importcsv.Parse(body.CSV)
	out := &MarketSignalImportResult{Failed: map[int]string{}}
	for i, source := range doc.Rows {
		item, e := marketItemFromMap(importcsv.MapRow(source, body.Mapping))
		if e != nil {
			out.Failed[i+2] = e.Error()
			continue
		}
		_, e = s.CreateMarketSignal(ctx, tenantID, item.CandidateID, item)
		if e == nil {
			out.Imported++
		} else if e == ErrConflict {
			out.Duplicates++
		} else {
			out.Failed[i+2] = truncateError(e)
		}
	}
	_ = preview
	return out, nil
}
func (s *Service) ImportMappedPerformanceCSV(ctx context.Context, tenantID int64, body CSVImportRequest) (*PerformanceImportResult, error) {
	if _, err := s.PreviewPerformanceCSV(ctx, tenantID, body); err != nil {
		return nil, err
	}
	doc, _ := importcsv.Parse(body.CSV)
	out := &PerformanceImportResult{Failed: map[int]string{}}
	for i, source := range doc.Rows {
		item, e := performanceItemFromMap(importcsv.MapRow(source, body.Mapping))
		if e != nil {
			out.Failed[i+2] = e.Error()
			continue
		}
		_, e = s.importPerformanceItem(ctx, tenantID, item)
		if e == nil {
			out.Imported++
		} else if e == ErrConflict {
			out.Duplicates++
		} else {
			out.Failed[i+2] = truncateError(e)
		}
	}
	return out, nil
}
