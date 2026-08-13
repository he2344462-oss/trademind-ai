package database

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
)

func TestSprint5MigrationIsIdempotent(t *testing.T) {
	if os.Getenv("TEST_DATABASE_URL") == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	harness := postgrestest.Require(t)
	require.NoError(t, AutoMigrate(harness.DB))
	require.NoError(t, AutoMigrate(harness.DB))
	require.True(t, harness.DB.Migrator().HasTable(&productflow.SelectionConfig{}))
	require.True(t, harness.DB.Migrator().HasTable(&productflow.ListingContentVersion{}))
	require.True(t, harness.DB.Migrator().HasTable(&productflow.ListingAsset{}))
	require.True(t, harness.DB.Migrator().HasTable(&productflow.ListingPublishPackage{}))
	require.True(t, harness.DB.Migrator().HasTable(&productflow.ManualPublishRecord{}))
	require.True(t, harness.DB.Migrator().HasColumn(&productflow.CandidateAnalysis{}, "selection_config_version"))
	require.True(t, harness.DB.Migrator().HasColumn(&productflow.MarketSignalProviderConfig{}, "credential_reference"))
	require.True(t, harness.DB.Migrator().HasIndex(&productflow.MarketSignalProviderConfig{}, "idx_market_provider_platform"))
}
