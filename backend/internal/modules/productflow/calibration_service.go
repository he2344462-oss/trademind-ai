package productflow

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/calibration"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/selectionengine"
	"gorm.io/gorm"
)

type calibrationActual struct {
	CandidateID                                        uuid.UUID
	Views, Inquiries, Orders, Refunds, Revenue, Profit int64
}

func (s *Service) CalibrationReport(ctx context.Context, tenantID int64, includeTestData bool) (*calibration.Report, error) {
	query := s.DB.WithContext(ctx).Table("listing_performance_snapshots").Select("candidate_id, COALESCE(SUM(views),0) views, COALESCE(SUM(inquiries),0) inquiries, COALESCE(SUM(orders),0) orders, COALESCE(SUM(refund_count),0) refunds, COALESCE(SUM(gross_revenue),0) revenue, COALESCE(SUM(realized_profit),0) profit").Where("tenant_id = ?", tenantID)
	if !includeTestData {
		query = query.Where("source <> ?", SignalOriginFixture)
	}
	var actuals []calibrationActual
	if err := query.Group("candidate_id").Scan(&actuals).Error; err != nil {
		return nil, err
	}
	latestByCandidate := map[uuid.UUID]time.Time{}
	var snapshots []ListingPerformanceSnapshot
	latestQuery := s.DB.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if !includeTestData {
		latestQuery = latestQuery.Where("source <> ?", SignalOriginFixture)
	}
	if err := latestQuery.Order("observed_at ASC").Find(&snapshots).Error; err != nil {
		return nil, err
	}
	for _, snapshot := range snapshots {
		latestByCandidate[snapshot.CandidateID] = snapshot.ObservedAt
	}
	observations := make([]calibration.Observation, 0, len(actuals))
	for _, actual := range actuals {
		var analysis CandidateAnalysis
		if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND candidate_id = ?", tenantID, actual.CandidateID).Order("analysis_version DESC").First(&analysis).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return nil, err
		}
		var dims map[string]selectionengine.Dimension
		_ = json.Unmarshal(analysis.ScoreBreakdown, &dims)
		scores := map[string]*int64{}
		for key, dimension := range dims {
			scores[key] = dimension.Score
		}
		var warnings, blockers []string
		_ = json.Unmarshal(analysis.Warnings, &warnings)
		_ = json.Unmarshal(analysis.Blockers, &blockers)
		observations = append(observations, calibration.Observation{Recommendation: analysis.Recommendation, OverallScore: analysis.OverallScore, Dimensions: scores, Warnings: warnings, Blockers: blockers, Views: actual.Views, Inquiries: actual.Inquiries, Orders: actual.Orders, Refunds: actual.Refunds, Revenue: actual.Revenue, Profit: actual.Profit, ObservedAt: latestByCandidate[actual.CandidateID]})
	}
	report := calibration.Build(observations, includeTestData)
	return &report, nil
}
