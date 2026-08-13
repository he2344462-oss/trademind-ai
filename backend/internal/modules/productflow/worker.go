package productflow

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
)

func StartAnalysisWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, reg *worker.Registry) {
	if svc == nil || svc.Redis == nil || svc.Redis.Client == nil || !svc.AnalysisQueueEnabled {
		return
	}
	_ = svc.RecoverStaleAnalysisItems(context.Background(), time.Now().UTC().Add(-10*time.Minute))
	_ = svc.ResumeAnalysisBatches(context.Background())
	concurrency := svc.AnalysisConcurrency
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 32 {
		concurrency = 32
	}
	for slot := 1; slot <= concurrency; slot++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			workerID := fmt.Sprintf("candidate-analysis-%d", slot)
			if reg != nil {
				inst := reg.Register(ctx, worker.TypeCandidateAnalysis, workerID, map[string]any{"queue": svc.batchQueueName()})
				if inst != nil {
					workerID = inst.WorkerID()
					defer inst.Stop(context.Background())
				}
			}
			for {
				res, err := svc.Redis.BRPop(ctx, 5*time.Second, svc.batchQueueName()).Result()
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					continue
				}
				if len(res) < 2 {
					continue
				}
				var msg analysisQueueMessage
				if json.Unmarshal([]byte(res[1]), &msg) != nil {
					if log != nil {
						log.Warn("candidate_analysis_bad_message", "worker", slot)
					}
					continue
				}
				id, err := uuid.Parse(strings.TrimSpace(msg.BatchID))
				if err != nil {
					continue
				}
				svc.RunAnalysisBatch(ctx, id, workerID)
			}
		}(slot)
	}
}

func StartListingOperationWorkers(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service) {
	if svc == nil || svc.Redis == nil || svc.Redis.Client == nil || !svc.ContentQueueEnabled {
		return
	}
	_ = svc.RecoverListingOperationBatches(context.Background())
	_ = svc.ResumeListingOperationBatches(context.Background())
	starts := []struct {
		operation   string
		concurrency int
	}{{ListingOperationContent, svc.ContentConcurrency}, {ListingOperationPackage, svc.PackageConcurrency}}
	for _, start := range starts {
		concurrency := start.concurrency
		if concurrency < 1 {
			concurrency = 1
		}
		if concurrency > 16 {
			concurrency = 16
		}
		for slot := 1; slot <= concurrency; slot++ {
			operation := start.operation
			wg.Add(1)
			go func(slot int) {
				defer wg.Done()
				workerID := fmt.Sprintf("listing-%s-%d", operation, slot)
				for {
					res, err := svc.Redis.BRPop(ctx, 5*time.Second, svc.listingQueueName(operation)).Result()
					if err != nil {
						if ctx.Err() != nil {
							return
						}
						continue
					}
					if len(res) < 2 {
						continue
					}
					var msg listingQueueMessage
					if json.Unmarshal([]byte(res[1]), &msg) != nil {
						if log != nil {
							log.Warn("listing_operation_bad_message", "operation", operation)
						}
						continue
					}
					id, err := uuid.Parse(strings.TrimSpace(msg.BatchID))
					if err != nil {
						continue
					}
					svc.RunListingOperationBatch(ctx, id, workerID)
				}
			}(slot)
		}
	}
}
