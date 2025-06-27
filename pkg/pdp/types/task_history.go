package types

import (
	"fmt"
	"net/url"
	"strconv"
	"time"
)

type TaskHistoryResponse struct {
	History []TaskHistory `json:"history"`
}

type TaskHistory struct {
	TaskID    int64     `json:"task_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	SessionID string    `json:"session_id"`
}

// TaskHistoryFilter represents all possible filters for task history queries
type TaskHistoryFilter struct {
	// Filter by specific task ID
	TaskID *int64 `json:"task_id,omitempty"`

	// Filter by task name (supports partial matching)
	Name *string `json:"name,omitempty"`

	// Time range filters
	CreatedAfter  *time.Time `json:"created_after,omitempty"`
	CreatedBefore *time.Time `json:"created_before,omitempty"`
	StartedAfter  *time.Time `json:"started_after,omitempty"`
	StartedBefore *time.Time `json:"started_before,omitempty"`
	EndedAfter    *time.Time `json:"ended_after,omitempty"`
	EndedBefore   *time.Time `json:"ended_before,omitempty"`

	// Filter by success status (true = successful, false = failed, nil = all)
	Success *bool `json:"success,omitempty"`

	// Filter to only show tasks with errors
	HasError *bool `json:"has_error,omitempty"`

	// Filter by session ID
	SessionID *string `json:"session_id,omitempty"`

	// Pagination support (for future use)
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// ToQueryParams converts the filter to URL query parameters
func (f *TaskHistoryFilter) ToQueryParams() url.Values {
	params := url.Values{}

	if f.TaskID != nil {
		params.Set("task_id", strconv.FormatInt(*f.TaskID, 10))
	}
	if f.Name != nil {
		params.Set("name", *f.Name)
	}
	if f.CreatedAfter != nil {
		params.Set("created_after", f.CreatedAfter.Format(time.RFC3339))
	}
	if f.CreatedBefore != nil {
		params.Set("created_before", f.CreatedBefore.Format(time.RFC3339))
	}
	if f.StartedAfter != nil {
		params.Set("started_after", f.StartedAfter.Format(time.RFC3339))
	}
	if f.StartedBefore != nil {
		params.Set("started_before", f.StartedBefore.Format(time.RFC3339))
	}
	if f.EndedAfter != nil {
		params.Set("ended_after", f.EndedAfter.Format(time.RFC3339))
	}
	if f.EndedBefore != nil {
		params.Set("ended_before", f.EndedBefore.Format(time.RFC3339))
	}
	if f.Success != nil {
		params.Set("success", strconv.FormatBool(*f.Success))
	}
	if f.HasError != nil {
		params.Set("has_error", strconv.FormatBool(*f.HasError))
	}
	if f.SessionID != nil {
		params.Set("session_id", *f.SessionID)
	}
	if f.Limit > 0 {
		params.Set("limit", strconv.Itoa(f.Limit))
	}
	if f.Offset > 0 {
		params.Set("offset", strconv.Itoa(f.Offset))
	}

	return params
}

// NewTaskHistoryFilterFromQuery creates a filter from URL query parameters
func NewTaskHistoryFilterFromQuery(values url.Values) (*TaskHistoryFilter, error) {
	filter := &TaskHistoryFilter{}

	if v := values.Get("task_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, err
		}
		filter.TaskID = &id
	}

	if v := values.Get("name"); v != "" {
		filter.Name = &v
	}

	if v := values.Get("created_after"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, err
		}
		filter.CreatedAfter = &t
	}

	if v := values.Get("created_before"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, err
		}
		filter.CreatedBefore = &t
	}

	if v := values.Get("started_after"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, err
		}
		filter.StartedAfter = &t
	}

	if v := values.Get("started_before"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, err
		}
		filter.StartedBefore = &t
	}

	if v := values.Get("ended_after"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, err
		}
		filter.EndedAfter = &t
	}

	if v := values.Get("ended_before"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, err
		}
		filter.EndedBefore = &t
	}

	if v := values.Get("success"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, err
		}
		filter.Success = &b
	}

	if v := values.Get("has_error"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, err
		}
		filter.HasError = &b
	}

	if v := values.Get("session_id"); v != "" {
		filter.SessionID = &v
	}

	if v := values.Get("limit"); v != "" {
		limit, err := strconv.Atoi(v)
		if err != nil {
			return nil, err
		}
		filter.Limit = limit
	}

	if v := values.Get("offset"); v != "" {
		offset, err := strconv.Atoi(v)
		if err != nil {
			return nil, err
		}
		filter.Offset = offset
	}

	return filter, nil
}

// Validate checks if the filter parameters are valid
func (f *TaskHistoryFilter) Validate() error {
	if f == nil {
		return nil
	}

	// Validate time ranges
	if f.CreatedAfter != nil && f.CreatedBefore != nil {
		if f.CreatedAfter.After(*f.CreatedBefore) {
			return fmt.Errorf("created_after must be before created_before")
		}
	}

	if f.StartedAfter != nil && f.StartedBefore != nil {
		if f.StartedAfter.After(*f.StartedBefore) {
			return fmt.Errorf("started_after must be before started_before")
		}
	}

	if f.EndedAfter != nil && f.EndedBefore != nil {
		if f.EndedAfter.After(*f.EndedBefore) {
			return fmt.Errorf("ended_after must be before ended_before")
		}
	}

	// Validate pagination
	if f.Limit < 0 {
		return fmt.Errorf("limit must be non-negative")
	}

	if f.Offset < 0 {
		return fmt.Errorf("offset must be non-negative")
	}

	// Optional: Add a max limit to prevent excessive resource usage
	if f.Limit > 1000 {
		return fmt.Errorf("limit cannot exceed 1000")
	}

	return nil
}
