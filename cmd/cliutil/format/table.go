package format

import (
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/storacha/piri/pkg/pdp/types"
)

// TableFormatter formats output as a table
type TableFormatter struct {
	writer io.Writer
}

// Format implements the Formatter interface for tables
func (f *TableFormatter) Format(data interface{}) error {
	switch v := data.(type) {
	case *types.TaskHistoryResponse:
		return f.formatTaskHistory(v)
	default:
		return fmt.Errorf("table format not supported for type %T", data)
	}
}

// formatTaskHistory formats task history as a table
func (f *TableFormatter) formatTaskHistory(history *types.TaskHistoryResponse) error {
	if len(history.History) == 0 {
		noTasksStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
		_, err := fmt.Fprintln(f.writer, noTasksStyle.Render("No tasks found"))
		return err
	}

	// Build rows for the table
	var rows []table.Row
	for _, task := range history.History {
		// Status with color
		status := "SUCCESS"
		if !task.Success {
			status = "FAILED"
		}

		rows = append(rows, table.Row{
			fmt.Sprintf("%d", task.TaskID),
			task.Name,
			status,
			task.CreatedAt.Format(time.DateTime),
			task.StartedAt.Format(time.DateTime),
			task.EndedAt.Format(time.DateTime),
			task.EndedAt.Sub(task.StartedAt).String(),
			task.SessionID,
			task.Error,
		})
	}

	// Create table columns with very large widths to prevent truncation
	columns := []table.Column{
		{Title: "TASK ID", Width: 8},
		{Title: "NAME", Width: 16},
		{Title: "STATUS", Width: 10},
		{Title: "CREATED", Width: 25},
		{Title: "STARTED", Width: 25},
		{Title: "ENDED", Width: 25},
		{Title: "DURATION", Width: 15},
		{Title: "SESSION", Width: 10}, // Full UUIDs are ~36 chars, but we want no truncation
		{Title: "ERROR", Width: 32},   // Very wide for full error messages
	}

	// Create the table
	tableHeight := len(rows)
	if tableHeight == 0 {
		tableHeight = 1
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(false),
		table.WithHeight(tableHeight),
		table.WithWidth(256),
	)

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)
	t.SetStyles(s)

	// Otherwise, render the static table
	tableView := t.View()
	if tableView == "" {
		// Fallback to simple text output if table rendering fails
		return f.fallbackTextOutput(history)
	}
	_, err := fmt.Fprintln(f.writer, tableView)
	return err
}

// fallbackTextOutput provides a simple text output when table rendering fails
func (f *TableFormatter) fallbackTextOutput(history *types.TaskHistoryResponse) error {
	for i, task := range history.History {
		if i > 0 {
			_, _ = fmt.Fprintln(f.writer, "---")
		}
		_, _ = fmt.Fprintf(f.writer, "Task ID: %d\n", task.TaskID)
		_, _ = fmt.Fprintf(f.writer, "Name: %s\n", task.Name)
		_, _ = fmt.Fprintf(f.writer, "Status: %s\n", map[bool]string{true: "SUCCESS", false: "FAILED"}[task.Success])
		_, _ = fmt.Fprintf(f.writer, "Created: %s\n", task.CreatedAt.Format(time.RFC3339))
		_, _ = fmt.Fprintf(f.writer, "Started: %s\n", task.StartedAt.Format(time.RFC3339))
		_, _ = fmt.Fprintf(f.writer, "Ended: %s\n", task.EndedAt.Format(time.RFC3339))
		_, _ = fmt.Fprintf(f.writer, "Duration: %s\n", task.EndedAt.Sub(task.StartedAt))
		_, _ = fmt.Fprintf(f.writer, "Session: %s\n", task.SessionID)
		if task.Error != "" {
			_, _ = fmt.Fprintf(f.writer, "Error: %s\n", task.Error)
		}
	}
	return nil
}
