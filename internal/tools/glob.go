package tools

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/berkan-cetinkaya/pairfs/internal/workspace"
)

// Glob returns sorted workspace-relative file paths matching pattern.
// The pattern uses slash-separated paths and supports ** for recursive matching.
func Glob(ws *workspace.Workspace, pattern string) ([]string, error) {
	rx := globToRegexp(filepath.ToSlash(pattern))
	var files []string
	err := filepath.WalkDir(ws.Root(), func(full string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(ws.Root(), full)
		rel = filepath.ToSlash(rel)
		if rx.MatchString(rel) {
			files = append(files, rel)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

// globToRegexp converts a slash-separated pairfs glob into an anchored regular expression.
func globToRegexp(pattern string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c == '*' {
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString("[^/]*")
			}
		} else if c == '?' {
			b.WriteString("[^/]")
		} else {
			if strings.ContainsRune(`.+()|[]{}^$\\`, rune(c)) {
				b.WriteByte('\\')
			}
			b.WriteByte(c)
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}
