package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("No command provided")
		os.Exit(1)
	}

	var err error
	switch command := os.Args[1]; command {
	case "init":
		err = initRepo()
	default:
		err = fmt.Errorf("Unknown command %s", command)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

const ROOT string = ".bgit"

func initRepo() error {
	for _, dir := range []string{filepath.Join(ROOT, "objects"), filepath.Join(ROOT, "refs")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("Error creating directory %s: %w", dir, err)
		}
	}

	headFileContents := []byte("ref: refs/heads/main\n")
	if err := os.WriteFile(filepath.Join(ROOT, "HEAD"), headFileContents, 0644); err != nil {
		return fmt.Errorf("Error creating HEAD: %w", err)
	}

	return nil
}
