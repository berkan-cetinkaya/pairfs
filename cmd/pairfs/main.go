package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/berkan-cetinkaya/pairfs"
)

// fail writes err to standard error and terminates the process with a failure status.
func fail(err error) { fmt.Fprintln(os.Stderr, "error:", err); os.Exit(1) }

// printJSON serializes v as indented JSON and writes it to standard output.
func printJSON(v any) { b, _ := json.MarshalIndent(v, "", "  "); fmt.Println(string(b)) }

// main parses the selected pairfs subcommand and dispatches it against a workspace.
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
	workspace, err := pairfs.Open(*root)
	if err != nil {
		fail(err)
	}

	switch cmd {
	case "read":
		out, err := workspace.Read(*path, *offset, *limit)
		if err != nil {
			fail(err)
		}
		fmt.Print(out)
	case "grep":
		out, err := workspace.Grep(*pattern, *include, *limit)
		if err != nil {
			fail(err)
		}
		printJSON(out)
	case "glob":
		out, err := workspace.Glob(*pattern)
		if err != nil {
			fail(err)
		}
		printJSON(out)
	case "edit":
		if *apply {
			r, err := workspace.ApplyEdit(*path, *oldText, *newText, *hash)
			if err != nil {
				fail(err)
			}
			printJSON(r)
		} else {
			p, err := workspace.PreviewEdit(*path, *oldText, *newText)
			if err != nil {
				fail(err)
			}
			printJSON(p)
		}
	case "write":
		if *apply {
			r, err := workspace.ApplyWrite(*path, *content, pairfs.WriteMode(*mode), *hash)
			if err != nil {
				fail(err)
			}
			printJSON(r)
		} else {
			p, err := workspace.PreviewWrite(*path, *content, pairfs.WriteMode(*mode))
			if err != nil {
				fail(err)
			}
			printJSON(p)
		}
	case "delete":
		if *apply {
			r, err := workspace.ApplyDelete(*path, *hash)
			if err != nil {
				fail(err)
			}
			printJSON(r)
		} else {
			p, err := workspace.PreviewDelete(*path)
			if err != nil {
				fail(err)
			}
			printJSON(p)
		}
	case "move":
		if *apply {
			r, err := workspace.ApplyMove(*from, *to, *hash)
			if err != nil {
				fail(err)
			}
			printJSON(r)
		} else {
			p, err := workspace.PreviewMove(*from, *to)
			if err != nil {
				fail(err)
			}
			printJSON(p)
		}
	default:
		fail(fmt.Errorf("unknown command %q", cmd))
	}
}
