// Package text turns the API's HTML announcements into plain text.
package text

import (
	"html"
	"regexp"
	"strings"
)

var (
	tagRE   = regexp.MustCompile(`(?s)<[^>]*>`)
	blockRE = regexp.MustCompile(`(?i)<\s*(br|/p|/div|/li|/h[1-6])\s*/?>`)
	spaceRE = regexp.MustCompile(`[ \t]+`)
	linesRE = regexp.MustCompile(`\n\s*\n+`)
)

// HTML turns an announcement's HTML into plain text: block ends become line breaks, tags go,
// entities and no-break spaces become plain characters.
func HTML(s string) string {
	s = blockRE.ReplaceAllString(s, "\n")
	s = tagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = spaceRE.ReplaceAllString(s, " ")
	s = linesRE.ReplaceAllString(s, "\n")
	return strings.TrimSpace(s)
}
