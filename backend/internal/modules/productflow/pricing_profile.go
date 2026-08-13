package productflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/pricingengine"
	"gorm.io/gorm"
)

func profileModelToEngine(row PricingProfile) pricingengine.Profile {
	return pricingengine.Profile{Code: row.ID.String(), Platform: row.Platform, Currency: row.Currency, Configured: true, PlatformFeeBPS: row.PlatformFeeBPS, PlatformFeeFixed: pricingengine.Money(row.PlatformFeeFixed), PaymentFeeBPS: row.PaymentFeeBPS, PaymentFeeFixed: pricingengine.Money(row.PaymentFeeFixed), ReturnReserveBPS: row.ReturnReserveBPS, OtherBPS: row.OtherBPS, OtherFixed: pricingengine.Money(row.OtherFixed)}
}

func pricingProfileFromInput(tenantID int64, body PricingProfileInput) (PricingProfile, error) {
	pf, err := pricingengine.ParseMoney(body.PlatformFeeFixed)
	if err != nil {
		return PricingProfile{}, err
	}
	pay, err := pricingengine.ParseMoney(body.PaymentFeeFixed)
	if err != nil {
		return PricingProfile{}, err
	}
	other, err := pricingengine.ParseMoney(body.OtherFixed)
	if err != nil {
		return PricingProfile{}, err
	}
	platform := strings.ToLower(strings.TrimSpace(body.Platform))
	currency := strings.ToUpper(strings.TrimSpace(body.Currency))
	if currency == "" {
		currency = "CNY"
	}
	if platform == "" || strings.TrimSpace(body.Name) == "" {
		return PricingProfile{}, fmt.Errorf("%w: name and platform are required", ErrValidation)
	}
	probe := pricingengine.Profile{PlatformFeeBPS: body.PlatformFeeBPS, PlatformFeeFixed: pf, PaymentFeeBPS: body.PaymentFeeBPS, PaymentFeeFixed: pay, ReturnReserveBPS: body.ReturnReserveBPS, OtherBPS: body.OtherBPS, OtherFixed: other}
	if err := pricingengine.ValidateProfile(probe); err != nil {
		return PricingProfile{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	return PricingProfile{TenantID: tenantID, Name: strings.TrimSpace(body.Name), Platform: platform, Currency: currency, PlatformFeeBPS: body.PlatformFeeBPS, PlatformFeeFixed: int64(pf), PaymentFeeBPS: body.PaymentFeeBPS, PaymentFeeFixed: int64(pay), ReturnReserveBPS: body.ReturnReserveBPS, OtherBPS: body.OtherBPS, OtherFixed: int64(other), IsDefault: body.IsDefault, Enabled: enabled}, nil
}

func (s *Service) savePricingProfile(ctx context.Context, row *PricingProfile) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if row.IsDefault {
			if err := tx.Model(&PricingProfile{}).Where("tenant_id = ? AND platform = ?", row.TenantID, row.Platform).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Save(row).Error
	})
}

func (s *Service) CreatePricingProfile(ctx context.Context, tenantID int64, body PricingProfileInput) (*PricingProfile, error) {
	row, err := pricingProfileFromInput(tenantID, body)
	if err != nil {
		return nil, err
	}
	if err = s.savePricingProfile(ctx, &row); err != nil {
		if isUniqueError(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return &row, nil
}
func (s *Service) UpdatePricingProfile(ctx context.Context, tenantID int64, id uuid.UUID, body PricingProfileInput) (*PricingProfile, error) {
	var existing PricingProfile
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	row, err := pricingProfileFromInput(tenantID, body)
	if err != nil {
		return nil, err
	}
	row.ID = existing.ID
	row.CreatedAt = existing.CreatedAt
	if err = s.savePricingProfile(ctx, &row); err != nil {
		return nil, err
	}
	return &row, nil
}
func (s *Service) ListPricingProfiles(ctx context.Context, tenantID int64, platform string) ([]PricingProfile, error) {
	var rows []PricingProfile
	db := s.DB.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if platform != "" {
		db = db.Where("platform = ?", strings.ToLower(platform))
	}
	err := db.Order("platform, is_default DESC, name").Find(&rows).Error
	return rows, err
}
func (s *Service) GetPricingProfile(ctx context.Context, tenantID int64, id uuid.UUID) (*PricingProfile, error) {
	var row PricingProfile
	err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &row, err
}
func (s *Service) DeletePricingProfile(ctx context.Context, tenantID int64, id uuid.UUID) error {
	var count int64
	s.DB.WithContext(ctx).Model(&ListingDraft{}).Where("tenant_id = ? AND pricing_profile_id = ?", tenantID, id).Count(&count)
	if count > 0 {
		return fmt.Errorf("%w: profile is referenced by listing history", ErrConflict)
	}
	result := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&PricingProfile{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) resolvePricingProfile(ctx context.Context, tenantID int64, platform string, id *uuid.UUID, fallback PricingProfileBody) (pricingengine.Profile, *uuid.UUID, error) {
	if id != nil {
		row, err := s.GetPricingProfile(ctx, tenantID, *id)
		if err != nil {
			return pricingengine.Profile{}, nil, err
		}
		if !row.Enabled || row.Platform != platform {
			return pricingengine.Profile{}, nil, fmt.Errorf("%w: pricing profile unavailable for platform", ErrValidation)
		}
		return profileModelToEngine(*row), &row.ID, nil
	}
	var row PricingProfile
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND platform = ? AND enabled = ? AND is_default = ?", tenantID, platform, true, true).First(&row).Error; err == nil {
		return profileModelToEngine(row), &row.ID, nil
	}
	p, err := profileFromBody(platform, fallback)
	if err != nil {
		return pricingengine.Profile{}, nil, err
	}
	row = PricingProfile{TenantID: tenantID, Name: platform + " 默认估算模型", Platform: platform, Currency: "CNY", PlatformFeeBPS: p.PlatformFeeBPS, PlatformFeeFixed: int64(p.PlatformFeeFixed), PaymentFeeBPS: p.PaymentFeeBPS, PaymentFeeFixed: int64(p.PaymentFeeFixed), ReturnReserveBPS: p.ReturnReserveBPS, OtherBPS: p.OtherBPS, OtherFixed: int64(p.OtherFixed), IsDefault: true, Enabled: true}
	if err := s.savePricingProfile(ctx, &row); err != nil {
		if !isUniqueError(err) {
			return pricingengine.Profile{}, nil, err
		}
		if err = s.DB.WithContext(ctx).Where("tenant_id = ? AND platform = ? AND enabled = ? AND is_default = ?", tenantID, platform, true, true).First(&row).Error; err != nil {
			return pricingengine.Profile{}, nil, err
		}
	}
	return profileModelToEngine(row), &row.ID, nil
}
