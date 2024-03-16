package main

import (
	"bufio"
	"compress/zlib"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	case "cat-file":
		err = catFile()
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

func catFile() error {
	catFileCmd := flag.NewFlagSet("cat-file", flag.ExitOnError)
	shaPtr := catFileCmd.String("p", "", "sha hash of the blob")
	_ = catFileCmd.Parse(os.Args[2:])

	if *shaPtr == "" {
		return fmt.Errorf("No SHA provided")
	}

	hash := *shaPtr
	file, err := os.Open(filepath.Join(ROOT, "objects", hash[:2], hash[2:]))
	if err != nil {
		return fmt.Errorf("Failed to open object: %w", err)
	}
	defer file.Close()

	zr, _ := zlib.NewReader(file)
	reader := bufio.NewReader(zr)

	s, err := reader.ReadString('\x00')
	if err != nil {
		return fmt.Errorf("Failed read object header: %w", err)
	}

	trimmed := strings.TrimSuffix(s, "\x00")
	split := strings.Split(trimmed, " ")
	if split[0] != "blob" {
		return fmt.Errorf("Unsupported object type: %s", split[0])
	}

	size, _ := strconv.Atoi(split[1])
	if _, err := io.CopyN(os.Stdout, reader, int64(size)); err != nil {
		return fmt.Errorf("Failed to write contents to stdout: %w", err)
	}

	return nil
}
