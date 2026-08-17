package config

import (
	"net/http"
	"strings"
)

// AllowedMediaMimeTypes defines the allowed MIME types for media files
// This is the single source of truth for media type validation
// Note: SVG is intentionally excluded — SVG can embed scripts and would
// enable stored XSS when served inline from the media endpoint.
var AllowedMediaMimeTypes = []string{
	"image/jpeg",
	"image/png",
	"image/gif",
	"image/webp",
}

// allowedMediaMimeSet is a map for fast lookup
var allowedMediaMimeSet = func() map[string]bool {
	m := make(map[string]bool, len(AllowedMediaMimeTypes))
	for _, t := range AllowedMediaMimeTypes {
		m[t] = true
	}
	return m
}()

// IsAllowedMediaType checks if the file content has an allowed MIME type
// Returns the detected MIME type and whether it's allowed
func IsAllowedMediaType(data []byte) (string, bool) {
	// http.DetectContentType reads at most 512 bytes
	mimeType := http.DetectContentType(data)
	// DetectContentType may return "text/xml; charset=utf-8" for SVG
	// so we need to handle the base type
	baseMime := strings.Split(mimeType, ";")[0]
	baseMime = strings.TrimSpace(baseMime)

	// SVG is never allowed (stored XSS risk)
	if baseMime == "text/xml" || baseMime == "application/xml" {
		return baseMime, false
	}

	return baseMime, allowedMediaMimeSet[baseMime]
}
