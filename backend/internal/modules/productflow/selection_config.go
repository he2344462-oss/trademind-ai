package productflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/pricingengine"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/selectionengine"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SelectionConfigDraft    = "draft"
	SelectionConfigReview   = "review"
	SelectionConfigActive   = "active"
	SelectionConfigArchived = "archived"
	DefaultSelectionVersion = "selection-v3-default"
)

type SelectionThresholds struct {
	StrongRecommend         int64 `json:"strongRecommend"`
	Recommend               int64 `json:"recommend"`
	Watch                   int64 `json:"watch"`
	MinimumMarginBPS        int64 `json:"minimumMarginBps"`
	MinimumProfit           int64 `json:"minimumProfit"`
	StaleAfterDays          int   `json:"staleAfterDays"`
	StrongMarketCoverageBPS int64 `json:"strongMarketCoverageBps,omitempty"`
}

type SelectionBlockers struct {
	SensitiveKeywords []string `json:"sensitiveKeywords"`
	BlockerKeywords   []string `json:"blockerKeywords"`
}

type SelectionConfigInput struct {
	Version    string              `json:"version" binding:"required"`
	Weights    map[string]int64    `json:"weights" binding:"required"`
	Thresholds SelectionThresholds `json:"thresholds" binding:"required"`
	Blockers   SelectionBlockers   `json:"blockers" binding:"required"`
}

func defaultSelectionConfig() selectionengine.Config { return selectionengine.DefaultConfig() }

func selectionRow(version, status string, tenantID int64, cfg selectionengine.Config, createdBy *uuid.UUID) SelectionConfig {
	weights, _ := json.Marshal(cfg.Weights)
	thresholds, _ := json.Marshal(SelectionThresholds{StrongRecommend: cfg.StrongRecommendThreshold, Recommend: cfg.RecommendThreshold, Watch: cfg.WatchThreshold, MinimumMarginBPS: cfg.MinimumMarginBPS, MinimumProfit: int64(cfg.MinimumProfit), StaleAfterDays: cfg.StaleAfterDays, StrongMarketCoverageBPS: cfg.StrongMarketCoverageBPS})
	blockers, _ := json.Marshal(SelectionBlockers{SensitiveKeywords: cfg.SensitiveKeywords, BlockerKeywords: cfg.BlockerKeywords})
	return SelectionConfig{TenantID: tenantID, Version: version, Weights: datatypes.JSON(weights), Thresholds: datatypes.JSON(thresholds), Blockers: datatypes.JSON(blockers), Status: status, CreatedBy: createdBy}
}

func validateSelectionConfig(cfg selectionengine.Config) error {
	required := []string{"profit", "data_quality", "supply", "risk", "platform_fit", "demand", "competition"}
	var total int64
	for _, key := range required {
		weight, ok := cfg.Weights[key]
		if !ok || weight < 0 || weight > 100 {
			return fmt.Errorf("%w: invalid weight %s", ErrValidation, key)
		}
		total += weight
	}
	if total != 100 || cfg.StrongRecommendThreshold < cfg.RecommendThreshold || cfg.RecommendThreshold < cfg.WatchThreshold || cfg.StrongRecommendThreshold > 100 || cfg.WatchThreshold < 0 || cfg.MinimumMarginBPS < 0 || cfg.MinimumProfit < 0 || cfg.StaleAfterDays < 1 || cfg.StrongMarketCoverageBPS < 0 || cfg.StrongMarketCoverageBPS > 10000 {
		return fmt.Errorf("%w: invalid selection thresholds", ErrValidation)
	}
	return nil
}

func configFromRow(row SelectionConfig) (selectionengine.Config, error) {
	var weights map[string]int64
	var thresholds SelectionThresholds
	var blockers SelectionBlockers
	if err := json.Unmarshal(row.Weights, &weights); err != nil {
		return selectionengine.Config{}, err
	}
	if err := json.Unmarshal(row.Thresholds, &thresholds); err != nil {
		return selectionengine.Config{}, err
	}
	if err := json.Unmarshal(row.Blockers, &blockers); err != nil {
		return selectionengine.Config{}, err
	}
	cfg := selectionengine.Config{Weights: weights, StrongRecommendThreshold: thresholds.StrongRecommend, RecommendThreshold: thresholds.Recommend, WatchThreshold: thresholds.Watch, MinimumMarginBPS: thresholds.MinimumMarginBPS, MinimumProfit: pricingengine.Money(thresholds.MinimumProfit), StaleAfterDays: thresholds.StaleAfterDays, SensitiveKeywords: blockers.SensitiveKeywords, BlockerKeywords: blockers.BlockerKeywords, StrongMarketCoverageBPS: thresholds.StrongMarketCoverageBPS}
	if cfg.StrongMarketCoverageBPS == 0 {
		cfg.StrongMarketCoverageBPS = selectionengine.DefaultConfig().StrongMarketCoverageBPS
	}
	return cfg, validateSelectionConfig(cfg)
}

func configFromInput(input SelectionConfigInput) (selectionengine.Config, error) {
	cfg := selectionengine.Config{Weights: input.Weights, StrongRecommendThreshold: input.Thresholds.StrongRecommend, RecommendThreshold: input.Thresholds.Recommend, WatchThreshold: input.Thresholds.Watch, MinimumMarginBPS: input.Thresholds.MinimumMarginBPS, MinimumProfit: pricingengine.Money(input.Thresholds.MinimumProfit), StaleAfterDays: input.Thresholds.StaleAfterDays, SensitiveKeywords: input.Blockers.SensitiveKeywords, BlockerKeywords: input.Blockers.BlockerKeywords, StrongMarketCoverageBPS: input.Thresholds.StrongMarketCoverageBPS}
	if cfg.StrongMarketCoverageBPS == 0 {
		cfg.StrongMarketCoverageBPS = selectionengine.DefaultConfig().StrongMarketCoverageBPS
	}
	return cfg, validateSelectionConfig(cfg)
}

func (s *Service) ensureDefaultSelectionConfig(ctx context.Context, tenantID int64) (*SelectionConfig, error) {
	var row SelectionConfig
	err := s.DB.WithContext(ctx).Where("tenant_id = ? AND status = ?", tenantID, SelectionConfigActive).First(&row).Error
	if err == nil {
		return &row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	row = selectionRow(DefaultSelectionVersion, SelectionConfigActive, tenantID, defaultSelectionConfig(), nil)
	now := time.Now().UTC()
	row.ActivatedAt = &now
	if err := s.DB.WithContext(ctx).Create(&row).Error; err != nil {
		if !isUniqueError(err) {
			return nil, err
		}
		if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND status = ?", tenantID, SelectionConfigActive).First(&row).Error; err != nil {
			return nil, err
		}
	}
	return &row, nil
}

func (s *Service) activeSelectionConfig(ctx context.Context, tenantID int64) (*SelectionConfig, selectionengine.Config, error) {
	row, err := s.ensureDefaultSelectionConfig(ctx, tenantID)
	if err != nil {
		return nil, selectionengine.Config{}, err
	}
	cfg, err := configFromRow(*row)
	return row, cfg, err
}

func (s *Service) ListSelectionConfigs(ctx context.Context, tenantID int64) ([]SelectionConfig, error) {
	if _, err := s.ensureDefaultSelectionConfig(ctx, tenantID); err != nil {
		return nil, err
	}
	var rows []SelectionConfig
	err := s.DB.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&rows).Error
	return rows, err
}

func (s *Service) CreateSelectionConfigDraft(ctx context.Context, tenantID int64, input SelectionConfigInput, actorID *uuid.UUID) (*SelectionConfig, error) {
	input.Version = strings.TrimSpace(input.Version)
	if input.Version == "" || len(input.Version) > 64 {
		return nil, fmt.Errorf("%w: invalid version", ErrValidation)
	}
	cfg, err := configFromInput(input)
	if err != nil {
		return nil, err
	}
	row := selectionRow(input.Version, SelectionConfigDraft, tenantID, cfg, actorID)
	if err := s.DB.WithContext(ctx).Create(&row).Error; err != nil {
		if isUniqueError(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return &row, nil
}

func (s *Service) ReviewSelectionConfig(ctx context.Context, tenantID int64, id uuid.UUID) (*SelectionConfig, error) {
	var row SelectionConfig
	res := s.DB.WithContext(ctx).Model(&row).Where("tenant_id = ? AND id = ? AND status = ?", tenantID, id, SelectionConfigDraft).Update("status", SelectionConfigReview)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrInvalidTransition
	}
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) ActivateSelectionConfig(ctx context.Context, tenantID int64, id uuid.UUID) (*SelectionConfig, error) {
	var row SelectionConfig
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var active SelectionConfig
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND status = ?", tenantID, SelectionConfigActive).First(&active).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
			return err
		}
		if row.Status != SelectionConfigReview {
			return ErrInvalidTransition
		}
		if _, err := configFromRow(row); err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&SelectionConfig{}).Where("tenant_id = ? AND status = ?", tenantID, SelectionConfigActive).Updates(map[string]any{"status": SelectionConfigArchived, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&row).Updates(map[string]any{"status": SelectionConfigActive, "activated_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		row.Status = SelectionConfigActive
		row.ActivatedAt = &now
		return nil
	})
	return &row, err
}
