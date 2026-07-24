package pairfs_test

import (
	"context"
	"fmt"

	"github.com/berkan-cetinkaya/pairfs"
)

func Example() {
	ctx := context.Background()
	fs, err := pairfs.Open("testdata/base")
	if err != nil {
		panic(err)
	}

	files, err := fs.ListFiles(ctx)
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		content, err := fs.ReadFile(ctx, file.Path)
		if err != nil {
			panic(err)
		}
		fmt.Println(file.Path, file.Size, len(content))
	}

	// Output:
	// README.md 36 36
	// internal/todo/model.go 60 60
}
