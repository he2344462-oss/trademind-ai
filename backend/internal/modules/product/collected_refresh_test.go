package product

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpsertCollectedDraftRefreshesSKUsIdempotentlyAndPreservesSalePrice(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:collected_refresh?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Product{}, &ProductSKU{}, &ProductImage{}))
	sourceID := uuid.New()
	salePrice := 17.70
	oldCost := 5.20
	row := Product{TenantID: 0, SourceProductID: &sourceID, Source: "1688", SourceURL: "https://detail.1688.com/offer/997929387210.html", Title: "catalog", OriginalTitle: "catalog", Currency: "CNY", Status: StatusDraft, SalePrice: &salePrice, PurchaseCost: &oldCost}
	require.NoError(t, db.Create(&row).Error)
	require.NoError(t, db.Create(&ProductSKU{ProductID: row.ID, SKUCode: "old-color", Price: &oldCost, CostPrice: &oldCost}).Error)

	attrsA, _ := json.Marshal(map[string]string{"颜色": "红色", "尺码": "24"})
	attrsB, _ := json.Marshal(map[string]string{"颜色": "黑色", "尺码": "25"})
	stockA, stockB := 12, 0
	costA, costB := 5.50, 6.50
	params := ImportDraftParams{TenantID: 0, SourceProductID: &sourceID, Source: "1688", SourceURL: row.SourceURL, Title: "refreshed", Currency: "CNY", FullNormalizedJSON: json.RawMessage(`{"source":"1688"}`), SKUs: []ImportSKUParams{
		{SKUCode: "101", SKUName: "红色 / 24", Attrs: attrsA, Price: &costA, CostPrice: &costA, Stock: &stockA, ImageURL: "https://cbu01.alicdn.com/red.jpg"},
		{SKUCode: "104", SKUName: "黑色 / 25", Attrs: attrsB, Price: &costB, CostPrice: &costB, Stock: &stockB, ImageURL: "https://cbu01.alicdn.com/black.jpg"},
	}}

	svc := &Service{DB: db}
	first, created, err := svc.UpsertCollectedDraftWithContext(context.Background(), nil, params)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, row.ID, first.ID)
	require.Len(t, first.SKUs, 2)
	require.Equal(t, salePrice, *first.SKUs[0].Price)
	require.Equal(t, costA, *first.SKUs[0].CostPrice)
	require.Equal(t, 0, *first.SKUs[1].Stock)

	second, created, err := svc.UpsertCollectedDraftWithContext(context.Background(), nil, params)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, row.ID, second.ID)
	require.Len(t, second.SKUs, 2)
	var count int64
	require.NoError(t, db.Model(&ProductSKU{}).Where("product_id = ?", row.ID).Count(&count).Error)
	require.EqualValues(t, 2, count)
}
