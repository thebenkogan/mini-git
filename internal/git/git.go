package git

import (
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/thebenkogan/git/internal/objects"
)

type Git struct {
	Root   string
	Output io.Writer
}

func (g *Git) Init() error {
	for _, dir := range []string{filepath.Join(g.Root, "objects"), filepath.Join(g.Root, "refs")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("Error creating directory %s: %w", dir, err)
		}
	}

	headFileContents := []byte("ref: refs/heads/main\n")
	if err := os.WriteFile(filepath.Join(g.Root, "HEAD"), headFileContents, 0644); err != nil {
		return fmt.Errorf("Error creating HEAD: %w", err)
	}

	return nil
}

func (g *Git) CatFile(sha string) error {
	reader, objectType, size, err := objects.ReadObject(g.Root, sha)
	if err != nil {
		return fmt.Errorf("Failed to open object: %w", err)
	}

	if objectType != "blob" {
		return fmt.Errorf("Unsupported object type: %s", objectType)
	}

	if _, err := io.CopyN(g.Output, reader, int64(size)); err != nil {
		return fmt.Errorf("Failed to write contents to output: %w", err)
	}

	return nil
}

func (g *Git) HashObject(path string, write bool) error {
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
	_, _ = g.Output.Write([]byte(sha))

	if write {
		if _, err := source.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("Failed to reset file: %w", err)
		}

		dir := filepath.Join(g.Root, "objects", sha[:2])
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

func (g *Git) LsTree(sha string, nameOnly bool) error {
	reader, objectType, size, err := objects.ReadObject(g.Root, sha)
	if err != nil {
		return fmt.Errorf("Failed to open object: %w", err)
	}

	if objectType != "tree" {
		return fmt.Errorf("Expected tree object, got: %s", objectType)
	}

	for size > 0 {
		bytes, err := reader.ReadBytes('\x00')
		if err != nil {
			return fmt.Errorf("Failed to tree entry: %w", err)
		}
		trimmed := strings.TrimSuffix(string(bytes), "\x00")

		mode, name, _ := strings.Cut(trimmed, " ")
		shaBytes := make([]byte, 20)
		if _, err := reader.Read(shaBytes); err != nil {
			return fmt.Errorf("Failed to read entry sha: %w", err)
		}

		var line string
		if nameOnly {
			line = name + "\n"
		} else {
			objectType := "blob"
			if mode == "040000" {
				objectType = "tree"
			}
			line = fmt.Sprintf("%s %s %s\t%s\n", mode, objectType, hex.EncodeToString(shaBytes), name)
		}

		if _, err := g.Output.Write([]byte(line)); err != nil {
			return fmt.Errorf("Failed to write entry: %w", err)
		}

		size = size - len(bytes) - 20
	}

	return nil
}
