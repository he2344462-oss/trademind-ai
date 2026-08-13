package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
)

func TestAutoMigrateAgainstIsolatedPostgres(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	_, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration migration test")
	}

	harness := postgrestest.Require(t)
	db := harness.DB

	require.NoError(t, database.AutoMigrate(db))

	for _, table := range []string{"admin_users", "products", "product_skus", "source_products", "candidates", "listing_drafts", "product_publish_tasks", "inventory_sync_tasks"} {
		require.Truef(t, db.Migrator().HasTable(table), "expected migrated table %s", table)
	}
	for _, column := range []string{"source_product_id", "candidate_id", "catalog_status", "purchase_cost", "sale_price"} {
		require.Truef(t, db.Migrator().HasColumn("products", column), "expected products.%s", column)
	}
}
