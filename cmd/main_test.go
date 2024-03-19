package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thebenkogan/git/internal/git"
)

func TestGit(t *testing.T) {
	testDir := t.TempDir()
	outputBuf := bytes.NewBuffer(nil)

	git := git.Git{Root: testDir, Output: outputBuf}

	t.Run("init", func(t *testing.T) {
		if err := git.Init(); err != nil {
			t.Fatal(err)
		}

		if _, err := os.Stat(filepath.Join(git.GitPath(), "objects")); os.IsNotExist(err) {
			t.Fatal("No objects folder found after init")
		}
		if _, err := os.Stat(filepath.Join(git.GitPath(), "refs")); os.IsNotExist(err) {
			t.Fatal("No refs folder found after init")
		}
		if _, err := os.Stat(filepath.Join(git.GitPath(), "HEAD")); os.IsNotExist(err) {
			t.Fatal("No HEAD file found after init")
		}

		headBytes, _ := os.ReadFile(filepath.Join(git.GitPath(), "HEAD"))
		head := string(headBytes)
		if head != "ref: refs/heads/main\n" {
			t.Fatalf("HEAD file contents are incorrect: %s", head)
		}
	})

	testObjectName1 := "test.txt"
	testObjectPath := filepath.Join(testDir, testObjectName1)
	testObjectContents := "these are test contents"
	if err := os.WriteFile(testObjectPath, []byte(testObjectContents), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("hash-object and cat-file", func(t *testing.T) {
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

	t.Run("write-tree and ls-tree", func(t *testing.T) {
		dirName := "dir"
		objectDir := filepath.Join(testDir, dirName)
		if err := os.Mkdir(objectDir, 0755); err != nil {
			t.Fatal(err)
		}

		testObjectName2 := "test2.txt"
		testObject2Path := filepath.Join(objectDir, testObjectName2)
		testObject2Contents := "these are more test contents"
		if err := os.WriteFile(testObject2Path, []byte(testObject2Contents), 0644); err != nil {
			t.Fatal(err)
		}

		testObjectName3 := "test3.txt"
		testObject3Path := filepath.Join(objectDir, testObjectName3)
		testObject3Contents := "these are even more test contents"
		if err := os.WriteFile(testObject3Path, []byte(testObject3Contents), 0644); err != nil {
			t.Fatal(err)
		}

		outputBuf.Reset()
		if err := git.WriteTree(); err != nil {
			t.Fatal(err)
		}
		treeSha := outputBuf.String()

		outputBuf.Reset()
		if err := git.LsTree(treeSha, false); err != nil {
			t.Fatal(err)
		}

		treeContents := strings.Split(outputBuf.String(), "\n")
		if len(treeContents) != 2 {
			t.Fatalf("Expected 2 lines in tree, got %d", len(treeContents))
		}

		innerTreeSha := checkTreeEntry(t, treeContents[0], "tree", "040000", dirName)
		blobSha := checkTreeEntry(t, treeContents[1], "blob", "100644", testObjectName1)

		outputBuf.Reset()
		if err := git.CatFile(blobSha); err != nil {
			t.Fatal(err)
		}
		if outputBuf.String() != testObjectContents {
			t.Fatalf("cat-file contents of tree entry are incorrect: %s", outputBuf.String())
		}

		outputBuf.Reset()
		if err := git.LsTree(innerTreeSha, false); err != nil {
			t.Fatal(err)
		}

		innerTreeContents := strings.Split(outputBuf.String(), "\n")
		if len(innerTreeContents) != 2 {
			t.Fatalf("Expected 2 lines in tree, got %d", len(innerTreeContents))
		}

		blob2Sha := checkTreeEntry(t, innerTreeContents[0], "blob", "100644", testObjectName2)
		blob3Sha := checkTreeEntry(t, innerTreeContents[1], "blob", "100644", testObjectName3)

		outputBuf.Reset()
		if err := git.CatFile(blob2Sha); err != nil {
			t.Fatal(err)
		}
		if outputBuf.String() != testObject2Contents {
			t.Fatalf("cat-file contents of tree entry are incorrect: %s", outputBuf.String())
		}

		outputBuf.Reset()
		if err := git.CatFile(blob3Sha); err != nil {
			t.Fatal(err)
		}
		if outputBuf.String() != testObject3Contents {
			t.Fatalf("cat-file contents of tree entry are incorrect: %s", outputBuf.String())
		}
	})
}

func checkTreeEntry(t *testing.T, entry, expectedType, expectedMode, expectedName string) string {
	t.Helper()

	if !strings.HasPrefix(entry, fmt.Sprintf("%s %s ", expectedMode, expectedType)) {
		t.Fatalf("tree entry has incorrect mode and type: %s", entry)
	}
	if !strings.HasSuffix(entry, expectedName) {
		t.Fatalf("tree entry has incorrect name: %s", entry)
	}

	sha := strings.Split(strings.Split(entry, " ")[2], "\t")[0]
	if len(sha) != 40 {
		t.Fatalf("Expected inner tree sha to be 40 characters long, got %s", sha)
	}

	return sha
}
