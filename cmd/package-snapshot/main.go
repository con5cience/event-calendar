// package-snapshot copies one validated generation into a new build directory.
package main

import (
	"event-calendar/internal/store"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func run(args []string, errout io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(errout, "usage: package-snapshot INPUT_DIR NEW_OUTPUT_DIR")
		return 2
	}
	if err := pack(args[0], args[1]); err != nil {
		fmt.Fprintln(errout, err)
		return 1
	}
	return 0
}

func pack(input, output string) error {
	root, err := os.OpenRoot(input)
	if err != nil {
		return err
	}
	defer root.Close()
	catalog, err := store.ReadDocument(root, "catalog.json")
	if err != nil {
		return err
	}
	// Keep the exact validated bytes. Never reopen mutable input while copying.
	documents := map[string][]byte{"catalog.json": catalog}
	_, _, err = store.Validate(catalog, func(name string) ([]byte, error) {
		b, err := store.ReadDocument(root, name)
		if err == nil {
			documents[name] = b
		}
		return b, err
	})
	if err != nil {
		return err
	}
	// Refuse replacement, including the input directory. Build failure discards
	// any partial output; this is not a live publication operation.
	if err := os.Mkdir(output, 0755); err != nil {
		return err
	}
	for name, b := range documents {
		dest := filepath.Join(output, name)
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, b, 0644); err != nil {
			return err
		}
	}
	return nil
}

func main() { os.Exit(run(os.Args[1:], os.Stderr)) }
