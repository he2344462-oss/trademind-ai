package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasktenant"
)

func normalizeCollectConcurrency(n int) int {
	if n < 1 {
		return 1
	}
	if n > 32 {
		return 32
	}
	return n
}

// StartWorker runs BRPOP consumers until ctx is cancelled.
func StartWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, queueName string, concurrency int, reg *worker.Registry) {
	if svc == nil || svc.Redis == nil || svc.Redis.Client == nil {
		return
	}
	if queueName == "" {
		queueName = "collect:tasks"
	}
	concurrency = normalizeCollectConcurrency(concurrency)

	SetCollectWorkersRunning(true)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			var wid string
			if reg != nil {
				inst := reg.Register(ctx, worker.TypeCollect, fmt.Sprintf("collect-%d", slot), map[string]any{"queue": queueName})
				if inst != nil {
					defer inst.Stop(context.Background())
					wid = inst.WorkerID()
				}
			}
			if wid == "" {
				wid = worker.GenerateWorkerID(worker.TypeCollect)
			}
			runCollectWorker(ctx, log, svc, queueName, slot, wid)
		}(i + 1)
	}
}

func runCollectWorker(ctx context.Context, log *slog.Logger, svc *Service, queueName string, slot int, workerLeaseID string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		res, err := svc.Redis.BRPop(ctx, 5*time.Second, queueName).Result()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		if len(res) < 2 {
			continue
		}
		processCollectQueuePayload(context.Background(), log, svc, res[1], slot, workerLeaseID, svc.RunCollectJob)
	}
}

type collectJobRunner func(context.Context, uuid.UUID, string)

func processCollectQueuePayload(ctx context.Context, log *slog.Logger, svc *Service, payload string, slot int, workerLeaseID string, run collectJobRunner) {
	var msg QueueMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		if log != nil {
			log.Warn("collect_worker_bad_message", "worker", slot, "error", err)
		}
		return
	}
	tid, err := uuid.Parse(strings.TrimSpace(msg.TaskID))
	if err != nil {
		if log != nil {
			log.Warn("collect_worker_bad_task_id", "worker", slot, "error", err)
		}
		return
	}
	if svc == nil || svc.DB == nil {
		return
	}

	var task CollectTask
	if err := svc.DB.WithContext(ctx).First(&task, "id = ?", tid).Error; err != nil {
		if log != nil {
			log.Warn("collect_worker_task_lookup_failed", "worker", slot, "taskId", tid.String(), "error", err)
		}
		return
	}

	jobCtx, terr := svc.collectWorkerContext(ctx, &task)
	if terr != nil {
		svc.failUnclaimedTask(ctx, &task, "TASK_TENANT_CONTEXT_INVALID", tasktenant.WrapError(terr))
		if log != nil {
			log.Warn("collect_worker_tenant_missing", "worker", slot, "taskId", tid.String(), "error", tasktenant.WrapError(terr))
		}
		return
	}
	if run != nil {
		run(jobCtx, tid, workerLeaseID)
	}
}

func (s *Service) collectWorkerContext(ctx context.Context, task *CollectTask) (context.Context, error) {
	if task == nil {
		return ctx, fmt.Errorf("collect task missing")
	}
	tenantID := task.TenantID
	resolutionSource := ""
	if s != nil && s.ResolveWorkerTenantID != nil {
		resolved, source, err := s.ResolveWorkerTenantID(tenantID)
		if err != nil {
			return ctx, err
		}
		tenantID = resolved
		resolutionSource = source
	}
	if tenantID == 0 && resolutionSource == "legacy_dev_zero" {
		return tasktenant.BuildWorkerContext(tasktenant.TaskScope{TenantID: 0}, uuid.Nil, "collect"), nil
	}
	wctx, _, err := tasktenant.BeginWorker(ctx, s.DB, tenantID, uuid.Nil, "collect")
	return wctx, err
}

func (s *Service) failUnclaimedTask(ctx context.Context, task *CollectTask, errorCode, message string) {
	if s == nil || s.DB == nil || task == nil {
		return
	}
	now := time.Now().UTC()
	fromStatus := task.Status
	result := s.DB.WithContext(ctx).Model(&CollectTask{}).
		Where("id = ? AND status IN ?", task.ID, []string{StatusPending, StatusRetrying}).
		Updates(map[string]any{
			"status":        StatusFailed,
			"error_message": truncateRunes(strings.TrimSpace(message), 8000),
			"finished_at":   &now,
			"locked_by":     nil,
			"locked_until":  nil,
			"updated_at":    now,
		})
	if result.Error != nil || result.RowsAffected == 0 {
		return
	}
	task.Status = StatusFailed
	task.FinishedAt = &now
	task.ErrorMessage = truncateRunes(strings.TrimSpace(message), 8000)
	s.RecordTaskEvent(ctx, task, TaskEventInput{
		EventType:    EventTaskFailed,
		FromStatus:   fromStatus,
		ToStatus:     StatusFailed,
		Message:      "collect worker initialization failed",
		ErrorMessage: task.ErrorMessage,
		RetryCount:   task.RetryCount,
		MaxRetries:   s.effectiveMaxRetries(task),
		PayloadMap:   map[string]any{"errorCode": errorCode, "retryable": false},
	})
	s.reconcileCollectBatch(ctx, task.BatchID)
}
