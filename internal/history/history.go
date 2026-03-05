package history

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Append adds a new query to the history file if it is not empty and not a duplicate of the previous entry.
func Append(historyFile, query string) error {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	last, err := readLastLine(historyFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if last == query {
		return nil
	}
	f, err := os.OpenFile(historyFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, query); err != nil {
		return fmt.Errorf("write history: %w", err)
	}
	return nil
}

// ReadAll returns the entire query history newest first.
func ReadAll(historyFile string) ([]string, error) {
	f, err := os.Open(historyFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open history: %w", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	// reverse to return newest first
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	return lines, scanner.Err()
}

func readLastLine(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var last string
	for scanner.Scan() {
		last = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if last == "" {
		return "", os.ErrNotExist
	}
	return strings.TrimSpace(last), nil
}
