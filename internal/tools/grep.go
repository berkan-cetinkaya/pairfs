package tools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/berkan-cetinkaya/pairfs/internal/workspace"
)

type GrepMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

func Grep(ws *workspace.Workspace, pattern, include string, max int) ([]GrepMatch, error) {
	rx, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}
	if max <= 0 {
		max = 100
	}
	var matches []GrepMatch
	err = filepath.WalkDir(ws.Root(), func(full string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(ws.Root(), full)
		rel = filepath.ToSlash(rel)
		if include != "" {
			ok, _ := filepath.Match(include, filepath.Base(rel))
			if !ok {
				return nil
			}
		}
		f, err := os.Open(full)
		if err != nil {
			return nil
		}
		defer f.Close()
		s := bufio.NewScanner(f)
		n := 0
		for s.Scan() {
			n++
			if rx.MatchString(s.Text()) {
				matches = append(matches, GrepMatch{Path: rel, Line: n, Text: strings.TrimSpace(s.Text())})
				if len(matches) >= max {
					return fmt.Errorf("__stop__")
				}
			}
		}
		return nil
	})
	if err != nil && err.Error() != "__stop__" {
		return nil, err
	}
	return matches, nil
}
