package settings

import (
	"strconv"
	"strings"
)

type ProcurementDefaults struct {
	Destination string
	Quantity    int
}

func ProcurementDefaultsFromMap(values map[string]string) ProcurementDefaults {
	out := ProcurementDefaults{Quantity: 1}
	if values == nil {
		return out
	}
	out.Destination = strings.TrimSpace(values["default_purchase_destination"])
	if quantity, err := strconv.Atoi(strings.TrimSpace(values["default_purchase_quantity"])); err == nil && quantity > 0 && quantity <= 100000 {
		out.Quantity = quantity
	}
	return out
}
