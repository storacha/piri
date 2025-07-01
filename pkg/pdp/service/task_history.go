package service

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPService) GetTaskHistory(ctx context.Context, filter *types.TaskHistoryFilter) (types.TaskHistoryResponse, error) {
	baseQuery := p.db.WithContext(ctx).Model(&models.TaskHistory{})

	// Apply filters to base query
	if filter != nil {
		// Task ID filter
		if filter.TaskID != nil {
			baseQuery = baseQuery.Where("task_id = ?", *filter.TaskID)
		}

		// Name filter (partial match, case-insensitive)
		if filter.Name != nil {
			baseQuery = baseQuery.Where("LOWER(name) LIKE LOWER(?)", "%"+*filter.Name+"%")
		}

		// Time range filters
		if filter.CreatedAfter != nil {
			baseQuery = baseQuery.Where("posted >= ?", *filter.CreatedAfter)
		}
		if filter.CreatedBefore != nil {
			baseQuery = baseQuery.Where("posted <= ?", *filter.CreatedBefore)
		}
		if filter.StartedAfter != nil {
			baseQuery = baseQuery.Where("work_start >= ?", *filter.StartedAfter)
		}
		if filter.StartedBefore != nil {
			baseQuery = baseQuery.Where("work_start <= ?", *filter.StartedBefore)
		}
		if filter.EndedAfter != nil {
			baseQuery = baseQuery.Where("work_end >= ?", *filter.EndedAfter)
		}
		if filter.EndedBefore != nil {
			baseQuery = baseQuery.Where("work_end <= ?", *filter.EndedBefore)
		}

		// Success filter
		if filter.Success != nil {
			baseQuery = baseQuery.Where("result = ?", *filter.Success)
		}

		// Error filter
		if filter.HasError != nil {
			if *filter.HasError {
				baseQuery = baseQuery.Where("err IS NOT NULL AND err != ''")
			} else {
				baseQuery = baseQuery.Where("err IS NULL OR err = ''")
			}
		}

		// Session ID filter
		if filter.SessionID != nil {
			baseQuery = baseQuery.Where("completed_by_session_id = ?", *filter.SessionID)
		}
	}

	// Get total count of matching records
	var totalCount int64
	if err := baseQuery.Count(&totalCount).Error; err != nil {
		return types.TaskHistoryResponse{}, fmt.Errorf("counting task history: %w", err)
	}

	// Create query for fetching data with pagination
	dataQuery := baseQuery.Session(&gorm.Session{})
	
	// Default values for pagination
	limit := 0
	offset := 0
	
	if filter != nil {
		limit = filter.Limit
		offset = filter.Offset
		
		// Apply limit and offset for pagination
		if filter.Limit > 0 {
			dataQuery = dataQuery.Limit(filter.Limit)
		}
		if filter.Offset > 0 {
			dataQuery = dataQuery.Offset(filter.Offset)
		}
	}

	// Order by posted time descending (newest first)
	dataQuery = dataQuery.Order("posted DESC")

	var history []models.TaskHistory
	if err := dataQuery.Find(&history).Error; err != nil {
		return types.TaskHistoryResponse{}, fmt.Errorf("reading task history: %w", err)
	}

	// Calculate pagination metadata
	hasMore := false
	if limit > 0 {
		hasMore = int64(offset+len(history)) < totalCount
	}
	
	// Calculate page and total pages if limit is set
	page := 0
	totalPages := 0
	if limit > 0 {
		page = (offset / limit) + 1
		totalPages = int(totalCount) / limit
		if int(totalCount)%limit > 0 {
			totalPages++
		}
	}

	out := types.TaskHistoryResponse{
		History:    make([]types.TaskHistory, len(history)),
		TotalCount: int(totalCount),
		HasMore:    hasMore,
		Limit:      limit,
		Offset:     offset,
		Page:       page,
		TotalPages: totalPages,
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
