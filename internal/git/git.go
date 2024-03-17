package git

import (
	"bufio"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	if len(sha) != 40 {
		return fmt.Errorf("Invalid SHA")
	}

	file, err := os.Open(filepath.Join(g.Root, "objects", sha[:2], sha[2:]))
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
	if _, err := io.CopyN(g.Output, reader, int64(size)); err != nil {
		return fmt.Errorf("Failed to write contents to stdout: %w", err)
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
