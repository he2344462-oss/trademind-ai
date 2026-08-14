package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aioperationbatch"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproductimage"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/backup"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectbrowserprofile"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/disasterrecovery"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventorysyncp9"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/performance"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/pricingengine"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/release"
	"github.com/trademind-ai/trademind/backend/internal/modules/restore"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"gorm.io/gorm"
)

// migrateLegacyPublicationSKUColumns renames GORM-default product_sk_uid / external_sk_uid
// to product_sku_id / external_sku_id so raw SQL and API field names stay consistent.
func migrateLegacyPublicationSKUColumns(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("product_publication_skus") {
		return nil
	}
	dst := &productpublish.ProductPublicationSKU{}
	if db.Migrator().HasColumn(dst, "product_sk_uid") && !db.Migrator().HasColumn(dst, "product_sku_id") {
		if err := db.Migrator().RenameColumn(dst, "product_sk_uid", "product_sku_id"); err != nil {
			return fmt.Errorf("rename product_publication_skus.product_sk_uid: %w", err)
		}
	}
	if db.Migrator().HasColumn(dst, "external_sk_uid") && !db.Migrator().HasColumn(dst, "external_sku_id") {
		if err := db.Migrator().RenameColumn(dst, "external_sk_uid", "external_sku_id"); err != nil {
			return fmt.Errorf("rename product_publication_skus.external_sk_uid: %w", err)
		}
	}
	return nil
}

// migrateLegacyInventorySKUColumns renames early GORM typo columns (product_sk_uid / external_sk_uid)
// and ensures inventory / order SKU linkage columns exist before raw SQL aggregations run.
func migrateLegacyInventorySKUColumns(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	type spec struct {
		model     any
		legacyCol string
		newCol    string
	}
	renames := []spec{
		{&inventory.InventorySyncTask{}, "product_sk_uid", "product_sku_id"},
		{&inventory.InventoryChangeLog{}, "product_sk_uid", "product_sku_id"},
		{&inventory.OrderInventoryEffect{}, "product_sk_uid", "product_sku_id"},
		{&order.OrderItem{}, "product_sk_uid", "product_sku_id"},
		{&order.OrderItem{}, "external_sk_uid", "external_sku_id"},
	}
	for _, r := range renames {
		if !db.Migrator().HasTable(r.model) {
			continue
		}
		if db.Migrator().HasColumn(r.model, r.legacyCol) && !db.Migrator().HasColumn(r.model, r.newCol) {
			if err := db.Migrator().RenameColumn(r.model, r.legacyCol, r.newCol); err != nil {
				return fmt.Errorf("rename %T.%s -> %s: %w", r.model, r.legacyCol, r.newCol, err)
			}
		}
	}
	// Ensure current models add any still-missing columns (product_sku_id, external_sku_id, …).
	return db.AutoMigrate(
		&inventory.InventorySyncTask{},
		&inventory.InventoryChangeLog{},
		&inventory.OrderInventoryEffect{},
		&order.OrderItem{},
	)
}

// migrateLegacyProductTextColumns ensures AI text columns exist on older product tables.
func migrateLegacyProductTextColumns(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	return db.AutoMigrate(&product.Product{})
}

func migrateLegacyFreightState(db *gorm.DB) error {
	var sources []productflow.SourceProduct
	if err := db.Where("freight IS NOT NULL AND (freight_status = ? OR freight_status = '')", productflow.FreightStatusUnknown).Find(&sources).Error; err != nil {
		return err
	}
	for _, row := range sources {
		if row.Freight == nil {
			continue
		}
		money, _, err := pricingengine.MoneyFromLegacyFloat(row.Freight)
		if err != nil {
			return err
		}
		cents := int64(money)
		if err := db.Model(&row).Updates(map[string]any{"freight_status": productflow.FreightStatusVerified, "freight_amount": cents, "freight_currency": "CNY", "freight_quantity": 1, "freight_source": "legacy_operator", "freight_confidence_bps": 10000, "freight_calculation_method": "legacy_operator_value"}).Error; err != nil {
			return err
		}
	}
	var products []product.Product
	if err := db.Where("freight_cost IS NOT NULL AND (freight_status = ? OR freight_status = '')", productflow.FreightStatusUnknown).Find(&products).Error; err != nil {
		return err
	}
	for _, row := range products {
		if row.FreightCost == nil {
			continue
		}
		money, _, err := pricingengine.MoneyFromLegacyFloat(row.FreightCost)
		if err != nil {
			return err
		}
		cents := int64(money)
		if err := db.Model(&row).Updates(map[string]any{"freight_status": productflow.FreightStatusVerified, "freight_amount": cents, "freight_currency": "CNY", "freight_quantity": 1, "freight_source": "legacy_operator", "freight_confidence_bps": 10000, "freight_calculation_method": "legacy_operator_value"}).Error; err != nil {
			return err
		}
	}
	return nil
}

// AutoMigrate applies schema for core foundation tables.
func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("auto migrate: db is nil")
	}
	if err := migrateLegacyPublicationSKUColumns(db); err != nil {
		return err
	}
	if err := migrateLegacyInventorySKUColumns(db); err != nil {
		return err
	}
	if err := migrateLegacyProductTextColumns(db); err != nil {
		return err
	}
	// Sprint 5 scopes provider uniqueness by platform. Drop the legacy two-column
	// index before AutoMigrate creates the new tenant/provider/platform index.
	if db.Migrator().HasTable(&productflow.MarketSignalProviderConfig{}) && db.Migrator().HasIndex(&productflow.MarketSignalProviderConfig{}, "idx_market_provider") {
		if err := db.Migrator().DropIndex(&productflow.MarketSignalProviderConfig{}, "idx_market_provider"); err != nil {
			return fmt.Errorf("drop legacy market provider index: %w", err)
		}
	}
	if err := db.AutoMigrate(
		&admin.AdminUser{},
		&admin.UserStorePermission{},
		&settings.Setting{},
		&operationlog.OperationLog{},
		&files.FileRecord{},
		&imagetask.ImageTask{},
		&imagetask.ImageTaskItem{},
		&product.Product{},
		&product.ProductImage{},
		&product.ProductSKU{},
		&product.ProductPlatformPublishConfig{},
		&product.ProductAIContentApplication{},
		&product.ProductImageApplication{},
		&productflow.SourceProduct{},
		&productflow.SourceProductFreightSnapshot{},
		&productflow.Candidate{},
		&productflow.CandidateAnalysis{},
		&productflow.ListingDraft{},
		&productflow.PricingProfile{},
		&productflow.PricingProfileRevision{},
		&productflow.MarketSignalSnapshot{},
		&productflow.MarketSignalProviderConfig{},
		&productflow.CandidateAnalysisBatch{},
		&productflow.CandidateAnalysisBatchItem{},
		&productflow.ListingPerformanceSnapshot{},
		&productflow.SelectionOutcomeEvaluation{},
		&productflow.SelectionConfig{},
		&productflow.ListingContentVersion{},
		&productflow.ListingAsset{},
		&productflow.ListingPublishPackage{},
		&productflow.ManualPublishRecord{},
		&productflow.ListingContentQualityReview{},
		&productflow.ListingOperationBatch{},
		&productflow.ListingOperationBatchItem{},
		&productpublish.ProductPublishTask{},
		&productpublish.ProductPublishBatch{},
		&productpublish.ProductPublication{},
		&productpublish.ProductPublicationSKU{},
		&order.Order{},
		&order.OrderItem{},
		&order.OrderItemSKUMatch{},
		&orderexception.OrderExceptionMark{},
		&ordersync.OrderSyncTask{},
		&customersync.CustomerMessageSyncTask{},
		&inventory.InventorySyncBatch{},
		&inventory.InventorySyncTask{},
		&inventory.InventoryChangeLog{},
		&inventory.OrderInventoryEffect{},
		&shop.Shop{},
		&shop.ShopAuthToken{},
		&shop.PlatformCategory{},
		&shop.PlatformCategoryAttribute{},
		&worker.Instance{},
		&collect.CollectBatch{},
		&collect.CollectTask{},
		&collect.CollectTaskEvent{},
		&collectrule.CollectRule{},
		&collectbrowserprofile.CollectBrowserProfile{},
		&aiprompt.AIPrompt{},
		&aitask.AITask{},
		&aioperationbatch.AIOperationBatch{},
		&aiproducttext.AIProductTextBatch{},
		&aiproducttext.AIProductTextItem{},
		&aiproductimage.AIProductImageBatch{},
		&aiproductimage.AIProductImageItem{},
		&customerchat.CustomerConversation{},
		&customerchat.CustomerMessage{},
		&customerchat.CustomerReplySuggestion{},
		&customerchat.CustomerFailureEvent{},
		&taskcenter.TaskFailureMark{},
		&taskcenter.TaskAlert{},
		&taskcenter.TaskAlertNotification{},
		&backup.Job{},
		&backup.Artifact{},
		&backup.Verification{},
		&backup.RetentionHold{},
		&backup.ObjectInventory{},
		&restore.Job{},
		&restore.Validation{},
		&release.Run{},
		&release.Artifact{},
		&release.Step{},
		&release.Rollback{},
		&disasterrecovery.Drill{},
		&performance.TestRun{},
		&performance.Regression{},
		&performance.CapacitySnapshot{},
		&performance.RateLimitPolicy{},
		&performance.QuotaPolicy{},
	); err != nil {
		return err
	}
	if err := migrateLegacyFreightState(db); err != nil {
		return fmt.Errorf("migrate legacy freight state: %w", err)
	}
	if err := operationtask.Migrate(db); err != nil {
		return err
	}
	if err := inventorysyncp9.Migrate(db); err != nil {
		return err
	}
	if err := migrateDouyinPhase102Indexes(db); err != nil {
		return err
	}
	if err := migratePublishBatchA21(db); err != nil {
		return err
	}
	if err := migrateP2Reliability(db); err != nil {
		return err
	}
	if err := migrateP21Reliability(db); err != nil {
		return err
	}
	if err := migrateP22Reliability(db); err != nil {
		return err
	}
	if err := migrateP3Douyin(db); err != nil {
		return err
	}
	if err := migrateP31Douyin(db); err != nil {
		return err
	}
	if err := migrateP32Webhook(db); err != nil {
		return err
	}
	if err := migrateP4Security(db); err != nil {
		return err
	}
	if err := migrateP41Security(db); err != nil {
		return err
	}
	if err := migrateP42Security(db); err != nil {
		return err
	}
	if err := migrateP5Observability(db); err != nil {
		return err
	}
	return migrateP7Performance(db)
}
