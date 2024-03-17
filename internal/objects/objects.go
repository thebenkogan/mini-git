package objects

import (
	"bufio"
	"compress/zlib"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ReadObject(root string, sha string) (*bufio.Reader, string, int, error) {
	if len(sha) != 40 {
		return nil, "", 0, fmt.Errorf("Invalid SHA")
	}

	file, err := os.Open(filepath.Join(root, "objects", sha[:2], sha[2:]))
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
