package productflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/selectionengine"
	"gorm.io/datatypes"
)

const (
	SignalDemandScore      = "demand_score"
	SignalCompetitionScore = "competition_score"
	SignalOriginManual     = "manual"
	SignalOriginFixture    = "fixture"
)

type CreateMarketSignalBody struct {
	Platform      string          `json:"platform" binding:"required"`
	SignalType    string          `json:"signalType" binding:"required"`
	Value         int64           `json:"value"`
	Unit          string          `json:"unit"`
	Source        string          `json:"source" binding:"required"`
	Origin        string          `json:"origin" binding:"required"`
	ConfidenceBPS int64           `json:"confidenceBps"`
	ObservedAt    *time.Time      `json:"observedAt"`
	ExpiresAt     *time.Time      `json:"expiresAt"`
	RawData       json.RawMessage `json:"rawData"`
}

type MarketSignalView struct {
	MarketSignalSnapshot
	Freshness string `json:"freshness"`
}

// MarketSignalProvider is the extension point for future verifiable data sources.
type MarketSignalProvider interface {
	Load(context.Context, int64, uuid.UUID, string) ([]MarketSignalView, error)
}

type databaseMarketSignalProvider struct{ service *Service }

func (p databaseMarketSignalProvider) Load(ctx context.Context, tenantID int64, candidateID uuid.UUID, platform string) ([]MarketSignalView, error) {
	return p.service.ListMarketSignals(ctx, tenantID, candidateID, platform)
}

func signalFreshness(row MarketSignalSnapshot, now time.Time) string {
	if row.ExpiresAt != nil && !row.ExpiresAt.After(now) {
		return "expired"
	}
	if now.Sub(row.ObservedAt) > 7*24*time.Hour {
		return "stale"
	}
	return "fresh"
}

func (s *Service) CreateMarketSignal(ctx context.Context, tenantID int64, candidateID uuid.UUID, body CreateMarketSignalBody) (*MarketSignalView, error) {
	if _, err := s.GetCandidate(ctx, tenantID, candidateID); err != nil {
		return nil, err
	}
	body.Platform = strings.ToLower(strings.TrimSpace(body.Platform))
	body.SignalType = strings.ToLower(strings.TrimSpace(body.SignalType))
	body.Origin = strings.ToLower(strings.TrimSpace(body.Origin))
	if !SupportedDraftPlatform(body.Platform) || (body.SignalType != SignalDemandScore && body.SignalType != SignalCompetitionScore) {
		return nil, fmt.Errorf("%w: unsupported normalized market signal", ErrValidation)
	}
	if body.Origin != SignalOriginManual && body.Origin != SignalOriginFixture {
		return nil, fmt.Errorf("%w: signal origin must be manual or fixture", ErrValidation)
	}
	if body.Value < 0 || body.Value > 100 || body.ConfidenceBPS < 0 || body.ConfidenceBPS > 10000 || strings.TrimSpace(body.Source) == "" {
		return nil, fmt.Errorf("%w: invalid signal value, confidence or source", ErrValidation)
	}
	observed := time.Now().UTC()
	if body.ObservedAt != nil {
		observed = body.ObservedAt.UTC()
	}
	raw := datatypes.JSON([]byte("{}"))
	if len(body.RawData) > 0 {
		if !json.Valid(body.RawData) {
			return nil, fmt.Errorf("%w: invalid rawData", ErrValidation)
		}
		raw = datatypes.JSON(body.RawData)
	}
	row := MarketSignalSnapshot{TenantID: tenantID, CandidateID: candidateID, Platform: body.Platform, SignalType: body.SignalType, Value: body.Value, Unit: "score_0_100", Source: strings.TrimSpace(body.Source), Origin: body.Origin, ConfidenceBPS: body.ConfidenceBPS, ObservedAt: observed, ExpiresAt: body.ExpiresAt, RawData: raw}
	if err := s.DB.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	return &MarketSignalView{MarketSignalSnapshot: row, Freshness: signalFreshness(row, time.Now().UTC())}, nil
}

func (s *Service) ListMarketSignals(ctx context.Context, tenantID int64, candidateID uuid.UUID, platform string) ([]MarketSignalView, error) {
	var rows []MarketSignalSnapshot
	db := s.DB.WithContext(ctx).Where("tenant_id = ? AND candidate_id = ?", tenantID, candidateID)
	if platform = strings.ToLower(strings.TrimSpace(platform)); platform != "" {
		db = db.Where("platform = ?", platform)
	}
	if err := db.Order("observed_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]MarketSignalView, 0, len(rows))
	now := time.Now().UTC()
	for _, row := range rows {
		out = append(out, MarketSignalView{MarketSignalSnapshot: row, Freshness: signalFreshness(row, now)})
	}
	return out, nil
}

func (s *Service) marketDimensions(ctx context.Context, tenantID int64, candidateID uuid.UUID, platform string) (map[string]selectionengine.MarketDimension, []MarketSignalView, error) {
	provider := s.MarketSignals
	if provider == nil {
		provider = databaseMarketSignalProvider{service: s}
	}
	views, err := provider.Load(ctx, tenantID, candidateID, platform)
	if err != nil {
		return nil, nil, err
	}
	out := map[string]selectionengine.MarketDimension{}
	for _, view := range views {
		key := strings.TrimSuffix(view.SignalType, "_score")
		if _, exists := out[key]; exists || view.Freshness == "expired" {
			continue
		}
		confidence := view.ConfidenceBPS
		if view.Freshness == "stale" {
			confidence /= 2
		}
		out[key] = selectionengine.MarketDimension{Score: view.Value, ConfidenceBPS: confidence, Freshness: view.Freshness, Evidence: []string{fmt.Sprintf("%s数据，来源：%s", view.Origin, view.Source)}}
	}
	return out, views, nil
}
