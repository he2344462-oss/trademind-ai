package productflow

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/pricingengine"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ImportPerformanceItem struct {
	ListingDraftID                                                                *uuid.UUID `json:"listingDraftId"`
	CatalogProductID                                                              uuid.UUID  `json:"catalogProductId"`
	CandidateID                                                                   uuid.UUID  `json:"candidateId"`
	Platform                                                                      string     `json:"platform"`
	Source                                                                        string     `json:"source"`
	ObservedAt                                                                    time.Time  `json:"observedAt"`
	PeriodStart                                                                   time.Time  `json:"periodStart"`
	PeriodEnd                                                                     time.Time  `json:"periodEnd"`
	Impressions, Views, Clicks, Favorites, Inquiries, Messages, Orders, UnitsSold *int64
	GrossRevenue, RefundAmount, PlatformCost, ActualCost, RealizedProfit          *string
	RefundCount, ReturnCount, AfterSaleCount                                      *int64
	RawData                                                                       json.RawMessage `json:"rawData"`
}

type ImportPerformanceBody struct {
	Items []ImportPerformanceItem `json:"items"`
	CSV   string                  `json:"csv"`
}
type PerformanceImportResult struct {
	Imported   int            `json:"imported"`
	Duplicates int            `json:"duplicates"`
	Failed     map[int]string `json:"failed"`
}

type PerformanceSummary struct {
	SnapshotCount    int64      `json:"snapshotCount"`
	ListingCount     int64      `json:"listingCount"`
	Views            int64      `json:"views"`
	Inquiries        int64      `json:"inquiries"`
	Orders           int64      `json:"orders"`
	Refunds          int64      `json:"refunds"`
	GrossRevenue     int64      `json:"grossRevenue"`
	RealizedProfit   int64      `json:"realizedProfit"`
	ActualMarginBPS  int64      `json:"actualMarginBps"`
	LatestObservedAt *time.Time `json:"latestObservedAt,omitempty"`
}

type SelectionPerformanceGroup struct {
	Recommendation string `json:"recommendation"`
	Tested         int64  `json:"tested"`
	WithOrders     int64  `json:"withOrders"`
	PositiveProfit int64  `json:"positiveProfit"`
	TotalProfit    int64  `json:"totalProfit"`
}
type SelectionPerformanceReport struct {
	GeneratedAt time.Time                    `json:"generatedAt"`
	Groups      []SelectionPerformanceGroup  `json:"groups"`
	Evaluations []SelectionOutcomeEvaluation `json:"evaluations"`
}

func performanceSourceAllowed(value string) bool {
	switch value {
	case SignalOriginOfficial, SignalOriginAuthorized, SignalOriginManual, SignalOriginImport, SignalOriginFixture:
		return true
	}
	return false
}
func nonnegative(values ...*int64) bool {
	for _, value := range values {
		if value != nil && *value < 0 {
			return false
		}
	}
	return true
}

func performanceFingerprint(tenantID int64, item ImportPerformanceItem) string {
	value := fmt.Sprintf("%d\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s", tenantID, item.CandidateID, item.CatalogProductID, strings.ToLower(item.Platform), strings.ToLower(item.Source), item.PeriodStart.UTC().Format(time.RFC3339Nano), item.PeriodEnd.UTC().Format(time.RFC3339Nano))
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func parseOptionalPerformanceMoney(value *string) (*int64, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	money, err := pricingengine.ParseMoney(*value)
	if err != nil {
		return nil, err
	}
	cents := int64(money)
	return &cents, nil
}

func (s *Service) importPerformanceItem(ctx context.Context, tenantID int64, item ImportPerformanceItem) (*ListingPerformanceSnapshot, error) {
	item.Platform = strings.ToLower(strings.TrimSpace(item.Platform))
	item.Source = strings.ToLower(strings.TrimSpace(item.Source))
	if item.CandidateID == uuid.Nil || item.CatalogProductID == uuid.Nil || !SupportedDraftPlatform(item.Platform) || !performanceSourceAllowed(item.Source) {
		return nil, fmt.Errorf("%w: invalid candidate, catalog, platform or source", ErrValidation)
	}
	if item.ObservedAt.IsZero() || item.PeriodStart.IsZero() || item.PeriodEnd.IsZero() || item.PeriodEnd.Before(item.PeriodStart) || item.ObservedAt.After(time.Now().UTC().Add(5*time.Minute)) {
		return nil, fmt.Errorf("%w: invalid performance dates", ErrValidation)
	}
	if !nonnegative(item.Impressions, item.Views, item.Clicks, item.Favorites, item.Inquiries, item.Messages, item.Orders, item.UnitsSold, item.RefundCount, item.ReturnCount, item.AfterSaleCount) {
		return nil, fmt.Errorf("%w: performance counts cannot be negative", ErrValidation)
	}
	if _, err := s.GetCandidate(ctx, tenantID, item.CandidateID); err != nil {
		return nil, err
	}
	var catalog struct{ ID uuid.UUID }
	if err := s.DB.WithContext(ctx).Table("products").Select("id").Where("tenant_id = ? AND id = ? AND candidate_id = ?", tenantID, item.CatalogProductID, item.CandidateID).Scan(&catalog).Error; err != nil || catalog.ID == uuid.Nil {
		return nil, ErrNotFound
	}
	if item.ListingDraftID != nil {
		var count int64
		if err := s.DB.WithContext(ctx).Model(&ListingDraft{}).Where("tenant_id = ? AND id = ? AND catalog_product_id = ?", tenantID, *item.ListingDraftID, item.CatalogProductID).Count(&count).Error; err != nil || count == 0 {
			return nil, ErrNotFound
		}
	}
	gross, err := parseOptionalPerformanceMoney(item.GrossRevenue)
	if err != nil {
		return nil, fmt.Errorf("%w: grossRevenue", ErrValidation)
	}
	refund, err := parseOptionalPerformanceMoney(item.RefundAmount)
	if err != nil {
		return nil, fmt.Errorf("%w: refundAmount", ErrValidation)
	}
	platformCost, err := parseOptionalPerformanceMoney(item.PlatformCost)
	if err != nil {
		return nil, fmt.Errorf("%w: platformCost", ErrValidation)
	}
	actualCost, err := parseOptionalPerformanceMoney(item.ActualCost)
	if err != nil {
		return nil, fmt.Errorf("%w: actualCost", ErrValidation)
	}
	profit, err := parseOptionalPerformanceMoney(item.RealizedProfit)
	if err != nil {
		return nil, fmt.Errorf("%w: realizedProfit", ErrValidation)
	}
	raw := datatypes.JSON([]byte("{}"))
	if len(item.RawData) > 0 {
		if !json.Valid(item.RawData) {
			return nil, fmt.Errorf("%w: invalid rawData", ErrValidation)
		}
		raw = datatypes.JSON(item.RawData)
	}
	row := ListingPerformanceSnapshot{TenantID: tenantID, ListingDraftID: item.ListingDraftID, CatalogProductID: item.CatalogProductID, CandidateID: item.CandidateID, Platform: item.Platform, Source: item.Source, ObservedAt: item.ObservedAt.UTC(), PeriodStart: item.PeriodStart.UTC(), PeriodEnd: item.PeriodEnd.UTC(), Impressions: item.Impressions, Views: item.Views, Clicks: item.Clicks, Favorites: item.Favorites, Inquiries: item.Inquiries, Messages: item.Messages, Orders: item.Orders, UnitsSold: item.UnitsSold, GrossRevenue: gross, RefundAmount: refund, PlatformCost: platformCost, ActualCost: actualCost, RealizedProfit: profit, RefundCount: item.RefundCount, ReturnCount: item.ReturnCount, AfterSaleCount: item.AfterSaleCount, RawData: raw, Fingerprint: performanceFingerprint(tenantID, item)}
	if err := s.DB.WithContext(ctx).Create(&row).Error; err != nil {
		if isUniqueError(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return &row, nil
}

func parsePerformanceCSV(input string) ([]ImportPerformanceItem, error) {
	r := csv.NewReader(strings.NewReader(input))
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("%w: invalid csv header", ErrValidation)
	}
	index := map[string]int{}
	for i, name := range header {
		index[strings.TrimSpace(strings.ToLower(name))] = i
	}
	required := []string{"candidate_id", "catalog_product_id", "platform", "source", "observed_at", "period_start", "period_end"}
	for _, key := range required {
		if _, ok := index[key]; !ok {
			return nil, fmt.Errorf("%w: missing csv column %s", ErrValidation, key)
		}
	}
	items := []ImportPerformanceItem{}
	for {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, fmt.Errorf("%w: invalid csv row", ErrValidation)
		}
		get := func(key string) string {
			idx, ok := index[key]
			if !ok || idx >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[idx])
		}
		candidateID, e := uuid.Parse(get("candidate_id"))
		if e != nil {
			return nil, fmt.Errorf("%w: invalid candidate_id", ErrValidation)
		}
		catalogID, e := uuid.Parse(get("catalog_product_id"))
		if e != nil {
			return nil, fmt.Errorf("%w: invalid catalog_product_id", ErrValidation)
		}
		observed, e := time.Parse(time.RFC3339, get("observed_at"))
		if e != nil {
			return nil, fmt.Errorf("%w: invalid observed_at", ErrValidation)
		}
		start, e := time.Parse(time.RFC3339, get("period_start"))
		if e != nil {
			return nil, fmt.Errorf("%w: invalid period_start", ErrValidation)
		}
		end, e := time.Parse(time.RFC3339, get("period_end"))
		if e != nil {
			return nil, fmt.Errorf("%w: invalid period_end", ErrValidation)
		}
		parseCount := func(key string) *int64 {
			text := get(key)
			if text == "" {
				return nil
			}
			value, parseErr := strconv.ParseInt(text, 10, 64)
			if parseErr != nil {
				return nil
			}
			return &value
		}
		money := func(key string) *string {
			text := get(key)
			if text == "" {
				return nil
			}
			return &text
		}
		items = append(items, ImportPerformanceItem{CandidateID: candidateID, CatalogProductID: catalogID, Platform: get("platform"), Source: get("source"), ObservedAt: observed, PeriodStart: start, PeriodEnd: end, Views: parseCount("views"), Inquiries: parseCount("inquiries"), Orders: parseCount("orders"), RefundCount: parseCount("refund_count"), GrossRevenue: money("gross_revenue"), RealizedProfit: money("realized_profit")})
	}
	return items, nil
}

func (s *Service) ImportPerformance(ctx context.Context, tenantID int64, body ImportPerformanceBody) (*PerformanceImportResult, error) {
	items := append([]ImportPerformanceItem{}, body.Items...)
	if strings.TrimSpace(body.CSV) != "" {
		parsed, err := parsePerformanceCSV(body.CSV)
		if err != nil {
			return nil, err
		}
		items = append(items, parsed...)
	}
	if len(items) == 0 || len(items) > 1000 {
		return nil, fmt.Errorf("%w: import requires 1..1000 rows", ErrValidation)
	}
	out := &PerformanceImportResult{Failed: map[int]string{}}
	for i, item := range items {
		_, err := s.importPerformanceItem(ctx, tenantID, item)
		if err == nil {
			out.Imported++
		} else if err == ErrConflict {
			out.Duplicates++
		} else {
			out.Failed[i] = truncateError(err)
		}
	}
	return out, nil
}

func (s *Service) ListPerformance(ctx context.Context, tenantID int64, q ListQuery) (*PageResult[ListingPerformanceSnapshot], error) {
	q = normalizePage(q)
	db := s.DB.WithContext(ctx).Model(&ListingPerformanceSnapshot{}).Where("tenant_id = ?", tenantID)
	if q.Platform != "" {
		db = db.Where("platform = ?", strings.ToLower(q.Platform))
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []ListingPerformanceSnapshot
	if err := db.Order("observed_at DESC, id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	return &PageResult[ListingPerformanceSnapshot]{List: rows, Page: q.Page, PageSize: q.PageSize, Total: total, TotalPages: totalPages(total, q.PageSize)}, nil
}

func (s *Service) PerformanceSummary(ctx context.Context, tenantID int64) (*PerformanceSummary, error) {
	var out PerformanceSummary
	row := s.DB.WithContext(ctx).Table("listing_performance_snapshots").Select("COUNT(*) snapshot_count, COUNT(DISTINCT catalog_product_id) listing_count, COALESCE(SUM(views),0) views, COALESCE(SUM(inquiries),0) inquiries, COALESCE(SUM(orders),0) orders, COALESCE(SUM(refund_count),0) refunds, COALESCE(SUM(gross_revenue),0) gross_revenue, COALESCE(SUM(realized_profit),0) realized_profit").Where("tenant_id = ?", tenantID).Scan(&out)
	if row.Error != nil {
		return nil, row.Error
	}
	if out.GrossRevenue != 0 {
		out.ActualMarginBPS = out.RealizedProfit * 10000 / out.GrossRevenue
	}
	var latest ListingPerformanceSnapshot
	if err := s.DB.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("observed_at DESC").First(&latest).Error; err == nil {
		out.LatestObservedAt = &latest.ObservedAt
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return &out, nil
}

func (s *Service) SelectionPerformance(ctx context.Context, tenantID int64) (*SelectionPerformanceReport, error) {
	type aggregate struct {
		CandidateID                                        uuid.UUID
		Views, Inquiries, Orders, Refunds, Revenue, Profit int64
	}
	var aggregates []aggregate
	if err := s.DB.WithContext(ctx).Table("listing_performance_snapshots").Select("candidate_id, COALESCE(SUM(views),0) views, COALESCE(SUM(inquiries),0) inquiries, COALESCE(SUM(orders),0) orders, COALESCE(SUM(refund_count),0) refunds, COALESCE(SUM(gross_revenue),0) revenue, COALESCE(SUM(realized_profit),0) profit").Where("tenant_id = ?", tenantID).Group("candidate_id").Scan(&aggregates).Error; err != nil {
		return nil, err
	}
	evaluations := []SelectionOutcomeEvaluation{}
	now := time.Now().UTC()
	for _, actual := range aggregates {
		var analysis CandidateAnalysis
		if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND candidate_id = ?", tenantID, actual.CandidateID).Order("analysis_version DESC").First(&analysis).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return nil, err
		}
		var cost struct {
			EstimatedMarginBPS int64 `json:"estimatedMarginBps"`
		}
		_ = json.Unmarshal(analysis.CostSnapshot, &cost)
		margin := int64(0)
		if actual.Revenue != 0 {
			margin = actual.Profit * 10000 / actual.Revenue
		}
		evaluation := SelectionOutcomeEvaluation{TenantID: tenantID, CandidateID: actual.CandidateID, AnalysisID: analysis.ID, Recommendation: analysis.Recommendation, OverallScore: analysis.OverallScore, PredictedMarginBPS: cost.EstimatedMarginBPS, Views: actual.Views, Inquiries: actual.Inquiries, Orders: actual.Orders, Refunds: actual.Refunds, RealizedProfit: actual.Profit, ActualMarginBPS: margin, EvaluatedAt: now}
		if err := s.DB.WithContext(ctx).Where("candidate_id = ? AND analysis_id = ?", actual.CandidateID, analysis.ID).Assign(evaluation).FirstOrCreate(&evaluation).Error; err != nil {
			return nil, err
		}
		evaluations = append(evaluations, evaluation)
	}
	byRecommendation := map[string]*SelectionPerformanceGroup{}
	for _, evaluation := range evaluations {
		group := byRecommendation[evaluation.Recommendation]
		if group == nil {
			group = &SelectionPerformanceGroup{Recommendation: evaluation.Recommendation}
			byRecommendation[evaluation.Recommendation] = group
		}
		group.Tested++
		if evaluation.Orders > 0 {
			group.WithOrders++
		}
		if evaluation.RealizedProfit > 0 {
			group.PositiveProfit++
		}
		group.TotalProfit += evaluation.RealizedProfit
	}
	groups := []SelectionPerformanceGroup{}
	for _, key := range []string{"strong_recommend", "recommend", "watch", "reject"} {
		if group := byRecommendation[key]; group != nil {
			groups = append(groups, *group)
		}
	}
	return &SelectionPerformanceReport{GeneratedAt: now, Groups: groups, Evaluations: evaluations}, nil
}
