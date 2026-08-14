package collect

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

func openCollectWorkerTenantTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:collect_worker_tenant_%s?mode=memory&cache=shared", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CollectTask{}, &CollectTaskEvent{}))
	return db
}

func createWorkerTenantTask(t *testing.T, db *gorm.DB, tenantID int64) CollectTask {
	t.Helper()
	task := CollectTask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		TenantID:       tenantID,
		Source:         "1688",
		SourceURL:      "https://detail.1688.com/offer/1.html",
		Status:         StatusPending,
		MaxRetries:     3,
	}
	require.NoError(t, db.Create(&task).Error)
	return task
}

func queuePayloadForTask(t *testing.T, taskID uuid.UUID) string {
	t.Helper()
	return fmt.Sprintf(`{"taskId":%q,"source":"1688","url":"https://detail.1688.com/offer/1.html","requestId":"test"}`, taskID.String())
}

func TestCollectWorkerDevelopmentAllowsLegacyTenantZero(t *testing.T) {
	db := openCollectWorkerTenantTestDB(t)
	task := createWorkerTenantTask(t, db, 0)
	cfg := &config.Config{AppEnv: config.EnvDevelopment}
	svc := &Service{DB: db, ResolveWorkerTenantID: cfg.ResolveRequestTenantID}

	called := false
	processCollectQueuePayload(context.Background(), nil, svc, queuePayloadForTask(t, task.ID), 1, "worker-test", func(ctx context.Context, id uuid.UUID, workerID string) {
		called = true
		require.Equal(t, task.ID, id)
		tc := security.FromContext(ctx)
		require.NotNil(t, tc)
		require.EqualValues(t, 0, tc.TenantID)
		require.Equal(t, security.AuthSourceWorker, tc.AuthSource)
	})

	require.True(t, called)
	var row CollectTask
	require.NoError(t, db.First(&row, "id = ?", task.ID).Error)
	require.Equal(t, StatusPending, row.Status)
}

func TestCollectWorkerProductionRejectsLegacyTenantZeroAndMarksFailed(t *testing.T) {
	db := openCollectWorkerTenantTestDB(t)
	task := createWorkerTenantTask(t, db, 0)
	cfg := &config.Config{AppEnv: config.EnvProduction}
	svc := &Service{DB: db, ResolveWorkerTenantID: cfg.ResolveRequestTenantID}

	processCollectQueuePayload(context.Background(), nil, svc, queuePayloadForTask(t, task.ID), 1, "worker-test", func(context.Context, uuid.UUID, string) {
		t.Fatal("production tenant zero must not execute")
	})

	var row CollectTask
	require.NoError(t, db.First(&row, "id = ?", task.ID).Error)
	require.Equal(t, StatusFailed, row.Status)
	require.NotNil(t, row.FinishedAt)
	require.NotEmpty(t, row.ErrorMessage)
	var event CollectTaskEvent
	require.NoError(t, db.Where("task_id = ? AND event_type = ?", task.ID, EventTaskFailed).First(&event).Error)
	var count int64
	require.NoError(t, db.Model(&CollectTask{}).Where("id = ?", task.ID).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestCollectWorkerTenantResolverFailureDoesNotLeavePending(t *testing.T) {
	db := openCollectWorkerTenantTestDB(t)
	task := createWorkerTenantTask(t, db, 0)
	svc := &Service{
		DB: db,
		ResolveWorkerTenantID: func(int64) (int64, string, error) {
			return 0, "", errors.New("tenant resolver unavailable")
		},
	}

	processCollectQueuePayload(context.Background(), nil, svc, queuePayloadForTask(t, task.ID), 1, "worker-test", nil)

	var row CollectTask
	require.NoError(t, db.First(&row, "id = ?", task.ID).Error)
	require.Equal(t, StatusFailed, row.Status)
	require.Contains(t, row.ErrorMessage, "tenant resolver unavailable")
}

func TestCollectWorkerDuplicateDeliveryRunsAtMostOnceAfterClaim(t *testing.T) {
	db := openCollectWorkerTenantTestDB(t)
	task := createWorkerTenantTask(t, db, 0)
	cfg := &config.Config{AppEnv: config.EnvDevelopment}
	svc := &Service{DB: db, ResolveWorkerTenantID: cfg.ResolveRequestTenantID}
	runs := 0
	run := func(ctx context.Context, id uuid.UUID, workerID string) {
		if _, _, ok := svc.tryClaimCollectTask(ctx, id, workerID, svc.collectLeaseTTL()); !ok {
			return
		}
		runs++
	}
	payload := queuePayloadForTask(t, task.ID)
	processCollectQueuePayload(context.Background(), nil, svc, payload, 1, "worker-test", run)
	processCollectQueuePayload(context.Background(), nil, svc, payload, 2, "worker-test-2", run)

	require.Equal(t, 1, runs)
	var count int64
	require.NoError(t, db.Model(&CollectTask{}).Where("id = ?", task.ID).Count(&count).Error)
	require.EqualValues(t, 1, count)
}
