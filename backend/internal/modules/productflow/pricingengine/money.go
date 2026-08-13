package pricingengine

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Money stores CNY amounts in integer fen. Critical calculations never use float64.
type Money int64

func ParseMoney(value string) (Money, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return 0, nil
	}
	negative := strings.HasPrefix(v, "-")
	if negative {
		v = strings.TrimPrefix(v, "-")
	}
	parts := strings.Split(v, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("invalid money %q", value)
	}
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid money %q", value)
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 2 {
		return 0, fmt.Errorf("money supports at most 2 decimal places")
	}
	fraction += strings.Repeat("0", 2-len(fraction))
	fen := int64(0)
	if fraction != "" {
		fen, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid money %q", value)
		}
	}
	out := Money(yuan*100 + fen)
	if negative {
		out = -out
	}
	return out, nil
}

func MoneyFromLegacyFloat(value *float64) (Money, bool, error) {
	if value == nil {
		return 0, false, nil
	}
	m, err := ParseMoney(strconv.FormatFloat(*value, 'f', 2, 64))
	return m, true, err
}

func (m Money) String() string {
	sign := ""
	v := int64(m)
	if v < 0 {
		sign, v = "-", -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}

func (m Money) MarshalJSON() ([]byte, error) { return json.Marshal(m.String()) }

func (m *Money) UnmarshalJSON(data []byte) error {
	if m == nil {
		return fmt.Errorf("money target is nil")
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("money must be encoded as a decimal string: %w", err)
	}
	parsed, err := ParseMoney(value)
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}

func ceilDiv(numerator, denominator int64) int64 {
	if denominator <= 0 {
		return 0
	}
	if numerator >= 0 {
		return (numerator + denominator - 1) / denominator
	}
	return numerator / denominator
}

func rateAmount(amount Money, basisPoints int64) Money {
	return Money(ceilDiv(int64(amount)*basisPoints, 10000))
}
