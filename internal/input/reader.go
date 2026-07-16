package input

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/longkey1/golack/internal/model"
)

// FileReader reads Thread arrays from JSON files
type FileReader struct{}

// NewFileReader creates a new FileReader
func NewFileReader() *FileReader {
	return &FileReader{}
}

// ReadFile reads threads from a single JSON file
func (r *FileReader) ReadFile(path string) ([]model.Thread, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var threads []model.Thread
	if err := json.Unmarshal(data, &threads); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return threads, nil
}

// FindFilesOptions specifies options for FindFiles
type FindFilesOptions struct {
	Pattern   string
	Recursive bool
}

// FindFiles finds JSON files matching the pattern in the directory
func FindFiles(directory string, opts FindFilesOptions) ([]string, error) {
	if opts.Pattern == "" {
		opts.Pattern = "*.json"
	}

	// Check if directory exists
	info, err := os.Stat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("directory not found: %s", directory)
		}
		return nil, fmt.Errorf("failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", directory)
	}

	var files []string

	if opts.Recursive {
		err = filepath.WalkDir(directory, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			matched, err := filepath.Match(opts.Pattern, d.Name())
			if err != nil {
				return fmt.Errorf("invalid pattern: %w", err)
			}
			if matched {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			matched, err := filepath.Match(opts.Pattern, entry.Name())
			if err != nil {
				return nil, fmt.Errorf("invalid pattern: %w", err)
			}
			if matched {
				files = append(files, filepath.Join(directory, entry.Name()))
			}
		}
	}

	return files, nil
}
