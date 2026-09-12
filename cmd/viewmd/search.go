// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"regexp"
	"strings"
)

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

// searchOptions toggles how query is interpreted. The zero value is the
// original behavior: a case-insensitive substring search.
type searchOptions struct {
	CaseSensitive bool
	Regex         bool
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

// lineMatcher finds the first match of a query within a line, returning its
// byte range [start, end) and whether it matched at all.
type lineMatcher func(line string) (start, end int, ok bool)

// newLineMatcher builds a lineMatcher for query under opts. In regex mode,
// query is compiled as a Go regular expression (RE2 syntax), case-insensitive
// unless opts.CaseSensitive; a bad pattern is reported as an error rather than
// silently matching nothing, so the caller can tell the requester what's
// wrong. In plain mode it is always a valid matcher: a substring search,
// case-insensitive unless opts.CaseSensitive.
func newLineMatcher(query string, opts searchOptions) (lineMatcher, error) {
	if opts.Regex {
		pattern := query
		if !opts.CaseSensitive {
			pattern = "(?i)" + pattern
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		return func(line string) (int, int, bool) {
			loc := re.FindStringIndex(line)
			if loc == nil {
				return 0, 0, false
			}
			return loc[0], loc[1], true
		}, nil
	}
	needle := query
	if !opts.CaseSensitive {
		needle = strings.ToLower(needle)
	}
	return func(line string) (int, int, bool) {
		hay := line
		if !opts.CaseSensitive {
			hay = strings.ToLower(hay)
		}
		idx := strings.Index(hay, needle)
		if idx < 0 {
			return 0, 0, false
		}
		return idx, idx + len(needle), true
	}, nil
}

// searchFiles runs query (interpreted per opts) across the content of paths
// (relative, slash-separated, as returned by listMarkdownPaths or
// flattenTreePaths), reading each file with read. It stops once scanLimit
// files have been read or maxSearchResultFiles matching files have been
// found, whichever comes first; a read error for one file is treated as "no
// match" rather than failing the whole search, since the file's set of
// Markdown paths and its actual readability can briefly disagree (a race with
// a delete, a GitHub blob past the point where fetchTree's cache went stale).
// An error is returned only when opts.Regex's query fails to compile.
func searchFiles(paths []string, query string, opts searchOptions, scanLimit int, read func(path string) ([]byte, error)) (searchResponse, error) {
	match, err := newLineMatcher(query, opts)
	if err != nil {
		return searchResponse{}, err
	}
	resp := searchResponse{Query: query, Files: []searchFileResult{}}
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
		if r := searchFileContent(p, string(data), match); r != nil {
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
	return resp, nil
}

// searchFileContent scans content line by line for match and returns the
// file's result, or nil when it has no matches.
func searchFileContent(relPath, content string, match lineMatcher) *searchFileResult {
	var matches []searchMatch
	lineNo := 0
	for _, line := range strings.Split(content, "\n") {
		lineNo++
		if _, _, ok := match(line); !ok {
			continue
		}
		matches = append(matches, searchMatch{Line: lineNo, Text: snippetAround(line, match)})
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
// snippetRadius runes on each side of match's first occurrence in the trimmed
// line — long enough to give context, short enough to fit a sidebar row.
// Rune-based throughout so a multi-byte character never gets split across the
// crop boundary.
func snippetAround(line string, match lineMatcher) string {
	trimmed := strings.TrimSpace(line)
	runes := []rune(trimmed)
	start, end, ok := match(trimmed)
	if !ok {
		// Cannot happen for a line that matched before trimming, but a
		// full-line fallback is harmless if it ever does.
		if len(runes) > 2*snippetRadius {
			return string(runes[:2*snippetRadius]) + "…"
		}
		return trimmed
	}
	runeIdx := len([]rune(trimmed[:start]))
	matchRunes := len([]rune(trimmed[start:end]))
	if len(runes) <= 2*snippetRadius+matchRunes {
		return trimmed
	}
	rStart := runeIdx - snippetRadius
	if rStart < 0 {
		rStart = 0
	}
	rEnd := runeIdx + matchRunes + snippetRadius
	if rEnd > len(runes) {
		rEnd = len(runes)
	}
	snippet := string(runes[rStart:rEnd])
	if rStart > 0 {
		snippet = "…" + snippet
	}
	if rEnd < len(runes) {
		snippet += "…"
	}
	return snippet
}
