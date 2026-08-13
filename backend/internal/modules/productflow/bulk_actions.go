package productflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
)

type BulkActionResult struct {
	Completed []uuid.UUID       `json:"completed"`
	Failed    map[string]string `json:"failed"`
}

func (s *Service) hasBlockers(ctx context.Context, tenantID int64, candidateID uuid.UUID) (bool, error) {
	analysis, err := s.LatestAnalysis(ctx, tenantID, candidateID)
	if err == ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var blockers []string
	if err := json.Unmarshal(analysis.Blockers, &blockers); err != nil {
		return false, err
	}
	return len(blockers) > 0, nil
}

func (s *Service) BulkCandidateAction(ctx context.Context, tenantID int64, actorID *uuid.UUID, action string, body BulkCandidateActionBody) (*BulkActionResult, error) {
	if len(body.CandidateIDs) == 0 || len(body.CandidateIDs) > 500 {
		return nil, fmt.Errorf("%w: candidateIds must contain 1..500 items", ErrValidation)
	}
	source := strings.TrimSpace(body.Source)
	if source == "" {
		source = "manual"
	}
	if source != "manual" && source != "batch_recommendation" {
		return nil, fmt.Errorf("%w: unsupported approval source", ErrValidation)
	}
	out := &BulkActionResult{Completed: []uuid.UUID{}, Failed: map[string]string{}}
	for _, id := range body.CandidateIDs {
		var err error
		switch action {
		case "watch":
			_, err = s.WatchCandidate(ctx, tenantID, id)
		case "reject":
			_, err = s.RejectCandidate(ctx, tenantID, id, body.Reason)
		case "approve":
			blocked, blockErr := s.hasBlockers(ctx, tenantID, id)
			if blockErr != nil {
				err = blockErr
			} else if blocked && !body.AllowBlocked {
				err = fmt.Errorf("candidate has blocker")
			} else if blocked && strings.TrimSpace(body.Reason) == "" {
				err = fmt.Errorf("blocked approval requires reason")
			} else {
				_, err = s.ApproveCandidate(ctx, tenantID, id)
			}
		default:
			return nil, fmt.Errorf("%w: unsupported bulk action", ErrValidation)
		}
		if err != nil {
			out.Failed[id.String()] = err.Error()
		} else {
			out.Completed = append(out.Completed, id)
		}
	}
	if s.OpLog != nil {
		payload, _ := json.Marshal(map[string]any{"source": source, "completed": out.Completed, "failed": out.Failed, "reason": body.Reason})
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{TenantID: tenantID, AdminUserID: actorID, Username: "user", Action: "candidate.bulk." + action, Resource: "candidate", ResourceID: fmt.Sprintf("%d items", len(body.CandidateIDs)), Status: "completed", Message: string(payload)})
	}
	return out, nil
}

func (s *Service) ImportSources(ctx context.Context, tenantID int64, body BulkSourceImportBody) ([]SourceProduct, error) {
	if len(body.Items) == 0 || len(body.Items) > 500 {
		return nil, fmt.Errorf("%w: items must contain 1..500 products", ErrValidation)
	}
	rows := make([]SourceProduct, 0, len(body.Items))
	for _, item := range body.Items {
		row, err := s.CreateSource(ctx, tenantID, item)
		if err != nil {
			return nil, err
		}
		rows = append(rows, *row)
	}
	return rows, nil
}
