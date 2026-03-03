package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fsamin/phoebus/internal/syncer"
)

func runValidate() {
	dir := "."
	if len(os.Args) > 2 {
		dir = os.Args[2]
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Invalid path: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Validating content in: %s\n\n", absDir)
	errors := syncer.ValidateContent(absDir)

	if len(errors) == 0 {
		fmt.Println("✅ All content is valid!")
		os.Exit(0)
	}

	fmt.Printf("❌ Found %d error(s):\n\n", len(errors))
	for i, e := range errors {
		fmt.Printf("  %d. %s\n", i+1, e)
	}
	os.Exit(1)
}
