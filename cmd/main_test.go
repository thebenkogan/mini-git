package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGit(t *testing.T) {
	testDir := t.TempDir()

	outputBuf := bytes.NewBuffer(nil)
	git := Git{root: testDir, output: outputBuf}

	t.Run("init", func(t *testing.T) {
		if err := git.Init(); err != nil {
			t.Fatal(err)
		}

		if _, err := os.Stat(filepath.Join(testDir, "objects")); os.IsNotExist(err) {
			t.Fatal("No objects folder found after init")
		}
		if _, err := os.Stat(filepath.Join(testDir, "refs")); os.IsNotExist(err) {
			t.Fatal("No refs folder found after init")
		}
		if _, err := os.Stat(filepath.Join(testDir, "HEAD")); os.IsNotExist(err) {
			t.Fatal("No HEAD file found after init")
		}

		headBytes, _ := os.ReadFile(filepath.Join(testDir, "HEAD"))
		head := string(headBytes)
		if head != "ref: refs/heads/main\n" {
			t.Fatalf("HEAD file contents are incorrect: %s", head)
		}
	})

	t.Run("hash-object and cat-file", func(t *testing.T) {
		testObjectPath := filepath.Join(testDir, "test.txt")
		testObjectContents := "these are test contents"
		if err := os.WriteFile(testObjectPath, []byte(testObjectContents), 0644); err != nil {
			t.Fatal(err)
		}

		if err := git.HashObject(testObjectPath, false); err != nil {
			t.Fatal(err)
		}
		hash := outputBuf.String()

		if len(hash) != 40 {
			t.Fatalf("Expected hash to be 40 characters long, got %d", len(hash))
		}

		err := git.CatFile(hash)
		if err == nil {
			t.Fatal("Expected error when cat-file is called with non-existent hash")
		}

		if err := git.HashObject(testObjectPath, true); err != nil {
			t.Fatal(err)
		}

		outputBuf.Reset()
		err = git.CatFile(hash)
		if err != nil {
			t.Fatal(err)
		}

		contents := outputBuf.String()
		if contents != testObjectContents {
			t.Fatalf("cat-file contents are incorrect: %s", contents)
		}
	})
}
