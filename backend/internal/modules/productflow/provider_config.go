package productflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/credentialstore"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const providerHealthTimeout = 8 * time.Second

type MarketProviderConfigInput struct {
	ProviderType        string          `json:"providerType" binding:"required"`
	Name                string          `json:"name" binding:"required"`
	Platform            string          `json:"platform" binding:"required"`
	SourceLevel         string          `json:"sourceLevel" binding:"required"`
	Config              json.RawMessage `json:"config"`
	CredentialReference string          `json:"credentialReference"`
}

type MarketProviderConfigView struct {
	ID                   uuid.UUID       `json:"id"`
	ProviderID           string          `json:"providerId"`
	ProviderType         string          `json:"providerType"`
	Name                 string          `json:"name"`
	Platform             string          `json:"platform"`
	SourceLevel          string          `json:"sourceLevel"`
	Enabled              bool            `json:"enabled"`
	Status               string          `json:"status"`
	Config               json.RawMessage `json:"config"`
	CredentialConfigured bool            `json:"credentialConfigured"`
	CredentialHint       string          `json:"credentialHint,omitempty"`
	LastHealthCheckAt    *time.Time      `json:"lastHealthCheckAt,omitempty"`
	LastSuccessAt        *time.Time      `json:"lastSuccessAt,omitempty"`
	LastError            string          `json:"lastError,omitempty"`
	CreatedAt            time.Time       `json:"createdAt"`
	UpdatedAt            time.Time       `json:"updatedAt"`
}

func (s *Service) credentialStore() credentialstore.Store {
	if s.CredentialStore != nil {
		return s.CredentialStore
	}
	return credentialstore.EnvironmentStore{}
}

func providerByID(s *Service, id string) MarketSignalProvider {
	providers := s.MarketSignalProviders
	if len(providers) == 0 {
		providers = DefaultMarketSignalProviders()
	}
	for _, provider := range providers {
		if provider.ID() == id {
			return provider
		}
	}
	return nil
}

func configContainsSecret(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return true
	}
	var walk func(any) bool
	walk = func(v any) bool {
		switch item := v.(type) {
		case map[string]any:
			for key, nested := range item {
				lower := strings.ToLower(key)
				for _, fragment := range []string{"secret", "token", "password", "api_key", "apikey", "credential"} {
					if strings.Contains(lower, fragment) {
						return true
					}
				}
				if walk(nested) {
					return true
				}
			}
		case []any:
			for _, nested := range item {
				if walk(nested) {
					return true
				}
			}
		}
		return false
	}
	return walk(value)
}

func validateProviderInput(s *Service, input MarketProviderConfigInput) (MarketSignalProvider, error) {
	input.ProviderType = strings.TrimSpace(input.ProviderType)
	input.Platform = strings.ToLower(strings.TrimSpace(input.Platform))
	input.SourceLevel = strings.ToLower(strings.TrimSpace(input.SourceLevel))
	provider := providerByID(s, input.ProviderType)
	if provider == nil || !isSupportedSignalOrigin(input.SourceLevel) || (input.Platform != "xianyu" && input.Platform != "taobao") || strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("%w: invalid provider configuration", ErrValidation)
	}
	if provider.ID() == SignalOriginManual && input.SourceLevel != SignalOriginManual {
		return nil, fmt.Errorf("%w: manual provider source level mismatch", ErrValidation)
	}
	if provider.ID() == SignalOriginImport && input.SourceLevel != SignalOriginImport {
		return nil, fmt.Errorf("%w: CSV provider source level mismatch", ErrValidation)
	}
	if provider.ID() == "official-placeholder" && input.SourceLevel != SignalOriginOfficial && input.SourceLevel != SignalOriginAuthorized {
		return nil, fmt.Errorf("%w: official provider source level mismatch", ErrValidation)
	}
	if configContainsSecret(input.Config) {
		return nil, fmt.Errorf("%w: secrets must use credentialReference", ErrValidation)
	}
	if len(input.Config) == 0 {
		input.Config = json.RawMessage(`{}`)
	}
	if err := provider.ValidateConfig(input.Config); err != nil && provider.ID() != "official-placeholder" {
		return nil, fmt.Errorf("%w: provider config", ErrValidation)
	}
	if strings.TrimSpace(input.CredentialReference) != "" {
		if _, err := s.credentialStore().Inspect(context.Background(), input.CredentialReference); err != nil {
			return nil, fmt.Errorf("%w: invalid credential reference", ErrValidation)
		}
	}
	return provider, nil
}

func (s *Service) providerView(ctx context.Context, row MarketSignalProviderConfig) MarketProviderConfigView {
	meta, _ := s.credentialStore().Inspect(ctx, row.CredentialReference)
	return MarketProviderConfigView{ID: row.ID, ProviderID: row.ProviderID, ProviderType: row.ProviderType, Name: row.Name, Platform: row.Platform, SourceLevel: row.SourceType, Enabled: row.Enabled, Status: row.HealthStatus, Config: json.RawMessage(row.Config), CredentialConfigured: meta.Configured, CredentialHint: meta.Hint, LastHealthCheckAt: row.LastCheckedAt, LastSuccessAt: row.LastSuccessAt, LastError: row.LastError, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func (s *Service) ListMarketProviderConfigs(ctx context.Context, tenantID int64) ([]MarketProviderConfigView, error) {
	var rows []MarketSignalProviderConfig
	if err := s.DB.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]MarketProviderConfigView, 0, len(rows))
	for _, row := range rows {
		out = append(out, s.providerView(ctx, row))
	}
	return out, nil
}

func (s *Service) SaveMarketProviderConfig(ctx context.Context, tenantID int64, id *uuid.UUID, input MarketProviderConfigInput) (*MarketProviderConfigView, error) {
	input.ProviderType = strings.TrimSpace(input.ProviderType)
	input.Name = strings.TrimSpace(input.Name)
	input.Platform = strings.ToLower(strings.TrimSpace(input.Platform))
	input.SourceLevel = strings.ToLower(strings.TrimSpace(input.SourceLevel))
	input.CredentialReference = strings.TrimSpace(input.CredentialReference)
	provider, err := validateProviderInput(s, input)
	if err != nil {
		return nil, err
	}
	raw := datatypes.JSON(input.Config)
	if len(raw) == 0 {
		raw = datatypes.JSON([]byte(`{}`))
	}
	row := MarketSignalProviderConfig{TenantID: tenantID, ProviderID: provider.ID(), ProviderType: provider.ID(), Name: strings.TrimSpace(input.Name), Platform: strings.ToLower(strings.TrimSpace(input.Platform)), SourceType: strings.ToLower(strings.TrimSpace(input.SourceLevel)), Config: raw, CredentialReference: strings.TrimSpace(input.CredentialReference), HealthStatus: "not_configured"}
	if id == nil {
		if err := s.DB.WithContext(ctx).Create(&row).Error; err != nil {
			if isUniqueError(err) {
				return nil, ErrConflict
			}
			return nil, err
		}
	} else {
		var current MarketSignalProviderConfig
		if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, *id).First(&current).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, ErrNotFound
			}
			return nil, err
		}
		row.ID = current.ID
		row.Enabled = current.Enabled
		if row.CredentialReference == "" {
			row.CredentialReference = current.CredentialReference
		}
		if err := s.DB.WithContext(ctx).Model(&current).Updates(map[string]any{"provider_id": row.ProviderID, "provider_type": row.ProviderType, "name": row.Name, "platform": row.Platform, "source_type": row.SourceType, "config": row.Config, "credential_reference": row.CredentialReference, "health_status": "not_configured", "last_error": ""}).Error; err != nil {
			return nil, err
		}
		row = current
		if err := s.DB.WithContext(ctx).Where("id = ?", current.ID).First(&row).Error; err != nil {
			return nil, err
		}
	}
	view := s.providerView(ctx, row)
	return &view, nil
}

func (s *Service) SetMarketProviderEnabled(ctx context.Context, tenantID int64, id uuid.UUID, enabled bool) (*MarketProviderConfigView, error) {
	status := "disabled"
	if enabled {
		status = "not_configured"
	}
	var row MarketSignalProviderConfig
	res := s.DB.WithContext(ctx).Model(&row).Where("tenant_id = ? AND id = ?", tenantID, id).Updates(map[string]any{"enabled": enabled, "health_status": status})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
		return nil, err
	}
	view := s.providerView(ctx, row)
	return &view, nil
}

func sanitizeProviderError(message string, secrets ...[]byte) string {
	for _, secret := range secrets {
		if len(secret) > 0 {
			message = strings.ReplaceAll(message, string(secret), "[REDACTED]")
		}
	}
	message = strings.ReplaceAll(message, "\n", " ")
	if len(message) > 512 {
		message = message[:512]
	}
	return message
}

func (s *Service) HealthCheckMarketProvider(ctx context.Context, tenantID int64, id uuid.UUID) (*MarketProviderConfigView, error) {
	var row MarketSignalProviderConfig
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	provider := providerByID(s, row.ProviderType)
	if provider == nil {
		return nil, fmt.Errorf("%w: provider adapter unavailable", ErrValidation)
	}
	checkCtx, cancel := context.WithTimeout(ctx, providerHealthTimeout)
	defer cancel()
	secret, secretErr := s.credentialStore().Resolve(checkCtx, row.CredentialReference)
	meta, _ := s.credentialStore().Inspect(checkCtx, row.CredentialReference)
	health := MarketSignalProviderHealth{Status: "unhealthy", Message: "credential is not configured"}
	if row.ProviderType == SignalOriginManual || row.ProviderType == SignalOriginImport {
		health = provider.HealthCheck(checkCtx, json.RawMessage(row.Config))
	} else if secretErr == nil && meta.Configured {
		health = provider.HealthCheck(checkCtx, json.RawMessage(row.Config))
	}
	status := health.Status
	switch status {
	case "healthy", "degraded", "unhealthy", "not_configured":
	default:
		if status == "available" {
			status = "healthy"
		} else {
			status = "unhealthy"
		}
	}
	if !row.Enabled {
		status = "disabled"
	}
	now := time.Now().UTC()
	updates := map[string]any{"health_status": status, "last_checked_at": now, "last_error": ""}
	if status == "healthy" {
		updates["last_success_at"] = now
	} else {
		updates["last_error"] = sanitizeProviderError(health.Message, secret)
	}
	if err := s.DB.WithContext(ctx).Model(&row).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	view := s.providerView(ctx, row)
	return &view, nil
}
