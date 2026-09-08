// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import "strings"

// searchMatch is one matching line inside a file: its 1-based line number and
// a snippet of the line's text, cropped around the match when the line is
// long.
type searchMatch struct {
	Line int    `json:"line"`
	Text string `json:"text"`
}

// searchFileResult is every reported match within a single file.
type searchFileResult struct {
	Path    string        `json:"path"`
	Matches []searchMatch `json:"matches"`
}

// searchResponse is the body of GET /api/search.
type searchResponse struct {
	Query string             `json:"query"`
	Files []searchFileResult `json:"files"`
	// Truncated is true when there were more matching files, or more files to
	// scan, than this response includes — the client shows a hint rather than
	// implying the listed files are the whole story.
	Truncated bool `json:"truncated"`
}

const (
	// maxSearchResultFiles caps how many matching files a search reports —
	// past this it stops looking, since a sidebar list past a few dozen files
	// stops being something anyone reads through anyway.
	maxSearchResultFiles = 50
	// maxMatchesPerFile caps how many lines are reported per file, so one
	// file dense with the query does not crowd out every other result.
	maxMatchesPerFile = 5
	// snippetRadius is how many characters of context a long line's snippet
	// keeps on each side of the match.
	snippetRadius = 60
	// maxScanFilesLocal bounds how many local files one search reads, purely
	// as a sanity limit — disk reads are cheap, but an enormous tree
	// shouldn't turn a keystroke into an unbounded walk.
	maxScanFilesLocal = 500
	// GitHub scan limits are far lower: each unread file costs a real API
	// call, and the unauthenticated rate limit is only 60 requests/hour (see
	// rateLimitError in github.go) — a handful of searches could exhaust it
	// on a repo with even a modest number of Markdown files. A configured
	// --github-token raises that ceiling to 5,000/hour, so it also gets a
	// much higher scan budget.
	maxScanFilesGitHubAnon   = 40
	maxScanFilesGitHubAuthed = 300
)

// searchFiles runs a case-insensitive substring search for query across the
// content of paths (relative, slash-separated, as returned by listMarkdownPaths
// or flattenTreePaths), reading each file with read. It stops once scanLimit
// files have been read or maxSearchResultFiles matching files have been
// found, whichever comes first; a read error for one file is treated as "no
// match" rather than failing the whole search, since the file's set of
// Markdown paths and its actual readability can briefly disagree (a race with
// a delete, a GitHub blob past the point where fetchTree's cache went stale).
func searchFiles(paths []string, query string, scanLimit int, read func(path string) ([]byte, error)) searchResponse {
	resp := searchResponse{Query: query, Files: []searchFileResult{}}
	lowerQuery := strings.ToLower(query)
	scanned := 0
	for _, p := range paths {
		if scanned >= scanLimit {
			resp.Truncated = true
			break
		}
		scanned++
		data, err := read(p)
		if err != nil {
			continue
		}
		if r := searchFileContent(p, string(data), lowerQuery, query); r != nil {
			resp.Files = append(resp.Files, *r)
			if len(resp.Files) >= maxSearchResultFiles {
				resp.Truncated = true
				break
			}
		}
	}
	if scanned >= scanLimit && scanned < len(paths) {
		resp.Truncated = true
	}
	return resp
}

// searchFileContent scans content line by line for lowerQuery (content and
// query already lowercased) and returns the file's result, or nil when it has
// no matches. query (original case) is only used to size the highlighted span
// in each snippet.
func searchFileContent(relPath, content, lowerQuery, query string) *searchFileResult {
	var matches []searchMatch
	lineNo := 0
	for _, line := range strings.Split(content, "\n") {
		lineNo++
		if !strings.Contains(strings.ToLower(line), lowerQuery) {
			continue
		}
		matches = append(matches, searchMatch{Line: lineNo, Text: snippetAround(line, query)})
		if len(matches) >= maxMatchesPerFile {
			break
		}
	}
	if len(matches) == 0 {
		return nil
	}
	return &searchFileResult{Path: relPath, Matches: matches}
}

// snippetAround trims line and, if it is long, crops it to a window of
// snippetRadius runes on each side of query's first occurrence — long enough
// to give context, short enough to fit a sidebar row. Rune-based throughout
// so a multi-byte character never gets split across the crop boundary.
func snippetAround(line, query string) string {
	trimmed := strings.TrimSpace(line)
	runes := []rune(trimmed)
	byteIdx := strings.Index(strings.ToLower(trimmed), strings.ToLower(query))
	if byteIdx < 0 {
		// Cannot happen for a line that matched, but a full-line fallback is
		// harmless if it ever does.
		if len(runes) > 2*snippetRadius {
			return string(runes[:2*snippetRadius]) + "…"
		}
		return trimmed
	}
	runeIdx := len([]rune(trimmed[:byteIdx]))
	queryRunes := len([]rune(query))
	if len(runes) <= 2*snippetRadius+queryRunes {
		return trimmed
	}
	start := runeIdx - snippetRadius
	if start < 0 {
		start = 0
	}
	end := runeIdx + queryRunes + snippetRadius
	if end > len(runes) {
		end = len(runes)
	}
	snippet := string(runes[start:end])
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(runes) {
		snippet += "…"
	}
	return snippet
}
