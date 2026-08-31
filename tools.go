package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type emptyInput struct{}

type headerInput struct {
	Header string `json:"header" jsonschema:"database entry header"`
}

type prefixInput struct {
	Prefix string `json:"prefix" jsonschema:"header prefix to search for"`
}

type dateRangeInput struct {
	From string `json:"from,omitempty" jsonschema:"inclusive start date in YYYY/MM/DD format"`
	To   string `json:"to,omitempty" jsonschema:"exclusive end date in YYYY/MM/DD format"`
}

type DatabaseSummary struct {
	Entries []string `json:"entries"`
	Count   int      `json:"count"`
}

type LogSummary struct {
	Count     int       `json:"count"`
	FirstDate string    `json:"first_date,omitempty"`
	LastDate  string    `json:"last_date,omitempty"`
	Totals    []Element `json:"totals"`
}

type LogEntriesResult struct {
	Entries []LogEntry `json:"entries"`
}

func registerTools(server *mcp.Server, config *config) {
	mcp.AddTool(server, &mcp.Tool{Name: "database_summary", Description: "List available database report headers."}, func(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, DatabaseSummary, error) {
		database, err := loadDatabase(config.databasePath)
		if err != nil {
			return nil, DatabaseSummary{}, err
		}
		entries := make([]string, 0, len(database))
		for _, entry := range database {
			entries = append(entries, entry.Header)
		}
		return nil, DatabaseSummary{Entries: entries, Count: len(entries)}, nil
	})

	mcp.AddTool(server, &mcp.Tool{Name: "database_search", Description: "Find database headers that start with a prefix."}, func(_ context.Context, _ *mcp.CallToolRequest, input prefixInput) (*mcp.CallToolResult, []string, error) {
		database, err := loadDatabase(config.databasePath)
		if err != nil {
			return nil, nil, err
		}
		return nil, searchDatabase(database, input.Prefix), nil
	})

	mcp.AddTool(server, &mcp.Tool{Name: "database_entry_raw", Description: "Read one raw database report entry by header."}, func(_ context.Context, _ *mcp.CallToolRequest, input headerInput) (*mcp.CallToolResult, DatabaseEntry, error) {
		database, err := loadDatabase(config.databasePath)
		if err != nil {
			return nil, DatabaseEntry{}, err
		}
		for _, entry := range database {
			if entry.Header == input.Header {
				return nil, entry, nil
			}
		}
		return nil, DatabaseEntry{}, fmt.Errorf("database header %q not found", input.Header)
	})

	mcp.AddTool(server, &mcp.Tool{Name: "database_entry_resolved", Description: "Read one database report entry with nested references resolved."}, func(_ context.Context, _ *mcp.CallToolRequest, input headerInput) (*mcp.CallToolResult, DatabaseEntry, error) {
		database, err := loadDatabase(config.databasePath)
		if err != nil {
			return nil, DatabaseEntry{}, err
		}
		database, err = resolveDatabase(database)
		if err != nil {
			return nil, DatabaseEntry{}, err
		}
		for _, entry := range database {
			if entry.Header == input.Header {
				return nil, entry, nil
			}
		}
		return nil, DatabaseEntry{}, fmt.Errorf("database header %q not found", input.Header)
	})

	mcp.AddTool(server, &mcp.Tool{Name: "log_entries", Description: "Read dated logfile entries in a half-open date range [from, to)."}, func(_ context.Context, _ *mcp.CallToolRequest, input dateRangeInput) (*mcp.CallToolResult, []LogEntry, error) {
		entries, err := filteredLogs(config, input)
		return nil, entries, err
	})

	mcp.AddTool(server, &mcp.Tool{Name: "log_entries_raw", Description: "Read raw dated logfile entries in a half-open date range [from, to)."}, func(_ context.Context, _ *mcp.CallToolRequest, input dateRangeInput) (*mcp.CallToolResult, LogEntriesResult, error) {
		entries, err := filteredRawLogs(config, input)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
				IsError: true,
			}, LogEntriesResult{Entries: []LogEntry{}}, nil
		}
		return nil, LogEntriesResult{Entries: entries}, nil
	})

	mcp.AddTool(server, &mcp.Tool{Name: "log_summary", Description: "Aggregate logfile element totals over a half-open date range [from, to)."}, func(_ context.Context, _ *mcp.CallToolRequest, input dateRangeInput) (*mcp.CallToolResult, LogSummary, error) {
		result, err := summarizeLogs(config, input)
		if err != nil {
			return nil, LogSummary{}, err
		}
		return nil, result, nil
	})
}

func searchDatabase(database []DatabaseEntry, prefix string) []string {
	matches := make([]string, 0)
	for _, entry := range database {
		if strings.HasPrefix(entry.Header, prefix) {
			matches = append(matches, entry.Header)
		}
	}
	return matches
}

func filteredLogs(config *config, input dateRangeInput) ([]LogEntry, error) {
	if config.logfilePath == "" {
		return nil, fmt.Errorf("HR_LOGFILE is not configured")
	}
	from, to, err := dateRange(input)
	if err != nil {
		return nil, err
	}
	database, err := loadDatabase(config.databasePath)
	if err != nil {
		return nil, err
	}
	database, err = resolveDatabase(database)
	if err != nil {
		return nil, err
	}
	result := make([]LogEntry, 0)
	err = loadResolvedLogfile(config.logfilePath, database, func(entry LogEntry) error {
		date, _ := time.Parse(dateFormat, entry.Date)
		if inDateRange(date, from, to) {
			result = append(result, entry)
		}
		return nil
	})
	return result, err
}

func filteredRawLogs(config *config, input dateRangeInput) ([]LogEntry, error) {
	if config.logfilePath == "" {
		return nil, fmt.Errorf("HR_LOGFILE is not configured")
	}
	from, to, err := dateRange(input)
	if err != nil {
		return nil, err
	}
	result := make([]LogEntry, 0)
	err = loadLogfile(config.logfilePath, func(entry LogEntry) error {
		date, _ := time.Parse(dateFormat, entry.Date)
		if inDateRange(date, from, to) {
			result = append(result, entry)
		}
		return nil
	})
	return result, err
}

func summarizeLogs(config *config, input dateRangeInput) (LogSummary, error) {
	if config.logfilePath == "" {
		return LogSummary{}, fmt.Errorf("HR_LOGFILE is not configured")
	}
	from, to, err := dateRange(input)
	if err != nil {
		return LogSummary{}, err
	}
	database, err := loadDatabase(config.databasePath)
	if err != nil {
		return LogSummary{}, err
	}
	database, err = resolveDatabase(database)
	if err != nil {
		return LogSummary{}, err
	}
	result := LogSummary{Totals: make([]Element, 0)}
	totals := make(map[string]float64)
	err = loadResolvedLogfile(config.logfilePath, database, func(entry LogEntry) error {
		date, _ := time.Parse(dateFormat, entry.Date)
		if !inDateRange(date, from, to) {
			return nil
		}
		if result.Count == 0 || entry.Date < result.FirstDate {
			result.FirstDate = entry.Date
		}
		if result.Count == 0 || entry.Date > result.LastDate {
			result.LastDate = entry.Date
		}
		result.Count++
		for _, value := range entry.Elements {
			totals[value.Name] += value.Value
		}
		return nil
	})
	if err != nil {
		return LogSummary{}, err
	}
	for name, value := range totals {
		result.Totals = append(result.Totals, Element{Name: name, Value: value})
	}
	sort.Slice(result.Totals, func(i, j int) bool { return result.Totals[i].Name < result.Totals[j].Name })
	return result, nil
}

func dateRange(input dateRangeInput) (time.Time, time.Time, error) {
	var from, to time.Time
	var err error
	if input.From != "" {
		from, err = time.Parse(dateFormat, input.From)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid from date %q: %w", input.From, err)
		}
	}
	if input.To != "" {
		to, err = time.Parse(dateFormat, input.To)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid to date %q: %w", input.To, err)
		}
	}
	if !from.IsZero() && !to.IsZero() && !from.Before(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("from date must be before to date")
	}
	return from, to, nil
}

func inDateRange(date, from, to time.Time) bool {
	return (from.IsZero() || !date.Before(from)) && (to.IsZero() || date.Before(to))
}
