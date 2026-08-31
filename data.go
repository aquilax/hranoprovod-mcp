package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/aquilax/hranoprovod-cli/v3/pkg/element"
	"github.com/aquilax/hranoprovod-cli/v3/pkg/node"
	"github.com/aquilax/hranoprovod-cli/v3/pkg/parser"
	"github.com/aquilax/hranoprovod-cli/v3/pkg/resolver"
)

const dateFormat = parser.DefaultDateFormat

type Element struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type Metadata struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type DatabaseEntry struct {
	Header   string     `json:"header"`
	Elements []Element  `json:"elements"`
	Metadata []Metadata `json:"metadata,omitempty"`
}

type LogEntry struct {
	Date     string     `json:"date"`
	Elements []Element  `json:"elements"`
	Metadata []Metadata `json:"metadata,omitempty"`
}

func loadDatabase(path string) ([]DatabaseEntry, error) {
	entries := make([]DatabaseEntry, 0)
	indexes := make(map[string]int)
	err := parser.ParseFileCallback(path, parser.NewDefaultConfig(), func(parsed *node.ParserNode, parseErr error) (bool, error) {
		if parseErr != nil {
			return true, fmt.Errorf("%s: %w", path, parseErr)
		}
		entry := databaseEntry(parsed)
		if index, exists := indexes[entry.Header]; exists {
			entries[index] = entry
		} else {
			indexes[entry.Header] = len(entries)
			entries = append(entries, entry)
		}
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("load database: %w", err)
	}
	return entries, nil
}

func loadLogfile(path string, callback func(LogEntry) error) error {
	return walkLogfile(path, func(parsed *node.ParserNode) ([]Element, error) {
		return elements(parsed.Elements), nil
	}, callback)
}

func loadResolvedLogfile(path string, database []DatabaseEntry, callback func(LogEntry) error) error {
	databaseByHeader := make(map[string]DatabaseEntry, len(database))
	for _, entry := range database {
		databaseByHeader[entry.Header] = entry
	}
	return walkLogfile(path, func(parsed *node.ParserNode) ([]Element, error) {
		return resolveLogElements(parsed.Elements, databaseByHeader), nil
	}, callback)
}

func walkLogfile(path string, elementsForNode func(*node.ParserNode) ([]Element, error), callback func(LogEntry) error) error {
	err := parser.ParseFileCallback(path, parser.NewDefaultConfig(), func(parsed *node.ParserNode, parseErr error) (bool, error) {
		if parseErr != nil {
			return true, fmt.Errorf("%s: %w", path, parseErr)
		}
		date, err := time.Parse(dateFormat, parsed.Header)
		if err != nil {
			return true, fmt.Errorf("%s: invalid date %q: %w", path, parsed.Header, err)
		}
		nodeElements, err := elementsForNode(parsed)
		if err != nil {
			return true, err
		}
		if err := callback(LogEntry{Date: date.Format(dateFormat), Elements: nodeElements, Metadata: metadata(parsed.Metadata)}); err != nil {
			return true, err
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("load logfile: %w", err)
	}
	return nil
}

func resolveDatabase(database []DatabaseEntry) ([]DatabaseEntry, error) {
	db := node.NewDBNodeMap()
	for _, entry := range database {
		nodeEntry := node.NewDBNodeFromNode(&node.ParserNode{Header: entry.Header, Elements: toElements(entry.Elements), Metadata: toNodeMetadata(entry.Metadata)})
		db.Push(nodeEntry)
	}
	resolved, err := resolver.Resolve(resolver.NewDefaultConfig(), db)
	if err != nil {
		return nil, fmt.Errorf("resolve database: %w", err)
	}
	result := make([]DatabaseEntry, 0, len(database))
	for _, entry := range database {
		resolvedEntry := resolved[entry.Header]
		result = append(result, databaseEntry(&node.ParserNode{Header: resolvedEntry.Header, Elements: resolvedEntry.Elements, Metadata: resolvedEntry.Metadata}))
	}
	return result, nil
}

func resolveLogElements(values element.Elements, database map[string]DatabaseEntry) []Element {
	totals := make(map[string]float64)
	for _, value := range values {
		if entry, found := database[value.Name]; found {
			for _, resolved := range entry.Elements {
				totals[resolved.Name] += value.Value * resolved.Value
			}
		} else {
			totals[value.Name] += value.Value
		}
	}
	result := make([]Element, 0, len(totals))
	for name, value := range totals {
		result = append(result, Element{Name: name, Value: value})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func toElements(values []Element) element.Elements {
	result := element.NewElements()
	for _, value := range values {
		result.Add(value.Name, value.Value)
	}
	return result
}

func toNodeMetadata(values []Metadata) *node.Metadata {
	if values == nil {
		return nil
	}
	result := node.Metadata{}
	for _, value := range values {
		result = append(result, node.MetadataPair{Name: value.Name, Value: value.Value})
	}
	return &result
}

func databaseEntry(parsed *node.ParserNode) DatabaseEntry {
	return DatabaseEntry{Header: parsed.Header, Elements: elements(parsed.Elements), Metadata: metadata(parsed.Metadata)}
}

func elements(values element.Elements) []Element {
	result := make([]Element, 0, len(values))
	for _, value := range values {
		result = append(result, Element{Name: value.Name, Value: value.Value})
	}
	return result
}

func metadata(values *node.Metadata) []Metadata {
	if values == nil {
		return nil
	}
	result := make([]Metadata, 0, len(*values))
	for _, value := range *values {
		result = append(result, Metadata{Name: value.Name, Value: value.Value})
	}
	return result
}
