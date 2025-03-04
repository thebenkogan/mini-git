package git

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/thebenkogan/git/internal/objects"
)

type Git struct {
	Root   string    // root directory of project
	Output io.Writer // where to write command outputs
}

const GIT_DIR string = ".bgit"

func (g *Git) GitPath() string {
	return filepath.Join(g.Root, GIT_DIR)
}

func (g *Git) Init() error {
	gitPath := g.GitPath()

	for _, dir := range []string{filepath.Join(gitPath, "objects"), filepath.Join(gitPath, "refs")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("Error creating directory %s: %w", dir, err)
		}
	}

	headFileContents := []byte("ref: refs/heads/main\n")
	if err := os.WriteFile(filepath.Join(gitPath, "HEAD"), headFileContents, 0644); err != nil {
		return fmt.Errorf("Error creating HEAD: %w", err)
	}

	return nil
}

func (g *Git) CatFile(sha string) error {
	object, err := objects.ReadObject(g.GitPath(), sha)
	if err != nil {
		return fmt.Errorf("Failed to open object: %w", err)
	}
	defer object.Close()

	if object.Typ == "tree" {
		return fmt.Errorf("Use ls-tree instead")
	}

	if _, err := io.CopyN(g.Output, object.Reader, int64(object.Size)); err != nil {
		return fmt.Errorf("Failed to write contents to output: %w", err)
	}

	return nil
}

func (g *Git) HashObject(path string, write bool) error {
	sha, err := objects.WriteBlob(g.GitPath(), path, write)
	if err != nil {
		return err
	}
	_, _ = g.Output.Write([]byte(sha))
	return nil
}

func (g *Git) LsTree(sha string, nameOnly bool) error {
	object, err := objects.ReadObject(g.GitPath(), sha)
	if err != nil {
		return fmt.Errorf("Failed to open object: %w", err)
	}
	defer object.Close()

	if object.Typ != "tree" {
		return fmt.Errorf("Expected tree object, got: %s", object.Typ)
	}

	size := object.Size
	lines := make([]string, 0)
	for size > 0 {
		bytes, err := object.Reader.ReadBytes('\x00')
		if err != nil {
			return fmt.Errorf("Failed to read tree entry: %w", err)
		}
		trimmed := strings.TrimSuffix(string(bytes), "\x00")

		mode, name, _ := strings.Cut(trimmed, " ")
		shaBytes := make([]byte, 20)
		if _, err := object.Reader.Read(shaBytes); err != nil {
			return fmt.Errorf("Failed to read entry sha: %w", err)
		}

		var line string
		if nameOnly {
			line = name
		} else {
			objectType := "blob"
			if mode == objects.TreeDirMode {
				objectType = "tree"
			}
			line = fmt.Sprintf("%s %s %s\t%s", mode, objectType, hex.EncodeToString(shaBytes), name)
		}

		lines = append(lines, line)
		size = size - len(bytes) - 20
	}

	_, _ = g.Output.Write([]byte(strings.Join(lines, "\n")))

	return nil
}

func (g *Git) ignoredPaths() []string {
	return []string{
		g.GitPath(),
		filepath.Join(g.Root, ".git"),
	}
}

func (g *Git) WriteTree() error {
	sha, err := objects.WriteTree(g.GitPath(), g.Root, g.ignoredPaths())
	if err != nil {
		return err
	}
	_, _ = g.Output.Write([]byte(sha))
	return nil
}

func (g *Git) CommitTree(treeSha, message, parentSha string) error {
	sha, err := objects.WriteCommit(g.GitPath(), treeSha, message, parentSha)
	if err != nil {
		return err
	}
	_, _ = g.Output.Write([]byte(sha))
	return nil
}
