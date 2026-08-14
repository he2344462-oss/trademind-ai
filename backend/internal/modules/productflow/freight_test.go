package productflow

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func freightTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SourceProduct{}, &SourceProductFreightSnapshot{}))
	return db
}

func createFreightSource(t *testing.T, db *gorm.DB) SourceProduct {
	t.Helper()
	price := 6.50
	row := SourceProduct{TenantID: 0, SourcePlatform: "1688", SourceProductID: "offer:test", SourceURL: "https://detail.1688.com/offer/1.html", OriginalTitle: "test", SourcePrice: &price, CollectedAt: time.Now().UTC(), Status: SourceStatusCollected, FreightStatus: FreightStatusUnknown, FreightCurrency: "CNY", FreightQuantity: 1}
	require.NoError(t, db.Create(&row).Error)
	return row
}

func TestFreightEvidenceLifecycleAndIdempotency(t *testing.T) {
	db := freightTestDB(t)
	row := createFreightSource(t, db)
	svc := &Service{DB: db}
	order := int64(800)
	raw := json.RawMessage(`{"text":"送至天津运费8元"}`)
	in := FreightEvidenceInput{Status: FreightStatusVerified, OrderFreight: &order, Currency: "CNY", Destination: "天津", Quantity: 10, Source: "official_page_dom", ConfidenceBPS: 9000, ObservedAt: time.Now().UTC(), RawSnapshot: raw, CalculationMethod: "official_order_freight_divided_by_quantity"}
	require.NoError(t, svc.persistFreightEvidence(context.Background(), &row, in))
	require.EqualValues(t, 80, *row.FreightAmount)
	require.Equal(t, 0.80, *row.Freight)

	require.NoError(t, svc.persistFreightEvidence(context.Background(), &row, in))
	var count int64
	require.NoError(t, db.Model(&SourceProductFreightSnapshot{}).Count(&count).Error)
	require.EqualValues(t, 1, count)

	order = 1000
	in.OrderFreight = &order
	require.NoError(t, svc.persistFreightEvidence(context.Background(), &row, in))
	require.NoError(t, db.Model(&SourceProductFreightSnapshot{}).Count(&count).Error)
	require.EqualValues(t, 2, count)
	require.EqualValues(t, 100, *row.FreightAmount)
}

func TestFreightFreeUnknownAndOperatorPreservation(t *testing.T) {
	db := freightTestDB(t)
	row := createFreightSource(t, db)
	svc := &Service{DB: db}
	require.NoError(t, svc.persistFreightEvidence(context.Background(), &row, FreightEvidenceInput{Status: FreightStatusFreeShipping, Source: "official_page_dom", ConfidenceBPS: 10000, RawSnapshot: json.RawMessage(`{"text":"包邮"}`)}))
	require.EqualValues(t, 0, *row.FreightAmount)

	manual := int64(250)
	require.NoError(t, db.Model(&row).Updates(map[string]any{"freight_status": FreightStatusVerified, "freight_amount": manual, "freight": 2.50, "freight_source": "manual_confirmed", "freight_confidence_bps": 10000}).Error)
	require.NoError(t, db.First(&row, "id = ?", row.ID).Error)
	require.NoError(t, svc.persistFreightEvidence(context.Background(), &row, FreightEvidenceInput{Status: FreightStatusUnknown, Source: "official_page_dom", RawSnapshot: json.RawMessage(`{"text":"运费2元起"}`)}))
	require.EqualValues(t, 250, *row.FreightAmount)
	require.Equal(t, "manual_confirmed", row.FreightSource)
}
