// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"path/filepath"
	"strings"
)

// assetTypes lists the non-Markdown files viewmd will hand out, mapped to the
// Content-Type it serves them with.
//
// This is an allowlist rather than "anything under --folder" on purpose. The
// server binds 0.0.0.0 by default and --expose puts it in front of the host,
// so the folder root is not a safe thing to serve file by file: a repo root
// holds .env, .git/config, private keys and CI secrets alongside its docs.
// Everything a Markdown page can embed or link to is here; nothing else is
// reachable.
//
// The table is explicit instead of deferring to mime.TypeByExtension because
// that consults /etc/mime.types, which a slim container image does not ship —
// video and font types in particular would come back empty there.
var assetTypes = map[string]string{
	".apng":  "image/apng",
	".avif":  "image/avif",
	".bmp":   "image/bmp",
	".gif":   "image/gif",
	".ico":   "image/x-icon",
	".jpeg":  "image/jpeg",
	".jpg":   "image/jpeg",
	".png":   "image/png",
	".svg":   "image/svg+xml",
	".webp":  "image/webp",
	".m4a":   "audio/mp4",
	".mp3":   "audio/mpeg",
	".oga":   "audio/ogg",
	".wav":   "audio/wav",
	".mov":   "video/quicktime",
	".mp4":   "video/mp4",
	".ogv":   "video/ogg",
	".webm":  "video/webm",
	".otf":   "font/otf",
	".ttf":   "font/ttf",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".pdf":   "application/pdf",
}

// assetContentType returns the Content-Type for name, or "" when the file is
// not one viewmd serves as an asset.
func assetContentType(name string) string {
	return assetTypes[strings.ToLower(filepath.Ext(name))]
}
