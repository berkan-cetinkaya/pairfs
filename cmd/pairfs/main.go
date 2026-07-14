package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/berkan-cetinkaya/pairfs/internal/tools"
	"github.com/berkan-cetinkaya/pairfs/internal/workspace"
)

func fail(err error)  { fmt.Fprintln(os.Stderr, "error:", err); os.Exit(1) }
func printJSON(v any) { b, _ := json.MarshalIndent(v, "", "  "); fmt.Println(string(b)) }

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: pairfs <read|grep|glob|edit|write|delete|move>")
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	root := fs.String("root", ".", "workspace root")
	path := fs.String("path", "", "relative file path")
	from := fs.String("from", "", "source path")
	to := fs.String("to", "", "target path")
	pattern := fs.String("pattern", "", "regex or glob pattern")
	include := fs.String("include", "", "file basename filter, e.g. *.go")
	offset := fs.Int("offset", 1, "read start line")
	limit := fs.Int("limit", 200, "read max lines / grep max results")
	oldText := fs.String("old", "", "old text")
	newText := fs.String("new", "", "new text")
	content := fs.String("content", "", "file content")
	mode := fs.String("mode", "create", "write mode: create|overwrite")
	apply := fs.Bool("apply", false, "apply mutation; default is preview")
	hash := fs.String("expected-hash", "", "expected pre-apply sha256")
	_ = fs.Parse(os.Args[2:])
	ws, err := workspace.New(*root)
	if err != nil {
		fail(err)
	}

	switch cmd {
	case "read":
		out, err := tools.ReadFile(ws, *path, *offset, *limit)
		if err != nil {
			fail(err)
		}
		fmt.Print(out)
	case "grep":
		out, err := tools.Grep(ws, *pattern, *include, *limit)
		if err != nil {
			fail(err)
		}
		printJSON(out)
	case "glob":
		out, err := tools.Glob(ws, *pattern)
		if err != nil {
			fail(err)
		}
		printJSON(out)
	case "edit":
		if *apply {
			r, err := tools.ApplyEdit(ws, *path, *oldText, *newText, *hash)
			if err != nil {
				fail(err)
			}
			printJSON(r)
		} else {
			p, _, _, err := tools.PreviewEdit(ws, *path, *oldText, *newText)
			if err != nil {
				fail(err)
			}
			printJSON(p)
		}
	case "write":
		if *apply {
			r, err := tools.ApplyWrite(ws, *path, *content, *mode, *hash)
			if err != nil {
				fail(err)
			}
			printJSON(r)
		} else {
			p, _, err := tools.PreviewWrite(ws, *path, *content, *mode)
			if err != nil {
				fail(err)
			}
			printJSON(p)
		}
	case "delete":
		if *apply {
			r, err := tools.ApplyDelete(ws, *path, *hash)
			if err != nil {
				fail(err)
			}
			printJSON(r)
		} else {
			p, err := tools.PreviewDelete(ws, *path)
			if err != nil {
				fail(err)
			}
			printJSON(p)
		}
	case "move":
		if *apply {
			r, err := tools.ApplyMove(ws, *from, *to, *hash)
			if err != nil {
				fail(err)
			}
			printJSON(r)
		} else {
			p, err := tools.PreviewMove(ws, *from, *to)
			if err != nil {
				fail(err)
			}
			printJSON(p)
		}
	default:
		fail(fmt.Errorf("unknown command %q", cmd))
	}
}
