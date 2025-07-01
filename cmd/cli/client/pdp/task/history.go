package task

import (
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/storacha/piri/cmd/cliutil/format"
	"github.com/storacha/piri/pkg/client"
	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp/types"
)

var (
	// Filter flags
	taskID        int64
	taskName      string
	createdAfter  string
	createdBefore string
	startedAfter  string
	startedBefore string
	endedAfter    string
	endedBefore   string
	successFilter string // "true", "false", or "" for all
	hasError      string // "true", "false", or "" for all  
	sessionID     string
	limit         int
	offset        int
	page          int

	// Output format flag
	outputFormat string

	HistoryCmd = &cobra.Command{
		Use:   "history",
		Short: "Get history of PDP tasks with optional filters",
		Long: `Retrieve PDP task history with various filtering options:
  - Filter by task ID, name, session ID
  - Filter by time ranges (created, started, ended)
  - Filter by success/failure status or tasks with errors
  - Support pagination with limit, offset, or page number
  - Default limit is 10 results per page`,
		Args: cobra.NoArgs,
		RunE: doHistory,
	}
)

func init() {
	// Task filters
	HistoryCmd.Flags().Int64Var(&taskID, "task-id", 0, "Filter by specific task ID")
	HistoryCmd.Flags().StringVar(&taskName, "name", "", "Filter by task name (partial match)")
	HistoryCmd.Flags().StringVar(&sessionID, "session-id", "", "Filter by session ID")

	// Time filters  
	HistoryCmd.Flags().StringVar(&createdAfter, "created-after", "", "Filter tasks created after this time (RFC3339 format, e.g., 2006-01-02T15:04:05Z)")
	HistoryCmd.Flags().StringVar(&createdBefore, "created-before", "", "Filter tasks created before this time (RFC3339 format)")
	HistoryCmd.Flags().StringVar(&startedAfter, "started-after", "", "Filter tasks started after this time (RFC3339 format)")
	HistoryCmd.Flags().StringVar(&startedBefore, "started-before", "", "Filter tasks started before this time (RFC3339 format)")
	HistoryCmd.Flags().StringVar(&endedAfter, "ended-after", "", "Filter tasks ended after this time (RFC3339 format)")
	HistoryCmd.Flags().StringVar(&endedBefore, "ended-before", "", "Filter tasks ended before this time (RFC3339 format)")

	// Status filters
	HistoryCmd.Flags().StringVar(&successFilter, "success", "", "Filter by success status (true/false)")
	HistoryCmd.Flags().StringVar(&hasError, "has-error", "", "Filter tasks with errors (true/false)")
	cobra.CheckErr(HistoryCmd.Flags().MarkHidden("has-error"))

	// Pagination
	HistoryCmd.Flags().IntVar(&limit, "limit", 10, "Limit number of results")
	HistoryCmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination")
	HistoryCmd.Flags().IntVar(&page, "page", 0, "Page number (1-based, alternative to offset)")

	// Output format
	HistoryCmd.Flags().StringVar(&outputFormat, "format", "table", "Output format: table or json")
}

func doHistory(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load[config.PDPClient]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	nodeURL, err := url.Parse(cfg.NodeURL)
	if err != nil {
		return fmt.Errorf("parsing node URL: %w", err)
	}

	// Build filter from flags
	filter, err := buildFilterFromFlags()
	if err != nil {
		return fmt.Errorf("building filter: %w", err)
	}

	// Validate filter
	if err := filter.Validate(); err != nil {
		return fmt.Errorf("invalid filter: %w", err)
	}

	piriClient := client.NewPiriClient(nodeURL)

	history, err := piriClient.TaskHistory(ctx, filter)
	if err != nil {
		return fmt.Errorf("getting task history: %w", err)
	}

	// Parse output format
	formatStr := outputFormat
	outFormat, err := format.ParseOutputFormat(formatStr)
	if err != nil {
		return fmt.Errorf("invalid output format: %w", err)
	}

	// Create formatter and format output
	formatter := format.NewFormatter(outFormat, cmd.OutOrStdout())
	if err := formatter.Format(history); err != nil {
		return fmt.Errorf("formatting output: %w", err)
	}

	// If table format, print pagination info after the table
	if outFormat == format.TableFormat && history.TotalCount > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\n")
		
		// Calculate current range
		start := history.Offset + 1
		end := history.Offset + len(history.History)
		
		// Print pagination summary
		if history.Page > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Page %d of %d (showing %d-%d of %d results)", 
				history.Page, history.TotalPages, start, end, history.TotalCount)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Showing %d-%d of %d results", 
				start, end, history.TotalCount)
		}
		
		// Show hint for next page if there are more results
		if history.HasMore {
			if page > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), " - Use --page %d for next page", page+1)
			} else {
				nextOffset := history.Offset + history.Limit
				fmt.Fprintf(cmd.OutOrStdout(), " - Use --offset %d for next page", nextOffset)
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\n")
	}

	return nil
}

func buildFilterFromFlags() (*types.TaskHistoryFilter, error) {
	filter := &types.TaskHistoryFilter{}

	// Task ID filter
	if taskID > 0 {
		filter.TaskID = &taskID
	}

	// Name filter
	if taskName != "" {
		filter.Name = &taskName
	}

	// Session ID filter
	if sessionID != "" {
		filter.SessionID = &sessionID
	}

	// Time filters
	if createdAfter != "" {
		t, err := time.Parse(time.RFC3339, createdAfter)
		if err != nil {
			return nil, fmt.Errorf("invalid created-after time format: %w", err)
		}
		filter.CreatedAfter = &t
	}

	if createdBefore != "" {
		t, err := time.Parse(time.RFC3339, createdBefore)
		if err != nil {
			return nil, fmt.Errorf("invalid created-before time format: %w", err)
		}
		filter.CreatedBefore = &t
	}

	if startedAfter != "" {
		t, err := time.Parse(time.RFC3339, startedAfter)
		if err != nil {
			return nil, fmt.Errorf("invalid started-after time format: %w", err)
		}
		filter.StartedAfter = &t
	}

	if startedBefore != "" {
		t, err := time.Parse(time.RFC3339, startedBefore)
		if err != nil {
			return nil, fmt.Errorf("invalid started-before time format: %w", err)
		}
		filter.StartedBefore = &t
	}

	if endedAfter != "" {
		t, err := time.Parse(time.RFC3339, endedAfter)
		if err != nil {
			return nil, fmt.Errorf("invalid ended-after time format: %w", err)
		}
		filter.EndedAfter = &t
	}

	if endedBefore != "" {
		t, err := time.Parse(time.RFC3339, endedBefore)
		if err != nil {
			return nil, fmt.Errorf("invalid ended-before time format: %w", err)
		}
		filter.EndedBefore = &t
	}

	// Success filter
	if successFilter != "" {
		b, err := parseBool(successFilter)
		if err != nil {
			return nil, fmt.Errorf("invalid success filter value: %w", err)
		}
		filter.Success = &b
	}

	// Has error filter
	if hasError != "" {
		b, err := parseBool(hasError)
		if err != nil {
			return nil, fmt.Errorf("invalid has-error filter value: %w", err)
		}
		filter.HasError = &b
	}

	// Pagination
	filter.Limit = limit
	
	// Validate that both page and offset are not used together
	if page > 0 && offset > 0 {
		return nil, fmt.Errorf("cannot use both --page and --offset flags together")
	}
	
	// If page is specified, calculate offset from it
	if page > 0 {
		if limit <= 0 {
			return nil, fmt.Errorf("page flag requires a positive limit")
		}
		filter.Offset = (page - 1) * limit
	} else {
		filter.Offset = offset
	}

	return filter, nil
}

func parseBool(s string) (bool, error) {
	switch s {
	case "true", "1", "yes", "y":
		return true, nil
	case "false", "0", "no", "n":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", s)
	}
}
