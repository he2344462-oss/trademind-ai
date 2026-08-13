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
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/selectionengine"
	"gorm.io/datatypes"
)

const (
	SignalDemandScore      = "demand_score"
	SignalCompetitionScore = "competition_score"
	SignalPriceMedian      = "price_median"
	SignalListingCount     = "listing_count"
	SignalTrendValue       = "trend_value"

	SignalOriginOfficial   = "official"
	SignalOriginAuthorized = "authorized"
	SignalOriginPublic     = "public"
	SignalOriginManual     = "manual"
	SignalOriginImport     = "csv_import"
	SignalOriginFixture    = "fixture"
)

type CreateMarketSignalBody struct {
	CandidateID   uuid.UUID       `json:"candidateId"`
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

type ImportMarketSignalsBody struct {
	Items []CreateMarketSignalBody `json:"items"`
	CSV   string                   `json:"csv"`
}

type MarketSignalImportResult struct {
	Imported   int            `json:"imported"`
	Duplicates int            `json:"duplicates"`
	Failed     map[int]string `json:"failed"`
}

type MarketSignalView struct {
	MarketSignalSnapshot
	Freshness           string `json:"freshness"`
	EffectiveConfidence int64  `json:"effectiveConfidenceBps"`
	Participates        bool   `json:"participates"`
}

type AggregatedSignal struct {
	Score         int64              `json:"score"`
	ConfidenceBPS int64              `json:"confidenceBps"`
	Freshness     string             `json:"freshness"`
	Sources       []MarketSignalView `json:"sources"`
}

type AggregatedMarketSignals struct {
	Demand        *AggregatedSignal `json:"demand,omitempty"`
	Competition   *AggregatedSignal `json:"competition,omitempty"`
	CoverageBPS   int64             `json:"coverageBps"`
	ConfidenceBPS int64             `json:"confidenceBps"`
}

type MarketSignalFetchRequest struct {
	TenantID    int64
	CandidateID uuid.UUID
	Platform    string
	Config      json.RawMessage
}

type MarketSignalProviderHealth struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// MarketSignalProvider is the legal-source adapter boundary. Providers return only signals they actually own.
type MarketSignalProvider interface {
	ID() string
	Name() string
	SourceType() string
	SupportedPlatforms() []string
	SupportedSignals() []string
	Fetch(context.Context, MarketSignalFetchRequest) ([]CreateMarketSignalBody, error)
	ValidateConfig(json.RawMessage) error
	HealthCheck(context.Context, json.RawMessage) MarketSignalProviderHealth
}

type MarketSignalProviderStatus struct {
	ID                 string                     `json:"id"`
	Name               string                     `json:"name"`
	SourceType         string                     `json:"sourceType"`
	SupportedPlatforms []string                   `json:"supportedPlatforms"`
	SupportedSignals   []string                   `json:"supportedSignals"`
	Health             MarketSignalProviderHealth `json:"health"`
	Configured         bool                       `json:"configured"`
}

type manualSignalProvider struct{ sourceType string }

func (p manualSignalProvider) ID() string { return p.sourceType }
func (p manualSignalProvider) Name() string {
	if p.sourceType == SignalOriginImport {
		return "CSV 导入"
	}
	return "人工录入"
}
func (p manualSignalProvider) SourceType() string           { return p.sourceType }
func (p manualSignalProvider) SupportedPlatforms() []string { return []string{"xianyu", "taobao"} }
func (p manualSignalProvider) SupportedSignals() []string   { return supportedMarketSignalTypes() }
func (p manualSignalProvider) Fetch(context.Context, MarketSignalFetchRequest) ([]CreateMarketSignalBody, error) {
	return nil, nil
}
func (p manualSignalProvider) ValidateConfig(json.RawMessage) error { return nil }
func (p manualSignalProvider) HealthCheck(context.Context, json.RawMessage) MarketSignalProviderHealth {
	return MarketSignalProviderHealth{Status: "available", Message: "由用户提交可追溯数据"}
}

type officialPlaceholderProvider struct{}

func (officialPlaceholderProvider) ID() string                   { return "official-placeholder" }
func (officialPlaceholderProvider) Name() string                 { return "官方/授权数据 Provider" }
func (officialPlaceholderProvider) SourceType() string           { return SignalOriginOfficial }
func (officialPlaceholderProvider) SupportedPlatforms() []string { return []string{"xianyu", "taobao"} }
func (officialPlaceholderProvider) SupportedSignals() []string   { return supportedMarketSignalTypes() }
func (officialPlaceholderProvider) Fetch(context.Context, MarketSignalFetchRequest) ([]CreateMarketSignalBody, error) {
	return nil, fmt.Errorf("%w: official provider is not configured", ErrConflict)
}
func (officialPlaceholderProvider) ValidateConfig(raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "{}" {
		return fmt.Errorf("%w: authorized provider config is required", ErrValidation)
	}
	return nil
}
func (officialPlaceholderProvider) HealthCheck(context.Context, json.RawMessage) MarketSignalProviderHealth {
	return MarketSignalProviderHealth{Status: "not_configured", Message: "当前没有已授权的官方市场数据接口"}
}

func DefaultMarketSignalProviders() []MarketSignalProvider {
	return []MarketSignalProvider{manualSignalProvider{sourceType: SignalOriginManual}, manualSignalProvider{sourceType: SignalOriginImport}, officialPlaceholderProvider{}}
}

type MarketSignalFreshnessPolicy struct{ FreshFor, ValidFor time.Duration }

func DefaultMarketSignalFreshnessPolicies() map[string]MarketSignalFreshnessPolicy {
	return map[string]MarketSignalFreshnessPolicy{
		SignalDemandScore:      {FreshFor: 72 * time.Hour, ValidFor: 14 * 24 * time.Hour},
		SignalCompetitionScore: {FreshFor: 72 * time.Hour, ValidFor: 14 * 24 * time.Hour},
		SignalPriceMedian:      {FreshFor: 24 * time.Hour, ValidFor: 7 * 24 * time.Hour},
		SignalListingCount:     {FreshFor: 24 * time.Hour, ValidFor: 7 * 24 * time.Hour},
		SignalTrendValue:       {FreshFor: 24 * time.Hour, ValidFor: 7 * 24 * time.Hour},
	}
}

func supportedMarketSignalTypes() []string {
	return []string{SignalDemandScore, SignalCompetitionScore, SignalPriceMedian, SignalListingCount, SignalTrendValue}
}

func isSupportedSignalType(value string) bool {
	for _, item := range supportedMarketSignalTypes() {
		if item == value {
			return true
		}
	}
	return false
}

func isSupportedSignalOrigin(value string) bool {
	switch value {
	case SignalOriginOfficial, SignalOriginAuthorized, SignalOriginPublic, SignalOriginManual, SignalOriginImport, SignalOriginFixture:
		return true
	}
	return false
}

func signalFreshness(row MarketSignalSnapshot, now time.Time, policies map[string]MarketSignalFreshnessPolicy) string {
	policy, ok := policies[row.SignalType]
	if !ok {
		policy = MarketSignalFreshnessPolicy{FreshFor: 7 * 24 * time.Hour, ValidFor: 30 * 24 * time.Hour}
	}
	if row.ExpiresAt != nil && !row.ExpiresAt.After(now) {
		return "expired"
	}
	age := now.Sub(row.ObservedAt)
	if age > policy.ValidFor {
		return "expired"
	}
	if age > policy.FreshFor {
		return "stale"
	}
	return "fresh"
}

func sourceConfidenceCap(origin string) int64 {
	switch origin {
	case SignalOriginOfficial:
		return 10000
	case SignalOriginAuthorized:
		return 9000
	case SignalOriginPublic:
		return 7500
	case SignalOriginManual:
		return 7000
	case SignalOriginImport:
		return 6500
	case SignalOriginFixture:
		return 1000
	default:
		return 0
	}
}

func effectiveSignalConfidence(row MarketSignalSnapshot, freshness string) int64 {
	confidence := row.ConfidenceBPS
	if cap := sourceConfidenceCap(row.Origin); confidence > cap {
		confidence = cap
	}
	if freshness == "stale" {
		confidence /= 2
	}
	if freshness == "expired" || row.Origin == SignalOriginFixture {
		return 0
	}
	return confidence
}

func marketSignalFingerprint(tenantID int64, body CreateMarketSignalBody, observed time.Time) string {
	value := fmt.Sprintf("%d\x00%s\x00%s\x00%s\x00%s\x00%s", tenantID, body.CandidateID, strings.ToLower(body.Platform), strings.ToLower(body.SignalType), strings.TrimSpace(body.Source), observed.UTC().Format(time.RFC3339Nano))
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (s *Service) marketPolicies() map[string]MarketSignalFreshnessPolicy {
	if s.MarketSignalFreshness != nil {
		return s.MarketSignalFreshness
	}
	return DefaultMarketSignalFreshnessPolicies()
}

func (s *Service) CreateMarketSignal(ctx context.Context, tenantID int64, candidateID uuid.UUID, body CreateMarketSignalBody) (*MarketSignalView, error) {
	body.CandidateID = candidateID
	if _, err := s.GetCandidate(ctx, tenantID, candidateID); err != nil {
		return nil, err
	}
	body.Platform = strings.ToLower(strings.TrimSpace(body.Platform))
	body.SignalType = strings.ToLower(strings.TrimSpace(body.SignalType))
	body.Origin = strings.ToLower(strings.TrimSpace(body.Origin))
	body.Source = strings.TrimSpace(body.Source)
	if !SupportedDraftPlatform(body.Platform) || !isSupportedSignalType(body.SignalType) {
		return nil, fmt.Errorf("%w: unsupported market signal", ErrValidation)
	}
	if !isSupportedSignalOrigin(body.Origin) {
		return nil, fmt.Errorf("%w: unsupported signal origin", ErrValidation)
	}
	if (body.SignalType == SignalDemandScore || body.SignalType == SignalCompetitionScore) && (body.Value < 0 || body.Value > 100) {
		return nil, fmt.Errorf("%w: normalized score must be 0..100", ErrValidation)
	}
	if body.Value < 0 || body.ConfidenceBPS < 0 || body.ConfidenceBPS > 10000 || body.Source == "" {
		return nil, fmt.Errorf("%w: invalid value, confidence or source", ErrValidation)
	}
	observed := time.Now().UTC()
	if body.ObservedAt != nil {
		observed = body.ObservedAt.UTC()
	}
	if observed.After(time.Now().UTC().Add(5 * time.Minute)) {
		return nil, fmt.Errorf("%w: observedAt is in the future", ErrValidation)
	}
	raw := datatypes.JSON([]byte("{}"))
	if len(body.RawData) > 0 {
		if !json.Valid(body.RawData) {
			return nil, fmt.Errorf("%w: invalid rawData", ErrValidation)
		}
		raw = datatypes.JSON(body.RawData)
	}
	unit := strings.TrimSpace(body.Unit)
	if unit == "" {
		if body.SignalType == SignalDemandScore || body.SignalType == SignalCompetitionScore {
			unit = "score_0_100"
		} else {
			unit = "count"
		}
	}
	row := MarketSignalSnapshot{TenantID: tenantID, CandidateID: candidateID, Platform: body.Platform, SignalType: body.SignalType, Value: body.Value, Unit: unit, Source: body.Source, Origin: body.Origin, ConfidenceBPS: body.ConfidenceBPS, ObservedAt: observed, ExpiresAt: body.ExpiresAt, RawData: raw, Fingerprint: marketSignalFingerprint(tenantID, body, observed)}
	if err := s.DB.WithContext(ctx).Create(&row).Error; err != nil {
		if isUniqueError(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	freshness := signalFreshness(row, time.Now().UTC(), s.marketPolicies())
	effective := effectiveSignalConfidence(row, freshness)
	return &MarketSignalView{MarketSignalSnapshot: row, Freshness: freshness, EffectiveConfidence: effective, Participates: effective >= 3000}, nil
}

func (s *Service) ListMarketSignals(ctx context.Context, tenantID int64, candidateID uuid.UUID, platform string) ([]MarketSignalView, error) {
	var rows []MarketSignalSnapshot
	db := s.DB.WithContext(ctx).Where("tenant_id = ? AND candidate_id = ?", tenantID, candidateID)
	if platform = strings.ToLower(strings.TrimSpace(platform)); platform != "" {
		db = db.Where("platform = ?", platform)
	}
	if err := db.Order("observed_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]MarketSignalView, 0, len(rows))
	now := time.Now().UTC()
	for _, row := range rows {
		freshness := signalFreshness(row, now, s.marketPolicies())
		effective := effectiveSignalConfidence(row, freshness)
		out = append(out, MarketSignalView{MarketSignalSnapshot: row, Freshness: freshness, EffectiveConfidence: effective, Participates: effective >= 3000})
	}
	return out, nil
}

func (s *Service) AggregateMarketSignals(ctx context.Context, tenantID int64, candidateID uuid.UUID, platform string) (*AggregatedMarketSignals, []MarketSignalView, error) {
	views, err := s.ListMarketSignals(ctx, tenantID, candidateID, platform)
	if err != nil {
		return nil, nil, err
	}
	result := &AggregatedMarketSignals{}
	for signalType, target := range map[string]**AggregatedSignal{SignalDemandScore: &result.Demand, SignalCompetitionScore: &result.Competition} {
		var weighted, weight int64
		sources := []MarketSignalView{}
		freshness := "expired"
		for _, view := range views {
			if view.SignalType != signalType || !view.Participates {
				continue
			}
			weighted += view.Value * view.EffectiveConfidence
			weight += view.EffectiveConfidence
			sources = append(sources, view)
			if view.Freshness == "fresh" {
				freshness = "fresh"
			} else if freshness == "expired" {
				freshness = "stale"
			}
		}
		if weight > 0 {
			confidence := weight / int64(len(sources))
			if confidence > 10000 {
				confidence = 10000
			}
			*target = &AggregatedSignal{Score: (weighted + weight/2) / weight, ConfidenceBPS: confidence, Freshness: freshness, Sources: sources}
		}
	}
	covered, confidence := int64(0), int64(0)
	for _, item := range []*AggregatedSignal{result.Demand, result.Competition} {
		if item != nil {
			covered++
			confidence += item.ConfidenceBPS
		}
	}
	result.CoverageBPS = covered * 5000
	if covered > 0 {
		result.ConfidenceBPS = confidence / covered
	}
	return result, views, nil
}

func (s *Service) marketDimensions(ctx context.Context, tenantID int64, candidateID uuid.UUID, platform string) (map[string]selectionengine.MarketDimension, []MarketSignalView, error) {
	aggregated, views, err := s.AggregateMarketSignals(ctx, tenantID, candidateID, platform)
	if err != nil {
		return nil, nil, err
	}
	out := map[string]selectionengine.MarketDimension{}
	for key, item := range map[string]*AggregatedSignal{"demand": aggregated.Demand, "competition": aggregated.Competition} {
		if item == nil {
			continue
		}
		evidence := make([]string, 0, len(item.Sources))
		for _, source := range item.Sources {
			evidence = append(evidence, fmt.Sprintf("%s / %s，观测于 %s，可信度 %.0f%%", source.Origin, source.Source, source.ObservedAt.Format(time.RFC3339), float64(source.EffectiveConfidence)/100))
		}
		out[key] = selectionengine.MarketDimension{Score: item.Score, ConfidenceBPS: item.ConfidenceBPS, Freshness: item.Freshness, Evidence: evidence}
	}
	return out, views, nil
}

func parseMarketSignalCSV(input string) ([]CreateMarketSignalBody, error) {
	r := csv.NewReader(strings.NewReader(input))
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("%w: invalid csv header", ErrValidation)
	}
	index := map[string]int{}
	for i, name := range header {
		index[strings.TrimSpace(strings.ToLower(name))] = i
	}
	required := []string{"candidate_id", "platform", "signal_type", "value", "observed_at", "source"}
	for _, key := range required {
		if _, ok := index[key]; !ok {
			return nil, fmt.Errorf("%w: missing csv column %s", ErrValidation, key)
		}
	}
	items := []CreateMarketSignalBody{}
	for {
		record, readErr := r.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("%w: invalid csv row", ErrValidation)
		}
		get := func(key string) string {
			i, ok := index[key]
			if !ok || i >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[i])
		}
		id, e := uuid.Parse(get("candidate_id"))
		if e != nil {
			return nil, fmt.Errorf("%w: invalid candidate_id", ErrValidation)
		}
		value, e := strconv.ParseInt(get("value"), 10, 64)
		if e != nil {
			return nil, fmt.Errorf("%w: invalid value", ErrValidation)
		}
		observed, e := time.Parse(time.RFC3339, get("observed_at"))
		if e != nil {
			return nil, fmt.Errorf("%w: invalid observed_at", ErrValidation)
		}
		confidence := int64(6000)
		if text := get("confidence_bps"); text != "" {
			confidence, e = strconv.ParseInt(text, 10, 64)
			if e != nil {
				return nil, fmt.Errorf("%w: invalid confidence_bps", ErrValidation)
			}
		}
		items = append(items, CreateMarketSignalBody{CandidateID: id, Platform: get("platform"), SignalType: get("signal_type"), Value: value, Unit: get("unit"), Source: get("source"), Origin: SignalOriginImport, ConfidenceBPS: confidence, ObservedAt: &observed})
	}
	return items, nil
}

func (s *Service) ImportMarketSignals(ctx context.Context, tenantID int64, body ImportMarketSignalsBody) (*MarketSignalImportResult, error) {
	items := append([]CreateMarketSignalBody{}, body.Items...)
	if strings.TrimSpace(body.CSV) != "" {
		parsed, err := parseMarketSignalCSV(body.CSV)
		if err != nil {
			return nil, err
		}
		items = append(items, parsed...)
	}
	if len(items) == 0 || len(items) > 1000 {
		return nil, fmt.Errorf("%w: import requires 1..1000 items", ErrValidation)
	}
	result := &MarketSignalImportResult{Failed: map[int]string{}}
	for i, item := range items {
		if item.Origin == "" {
			item.Origin = SignalOriginImport
		}
		_, err := s.CreateMarketSignal(ctx, tenantID, item.CandidateID, item)
		if err == nil {
			result.Imported++
		} else if err == ErrConflict {
			result.Duplicates++
		} else {
			result.Failed[i] = truncateError(err)
		}
	}
	return result, nil
}

func (s *Service) MarketSignalProviderStatuses(ctx context.Context, tenantID int64) ([]MarketSignalProviderStatus, error) {
	configs := map[string]MarketSignalProviderConfig{}
	var rows []MarketSignalProviderConfig
	if err := s.DB.WithContext(ctx).Where("tenant_id = ?", tenantID).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		configs[row.ProviderID] = row
	}
	providers := s.MarketSignalProviders
	if len(providers) == 0 {
		providers = DefaultMarketSignalProviders()
	}
	out := make([]MarketSignalProviderStatus, 0, len(providers))
	for _, provider := range providers {
		config, configured := configs[provider.ID()]
		raw := json.RawMessage(config.Config)
		health := provider.HealthCheck(ctx, raw)
		if provider.ID() == SignalOriginManual || provider.ID() == SignalOriginImport {
			configured = true
		}
		out = append(out, MarketSignalProviderStatus{ID: provider.ID(), Name: provider.Name(), SourceType: provider.SourceType(), SupportedPlatforms: provider.SupportedPlatforms(), SupportedSignals: provider.SupportedSignals(), Health: health, Configured: configured && (config.Enabled || provider.ID() == SignalOriginManual || provider.ID() == SignalOriginImport)})
	}
	return out, nil
}
