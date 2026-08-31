package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSnapshot(t *testing.T) {
	dir := t.TempDir()
	database := filepath.Join(dir, "database.hr")
	logfile := filepath.Join(dir, "log.hr")
	writeTestFile(t, database, "breakfast\n  oats 10\n  milk 2\n\nbreakfast\n  oats 12\n")
	writeTestFile(t, logfile, "2024/01/02\n  oats 3\n\n2024/01/01\n  oats 2\n  milk 1\n")

	config, err := newConfig(database, logfile)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := loadDatabase(config.databasePath)
	if err != nil || len(entries) != 1 || entries[0].Elements[0].Value != 12 {
		t.Fatalf("unexpected database entries: %#v, %v", entries, err)
	}
	logs := make([]LogEntry, 0)
	databaseEntries, err := loadDatabase(config.databasePath)
	if err != nil {
		t.Fatal(err)
	}
	err = loadResolvedLogfile(config.logfilePath, databaseEntries, func(entry LogEntry) error {
		logs = append(logs, entry)
		return nil
	})
	if err != nil || len(logs) != 2 || logs[0].Date != "2024/01/02" {
		t.Fatalf("unexpected walked logs: %#v, %v", logs, err)
	}
	filtered, err := filteredLogs(config, dateRangeInput{From: "2024/01/01", To: "2024/01/02"})
	if err != nil || len(filtered) != 1 || len(filtered[0].Elements) != 2 || filtered[0].Elements[1] != (Element{Name: "oats", Value: 2}) {
		t.Fatalf("unexpected filtered entries: %#v, %v", filtered, err)
	}
	summary, err := summarizeLogs(config, dateRangeInput{})
	if err != nil || summary.Count != 2 || summary.FirstDate != "2024/01/01" || summary.LastDate != "2024/01/02" || len(summary.Totals) != 2 || summary.Totals[0].Name != "milk" || summary.Totals[0].Value != 1 || summary.Totals[1].Name != "oats" || summary.Totals[1].Value != 5 {
		t.Fatalf("unexpected summary: %#v, %v", summary, err)
	}
}

func TestDateRangeUsesExclusiveEnd(t *testing.T) {
	from, to, err := dateRange(dateRangeInput{From: "2024/01/01", To: "2024/01/03"})
	if err != nil || !inDateRange(from, from, to) || !inDateRange(to.AddDate(0, 0, -1), from, to) || inDateRange(to, from, to) {
		t.Fatalf("unexpected half-open interval: %v, %v, %v", from, to, err)
	}
	if _, _, err := dateRange(dateRangeInput{From: "2024/01/02", To: "2024/01/02"}); err == nil {
		t.Fatal("expected empty interval to be rejected")
	}
}

func TestSearchDatabaseByPrefix(t *testing.T) {
	database := []DatabaseEntry{
		{Header: "soups/chicken"},
		{Header: "bread/rye"},
		{Header: "soups/tomato"},
	}
	matches := searchDatabase(database, "soups/")
	if len(matches) != 2 || matches[0] != "soups/chicken" || matches[1] != "soups/tomato" {
		t.Fatalf("unexpected prefix matches: %#v", matches)
	}
}

func TestRawLogfileEntriesDoNotResolveDatabaseRecords(t *testing.T) {
	dir := t.TempDir()
	databasePath := filepath.Join(dir, "database.hr")
	logfilePath := filepath.Join(dir, "log.hr")
	writeTestFile(t, databasePath, "meal\n  calories 100\n")
	writeTestFile(t, logfilePath, "2024/01/01\n  meal 2\n")
	var entries []LogEntry
	err := loadLogfile(logfilePath, func(entry LogEntry) error {
		entries = append(entries, entry)
		return nil
	})
	if err != nil || len(entries) != 1 || len(entries[0].Elements) != 1 || entries[0].Elements[0] != (Element{Name: "meal", Value: 2}) {
		t.Fatalf("unexpected raw entries: %#v, %v", entries, err)
	}
}

func TestLogfileResolvesDatabaseRecords(t *testing.T) {
	dir := t.TempDir()
	databasePath := filepath.Join(dir, "database.hr")
	logfilePath := filepath.Join(dir, "log.hr")
	writeTestFile(t, databasePath, "meal\n  calories 100\n  ingredient 2\n\ningredient\n  protein 3\n")
	writeTestFile(t, logfilePath, "2024/01/01\n  meal 2\n  unknown 4\n")
	database, err := loadDatabase(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	database, err = resolveDatabase(database)
	if err != nil {
		t.Fatal(err)
	}
	var entries []LogEntry
	err = loadResolvedLogfile(logfilePath, database, func(entry LogEntry) error {
		entries = append(entries, entry)
		return nil
	})
	if err != nil || len(entries) != 1 || len(entries[0].Elements) != 3 {
		t.Fatalf("unexpected resolved entries: %#v, %v", entries, err)
	}
	if entries[0].Elements[0] != (Element{Name: "calories", Value: 200}) || entries[0].Elements[1] != (Element{Name: "protein", Value: 12}) || entries[0].Elements[2] != (Element{Name: "unknown", Value: 4}) {
		t.Fatalf("unexpected resolved elements: %#v", entries[0].Elements)
	}
}

func TestLoadSnapshotRequiresDatabase(t *testing.T) {
	if _, err := newConfig("", ""); err == nil {
		t.Fatal("expected HR_DATABASE error")
	}
}

func TestOptionalLogfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.hr")
	writeTestFile(t, path, "entry\n  oats 1\n")
	config, err := newConfig(path, "")
	if err != nil || config.logfilePath != "" {
		t.Fatalf("unexpected config: %#v, %v", config, err)
	}
	if _, err := filteredLogs(config, dateRangeInput{}); err == nil {
		t.Fatal("expected unavailable logfile error")
	}
}

func TestDatabaseIsReadWhenRequested(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.hr")
	writeTestFile(t, path, "entry\n  oats 1\n")
	config, err := newConfig(path, "")
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, "entry\n  oats 2\n")
	entries, err := loadDatabase(config.databasePath)
	if err != nil || len(entries) != 1 || entries[0].Elements[0].Value != 2 {
		t.Fatalf("expected latest file contents: %#v, %v", entries, err)
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}
