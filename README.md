# hranoprovod-mcp

Read-only MCP server for [hranoprov-cli](https://github.com/aquilax/hranoprovod-cli). It communicates over stdio and reads its configured files for each request.

## Configuration

`HR_DATABASE` is required and must point to a hranoprov database file. `HR_LOGFILE` is optional and must point to a logfile whose top-level headers are dates in `YYYY/MM/DD` format. Paths are supplied as-is; tools cannot read arbitrary files.

```sh
HR_DATABASE=/path/database.hr HR_LOGFILE=/path/log.hr go run .
```

The server exits if `HR_DATABASE` is missing. File open and parse errors are returned by the affected tool. Without `HR_LOGFILE`, database tools remain available and logfile tools return an unavailable error.

## Tools

- `database_summary` lists database headers and their count.
- `database_search` returns database headers beginning with the requested `prefix`, for example `soups/` matches `soups/chicken`.
- `database_entry_raw` reads an unresolved database entry by its `header`.
- `database_entry_resolved` reads a database entry by its `header` after nested references are resolved.
- `log_entries` reads logfile entries in the half-open `[from, to)` date interval.
- `log_entries_raw` reads logfile entries without resolving database references.
- `log_summary` returns the filtered entry count, date range, and summed element totals using `[from, to)`.

All tools are read-only. Database duplicate headers use the last entry, matching the CLI data model. Duplicate elements are preserved in raw entries and summed by `log_summary`.

Before logfile tools process a node, the database is resolved using the CLI resolver. Nested recipe references are expanded with quantity multiplication; elements missing from the database remain unchanged. Date intervals use an inclusive `from` and exclusive `to` boundary, so adjacent intervals do not overlap.

The database and logfile format uses a top-level header followed by indented `name value` lines. Lines beginning with `#` are comments; metadata can be written as indented `# name: value` lines.