package productflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/pricingengine"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func ParseFreightMoney(value string) (pricingengine.Money, error) {
	return pricingengine.ParseMoney(value)
}

type FreightEvidenceInput struct {
	Status            string
	FreightAmount     *int64
	OrderFreight      *int64
	Currency          string
	Destination       string
	Quantity          int
	Source            string
	ConfidenceBPS     int64
	ObservedAt        time.Time
	RawSnapshot       json.RawMessage
	CalculationMethod string
}

func normalizeFreightEvidence(in FreightEvidenceInput) (FreightEvidenceInput, error) {
	in.Status = strings.ToLower(strings.TrimSpace(in.Status))
	switch in.Status {
	case FreightStatusVerified, FreightStatusEstimated, FreightStatusFreeShipping, FreightStatusUnknown:
	default:
		return in, fmt.Errorf("invalid freight status %q", in.Status)
	}
	if in.Quantity < 1 {
		in.Quantity = 1
	}
	if in.Currency == "" {
		in.Currency = "CNY"
	}
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	in.Destination = strings.TrimSpace(in.Destination)
	in.Source = strings.TrimSpace(in.Source)
	if in.Source == "" {
		in.Source = "unknown"
	}
	in.CalculationMethod = strings.TrimSpace(in.CalculationMethod)
	if in.CalculationMethod == "" {
		in.CalculationMethod = "unknown"
	}
	if in.ConfidenceBPS < 0 || in.ConfidenceBPS > 10000 {
		return in, fmt.Errorf("freight confidence must be between 0 and 10000 bps")
	}
	if (in.FreightAmount != nil && *in.FreightAmount < 0) || (in.OrderFreight != nil && *in.OrderFreight < 0) {
		return in, fmt.Errorf("freight amount must be non-negative")
	}
	if in.Status == FreightStatusUnknown {
		in.FreightAmount = nil
		in.OrderFreight = nil
		in.ConfidenceBPS = 0
	}
	if in.Status == FreightStatusFreeShipping {
		zero := int64(0)
		in.FreightAmount = &zero
		in.OrderFreight = &zero
	}
	if in.OrderFreight != nil && in.Status != FreightStatusUnknown && in.Status != FreightStatusFreeShipping {
		unit := (*in.OrderFreight + int64(in.Quantity) - 1) / int64(in.Quantity)
		in.FreightAmount = &unit
	}
	if (in.Status == FreightStatusVerified || in.Status == FreightStatusEstimated) && in.FreightAmount == nil {
		return in, fmt.Errorf("known freight status requires an amount")
	}
	if in.ObservedAt.IsZero() {
		in.ObservedAt = time.Now().UTC()
	}
	if len(in.RawSnapshot) == 0 || !json.Valid(in.RawSnapshot) {
		in.RawSnapshot = json.RawMessage(`{}`)
	}
	return in, nil
}

func freightFingerprint(sourceProductID string, in FreightEvidenceInput) string {
	payload, _ := json.Marshal(map[string]any{
		"sourceProductId": sourceProductID,
		"status":          in.Status, "freightAmount": in.FreightAmount, "orderFreight": in.OrderFreight,
		"currency": in.Currency, "destination": in.Destination, "quantity": in.Quantity,
		"source": in.Source, "confidenceBps": in.ConfidenceBPS, "method": in.CalculationMethod,
		"raw": json.RawMessage(in.RawSnapshot),
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func freightIsOperatorConfirmed(row SourceProduct) bool {
	return (row.FreightStatus == FreightStatusVerified || row.FreightStatus == FreightStatusFreeShipping) &&
		(strings.HasPrefix(row.FreightSource, "manual") || row.FreightSource == "legacy_operator")
}

func (s *Service) persistFreightEvidence(ctx context.Context, source *SourceProduct, raw FreightEvidenceInput) error {
	if s == nil || s.DB == nil || source == nil {
		return nil
	}
	in, err := normalizeFreightEvidence(raw)
	if err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fingerprint := freightFingerprint(source.ID.String(), in)
		snapshot := SourceProductFreightSnapshot{
			TenantID: source.TenantID, SourceProductID: source.ID, Status: in.Status,
			FreightAmount: in.FreightAmount, OrderFreight: in.OrderFreight, Currency: in.Currency,
			Destination: in.Destination, Quantity: in.Quantity, Source: in.Source,
			ConfidenceBPS: in.ConfidenceBPS, ObservedAt: in.ObservedAt,
			RawSnapshot: datatypes.JSON(in.RawSnapshot), CalculationMethod: in.CalculationMethod,
			Fingerprint: fingerprint,
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "fingerprint"}}, DoNothing: true}).Create(&snapshot).Error; err != nil {
			return err
		}

		var locked SourceProduct
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id = ?", source.ID).Error; err != nil {
			return err
		}
		if freightIsOperatorConfirmed(locked) || (in.Status == FreightStatusUnknown && locked.FreightStatus != "" && locked.FreightStatus != FreightStatusUnknown) {
			*source = locked
			return nil
		}
		var legacy *float64
		if in.FreightAmount != nil {
			value := float64(*in.FreightAmount) / 100
			legacy = &value
		}
		updates := map[string]any{
			"freight": legacy, "freight_status": in.Status, "freight_amount": in.FreightAmount,
			"freight_order_amount": in.OrderFreight, "freight_currency": in.Currency,
			"freight_destination": in.Destination, "freight_quantity": in.Quantity,
			"freight_source": in.Source, "freight_confidence_bps": in.ConfidenceBPS,
			"freight_observed_at": in.ObservedAt, "freight_raw_snapshot": datatypes.JSON(in.RawSnapshot),
			"freight_calculation_method": in.CalculationMethod,
		}
		if err := tx.Model(&locked).Updates(updates).Error; err != nil {
			return err
		}
		return tx.First(source, "id = ?", source.ID).Error
	})
}
