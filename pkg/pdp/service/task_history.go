package service

import (
	"context"
	"fmt"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPService) GetTaskHistory(ctx context.Context, filter *types.TaskHistoryFilter) (types.TaskHistoryResponse, error) {
	query := p.db.WithContext(ctx).Model(&models.TaskHistory{})

	// Apply filters
	if filter != nil {
		// Task ID filter
		if filter.TaskID != nil {
			query = query.Where("task_id = ?", *filter.TaskID)
		}

		// Name filter (partial match, case-insensitive)
		if filter.Name != nil {
			query = query.Where("LOWER(name) LIKE LOWER(?)", "%"+*filter.Name+"%")
		}

		// Time range filters
		if filter.CreatedAfter != nil {
			query = query.Where("posted >= ?", *filter.CreatedAfter)
		}
		if filter.CreatedBefore != nil {
			query = query.Where("posted <= ?", *filter.CreatedBefore)
		}
		if filter.StartedAfter != nil {
			query = query.Where("work_start >= ?", *filter.StartedAfter)
		}
		if filter.StartedBefore != nil {
			query = query.Where("work_start <= ?", *filter.StartedBefore)
		}
		if filter.EndedAfter != nil {
			query = query.Where("work_end >= ?", *filter.EndedAfter)
		}
		if filter.EndedBefore != nil {
			query = query.Where("work_end <= ?", *filter.EndedBefore)
		}

		// Success filter
		if filter.Success != nil {
			query = query.Where("result = ?", *filter.Success)
		}

		// Error filter
		if filter.HasError != nil {
			if *filter.HasError {
				query = query.Where("err IS NOT NULL AND err != ''")
			} else {
				query = query.Where("err IS NULL OR err = ''")
			}
		}

		// Session ID filter
		if filter.SessionID != nil {
			query = query.Where("completed_by_session_id = ?", *filter.SessionID)
		}

		// Apply limit and offset for pagination
		if filter.Limit > 0 {
			query = query.Limit(filter.Limit)
		}
		if filter.Offset > 0 {
			query = query.Offset(filter.Offset)
		}
	}

	// Order by posted time descending (newest first)
	query = query.Order("posted DESC")

	var history []models.TaskHistory
	if err := query.Find(&history).Error; err != nil {
		return types.TaskHistoryResponse{}, fmt.Errorf("reading task history: %w", err)
	}

	out := types.TaskHistoryResponse{
		History: make([]types.TaskHistory, len(history)),
	}
	for i, h := range history {
		out.History[i] = types.TaskHistory{
			TaskID:    h.TaskID,
			Name:      h.Name,
			CreatedAt: h.Posted,
			StartedAt: h.WorkStart,
			EndedAt:   h.WorkEnd,
			Success:   h.Result,
			Error:     h.Err,
			SessionID: h.CompletedBySessionID,
		}
	}
	return out, nil
}
