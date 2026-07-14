package tools

import (
	"bufio"
	"bytes"
	"fmt"

	"github.com/berkan-cetinkaya/pairfs/internal/workspace"
)

func ReadFile(ws *workspace.Workspace, path string, offset, limit int) (string, error) {
	data, _, err := ws.Read(path)
	if err != nil {
		return "", err
	}
	if offset < 1 {
		offset = 1
	}
	if limit <= 0 {
		limit = 200
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	line := 0
	out := ""
	for scanner.Scan() {
		line++
		if line < offset {
			continue
		}
		if line >= offset+limit {
			break
		}
		out += fmt.Sprintf("%d\t%s\n", line, scanner.Text())
	}
	return out, scanner.Err()
}
