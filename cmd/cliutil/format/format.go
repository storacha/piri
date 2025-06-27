package format

import (
	"encoding/json"
	"fmt"
	"io"
)

// OutputFormat represents the format for CLI output
type OutputFormat string

const (
	TableFormat OutputFormat = "table"
	JSONFormat  OutputFormat = "json"
)

// ParseOutputFormat parses a string into an OutputFormat
func ParseOutputFormat(s string) (OutputFormat, error) {
	switch s {
	case "table", "":
		return TableFormat, nil
	case "json":
		return JSONFormat, nil
	default:
		return "", fmt.Errorf("unknown output format: %s (valid formats: table or json)", s)
	}
}

// Formatter is an interface for formatting output
type Formatter interface {
	Format(data interface{}) error
}

// NewFormatter creates a formatter based on the output format
func NewFormatter(format OutputFormat, writer io.Writer) Formatter {
	switch format {
	case JSONFormat:
		return &JSONFormatter{writer: writer}
	case TableFormat:
		return &TableFormatter{writer: writer}
	default:
		return &TableFormatter{writer: writer}
	}
}

// JSONFormatter formats output as JSON
type JSONFormatter struct {
	writer io.Writer
}

// Format implements the Formatter interface for JSON
func (f *JSONFormatter) Format(data interface{}) error {
	encoder := json.NewEncoder(f.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
