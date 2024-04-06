package objects

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

func ReadObject(gitPath string, sha string) (*bufio.Reader, string, int, error) {
	if len(sha) != 40 {
		return nil, "", 0, fmt.Errorf("Invalid SHA")
	}

	file, err := os.Open(filepath.Join(gitPath, "objects", sha[:2], sha[2:]))
	if err != nil {
		return nil, "", 0, fmt.Errorf("Failed to open object: %w", err)
	}
	defer file.Close()

	zr, _ := zlib.NewReader(file)
	reader := bufio.NewReader(zr)

	s, err := reader.ReadString('\x00')
	if err != nil {
		return nil, "", 0, fmt.Errorf("Failed to read object header: %w", err)
	}

	trimmed := strings.TrimSuffix(s, "\x00")
	split := strings.Split(trimmed, " ")
	objectType := split[0]
	size, _ := strconv.Atoi(split[1])

	return reader, objectType, size, nil
}

func objectHeader(typ string, size int) string {
	return fmt.Sprintf("%s %d\x00", typ, size)
}

func writeObject(gitPath string, sha string, header string, source io.Reader) error {
	dir := filepath.Join(gitPath, "objects", sha[:2])
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("Failed to create object directory: %w", err)
	}

	object, err := os.Create(filepath.Join(dir, sha[2:]))
	if err != nil {
		return fmt.Errorf("Failed to create object file: %w", err)
	}
	defer object.Close()

	zw := zlib.NewWriter(object)
	_, _ = zw.Write([]byte(header))
	if _, err := io.Copy(zw, source); err != nil {
		return fmt.Errorf("Failed to write object: %w", err)
	}
	zw.Close()

	return nil
}

func hashObject(header string, source io.ReadSeeker, reset bool) (string, error) {
	hash := sha1.New()
	hash.Write([]byte(header))
	if _, err := io.Copy(hash, source); err != nil {
		return "", fmt.Errorf("Failed to read source into hash: %w", err)
	}

	if reset {
		if _, err := source.Seek(0, io.SeekStart); err != nil {
			return "", fmt.Errorf("Failed to reset reader: %w", err)
		}
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func WriteBlob(gitPath string, path string, write bool) (string, error) {
	source, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("Failed to open object: %w", err)
	}
	defer source.Close()
	info, _ := source.Stat()
	header := objectHeader("blob", int(info.Size()))

	sha, err := hashObject(header, source, write)
	if err != nil {
		return "", err
	}

	if write {
		if err := writeObject(gitPath, sha, header, source); err != nil {
			return "", err
		}
	}

	return sha, nil
}

const DEFAULT_AUTHOR_NAME string = "Ben"
const DEFAULT_AUTHOR_EMAIL string = "benkogan9@gmail.com"

func WriteCommit(gitPath, treeSha, message, parentSha string) (string, error) {
	contents := make([]byte, 0)
	contents = append(contents, []byte(fmt.Sprintf("tree %s\n", treeSha))...)
	if parentSha != "" {
		contents = append(contents, []byte(fmt.Sprintf("parent %s\n", parentSha))...)
	}

	t := time.Now()
	dateSeconds := t.Unix()
	timezone := t.Format("-0700")

	authorLine := fmt.Sprintf("author %s <%s> %d %s\n", DEFAULT_AUTHOR_NAME, DEFAULT_AUTHOR_EMAIL, dateSeconds, timezone)
	contents = append(contents, []byte(authorLine)...)
	committerLine := fmt.Sprintf("committer %s <%s> %d %s\n\n", DEFAULT_AUTHOR_NAME, DEFAULT_AUTHOR_EMAIL, dateSeconds, timezone)
	contents = append(contents, []byte(committerLine)...)

	contents = append(contents, []byte(fmt.Sprintf("%s\n", message))...)

	header := objectHeader("commit", len(contents))
	commitReader := bytes.NewReader(contents)
	sha, err := hashObject(header, commitReader, true)
	if err != nil {
		return "", err
	}

	if err := writeObject(gitPath, sha, header, commitReader); err != nil {
		return "", err
	}

	return sha, nil
}

const (
	TreeBlobMode string = "100644"
	TreeDirMode  string = "040000"
)

func WriteTree(gitPath string, root string, ignoredPaths []string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("Failed to read directory: %w", err)
	}

	type entryResult struct {
		bytes []byte
		index int
		err   error
	}
	entriesChan := make(chan entryResult, len(entries))
	numEntries := 0

	for i, entry := range entries {
		path := filepath.Join(root, entry.Name())
		if slices.Contains(ignoredPaths, path) {
			continue
		}
		numEntries++

		go func() {
			var sha string
			var mode string
			var err error
			if entry.IsDir() {
				sha, err = WriteTree(gitPath, path, ignoredPaths)
				mode = TreeDirMode
			} else {
				sha, err = WriteBlob(gitPath, path, true)
				mode = TreeBlobMode
			}
			name := entry.Name()

			if err != nil {
				entriesChan <- entryResult{err: err}
				return
			}

			shaBytes, _ := hex.DecodeString(sha)
			entryBytes := append([]byte(fmt.Sprintf("%s %s\x00", mode, name)), shaBytes...)
			entriesChan <- entryResult{bytes: entryBytes, index: i}
		}()
	}

	results := make([]entryResult, 0)
	for i := 0; i < numEntries; i++ {
		result := <-entriesChan
		if result.err != nil {
			return "", result.err
		}
		results = append(results, result)
	}
	slices.SortFunc(results, func(a, b entryResult) int {
		return a.index - b.index
	})

	entriesBytes := make([]byte, 0)
	for _, result := range results {
		entriesBytes = append(entriesBytes, result.bytes...)
	}

	header := objectHeader("tree", len(entriesBytes))
	entryReader := bytes.NewReader(entriesBytes)
	sha, err := hashObject(header, entryReader, true)
	if err != nil {
		return "", err
	}

	if err := writeObject(gitPath, sha, header, entryReader); err != nil {
		return "", err
	}

	return sha, nil
}
