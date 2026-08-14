package collect

import (
	"context"
	"encoding/json"

	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
)

func (s *Service) buildFreightRequestOptions(ctx context.Context, tenantID int64) []byte {
	defaults := settings.ProcurementDefaults{Quantity: 1}
	if s != nil && s.Settings != nil {
		if values, err := s.Settings.PlainByGroup(ctx, tenantID, "procurement"); err == nil {
			defaults = settings.ProcurementDefaultsFromMap(values)
		}
	}
	body := map[string]any{"freightQuantity": defaults.Quantity}
	if defaults.Destination != "" {
		body["freightDestination"] = defaults.Destination
	}
	encoded, _ := json.Marshal(body)
	return encoded
}
