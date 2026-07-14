package tools

import (
	"fmt"
	"strings"
)

func UnifiedDiff(path, oldContent, newContent string) string {
	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", path, path)
	fmt.Fprintf(&b, "--- a/%s\n", path)
	fmt.Fprintf(&b, "+++ b/%s\n", path)
	fmt.Fprintf(&b, "@@ -1,%d +1,%d @@\n", len(oldLines), len(newLines))

	// Simple line diff using common prefix/suffix. Deterministic and adequate for one-file MVP.
	prefix := 0
	for prefix < len(oldLines) && prefix < len(newLines) && oldLines[prefix] == newLines[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldLines)-prefix && suffix < len(newLines)-prefix && oldLines[len(oldLines)-1-suffix] == newLines[len(newLines)-1-suffix] {
		suffix++
	}

	for i := 0; i < prefix; i++ {
		fmt.Fprintf(&b, " %s\n", oldLines[i])
	}
	for i := prefix; i < len(oldLines)-suffix; i++ {
		fmt.Fprintf(&b, "-%s\n", oldLines[i])
	}
	for i := prefix; i < len(newLines)-suffix; i++ {
		fmt.Fprintf(&b, "+%s\n", newLines[i])
	}
	for i := len(oldLines) - suffix; i < len(oldLines); i++ {
		if i >= 0 {
			fmt.Fprintf(&b, " %s\n", oldLines[i])
		}
	}
	return b.String()
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return []string{}
	}
	return strings.Split(s, "\n")
}
