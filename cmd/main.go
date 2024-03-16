package main

import (
	"bufio"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
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

	git := Git{root: ".bgit"}

	var err error
	switch command := os.Args[1]; command {
	case "init":
		err = git.InitRepo()
	case "cat-file":
		err = git.CatFile()
	case "hash-object":
		err = git.HashObject()
	default:
		err = fmt.Errorf("Unknown command %s", command)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type Git struct {
	root string
}

func (g *Git) InitRepo() error {
	for _, dir := range []string{filepath.Join(g.root, "objects"), filepath.Join(g.root, "refs")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("Error creating directory %s: %w", dir, err)
		}
	}

	headFileContents := []byte("ref: refs/heads/main\n")
	if err := os.WriteFile(filepath.Join(g.root, "HEAD"), headFileContents, 0644); err != nil {
		return fmt.Errorf("Error creating HEAD: %w", err)
	}

	return nil
}

func (g *Git) CatFile() error {
	catFileCmd := flag.NewFlagSet("cat-file", flag.ExitOnError)
	shaPtr := catFileCmd.String("p", "", "sha hash of the blob")
	_ = catFileCmd.Parse(os.Args[2:])

	if *shaPtr == "" {
		return fmt.Errorf("No SHA provided")
	}

	hash := *shaPtr
	file, err := os.Open(filepath.Join(g.root, "objects", hash[:2], hash[2:]))
	if err != nil {
		return fmt.Errorf("Failed to open object: %w", err)
	}
	defer file.Close()

	zr, _ := zlib.NewReader(file)
	defer zr.Close()
	reader := bufio.NewReader(zr)

	s, err := reader.ReadString('\x00')
	if err != nil {
		return fmt.Errorf("Failed to read object header: %w", err)
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

func (g *Git) HashObject() error {
	hashObjectCmd := flag.NewFlagSet("cat-file", flag.ExitOnError)
	writePtr := hashObjectCmd.Bool("w", false, "write the blob to the objects store")
	_ = hashObjectCmd.Parse(os.Args[2:])

	path := hashObjectCmd.Args()[0]
	source, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("Failed to open object: %w", err)
	}
	defer source.Close()
	info, _ := source.Stat()

	hash := sha1.New()
	hash.Write([]byte(fmt.Sprintf("blob %d\x00", info.Size())))
	if _, err := io.Copy(hash, source); err != nil {
		return fmt.Errorf("Failed to read object: %w", err)
	}

	sha := hex.EncodeToString(hash.Sum(nil))
	fmt.Println(sha)

	if *writePtr {
		if _, err := source.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("Failed to reset file: %w", err)
		}

		dir := filepath.Join(g.root, "objects", sha[:2])
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("Failed to create object directory: %w", err)
		}

		object, err := os.Create(filepath.Join(dir, sha[2:]))
		if err != nil {
			return fmt.Errorf("Failed to create object file: %w", err)
		}
		defer object.Close()

		zw := zlib.NewWriter(object)
		_, _ = zw.Write([]byte(fmt.Sprintf("blob %d\x00", info.Size())))
		if _, err := io.Copy(zw, source); err != nil {
			return fmt.Errorf("Failed to write object: %w", err)
		}
		zw.Close()
	}

	return nil
}
