package input

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// writeFile creates a file with the given content, creating parent
// directories as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		wantThreads int
		wantErr     bool
	}{
		{
			name: "valid threads json",
			content: `[
  {
    "thread_id": "1.0",
    "channel": "general",
    "messages": [
      {"id": "1.0", "type": "slack_message", "content": "hello", "author": "U123",
       "timestamp": "2025-01-15T10:30:00Z", "channel": "general", "channel_id": "C123",
       "thread_ts": "1.0", "is_thread_parent": true}
    ]
  }
]`,
			wantThreads: 1,
		},
		{
			name:        "empty array",
			content:     `[]`,
			wantThreads: 0,
		},
		{
			name:    "invalid json",
			content: `not json`,
			wantErr: true,
		},
		{
			name:    "json object instead of array",
			content: `{"thread_id": "1.0"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "threads.json")
			writeFile(t, path, tt.content)

			got, err := NewFileReader().ReadFile(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != tt.wantThreads {
				t.Errorf("ReadFile() returned %d threads, want %d", len(got), tt.wantThreads)
			}
		})
	}
}

func TestReadFileFieldMapping(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "threads.json")
	writeFile(t, path, `[{"thread_id": "1.0", "channel_id": "C123", "messages": [{"id": "1.0", "content": "hi"}]}]`)

	got, err := NewFileReader().ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(got))
	}
	if got[0].ThreadID != "1.0" || got[0].ChannelID != "C123" {
		t.Errorf("thread = (%q, %q), want (1.0, C123)", got[0].ThreadID, got[0].ChannelID)
	}
	if len(got[0].Messages) != 1 || got[0].Messages[0].Content != "hi" {
		t.Errorf("messages = %+v, want one message with content %q", got[0].Messages, "hi")
	}
}

func TestReadFileMissing(t *testing.T) {
	t.Parallel()

	_, err := NewFileReader().ReadFile(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Error("ReadFile() with a missing file should return an error")
	}
}

func TestFindFiles(t *testing.T) {
	t.Parallel()

	// Directory layout:
	//   a.json
	//   b.json
	//   note.txt
	//   sub/c.json
	newTree := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "a.json"), "[]")
		writeFile(t, filepath.Join(dir, "b.json"), "[]")
		writeFile(t, filepath.Join(dir, "note.txt"), "")
		writeFile(t, filepath.Join(dir, "sub", "c.json"), "[]")
		return dir
	}

	tests := []struct {
		name    string
		opts    FindFilesOptions
		want    []string // relative to the tree root
		wantErr bool
	}{
		{
			name: "default pattern non-recursive",
			opts: FindFilesOptions{},
			want: []string{"a.json", "b.json"},
		},
		{
			name: "explicit pattern",
			opts: FindFilesOptions{Pattern: "b*.json"},
			want: []string{"b.json"},
		},
		{
			name: "recursive includes subdirectories",
			opts: FindFilesOptions{Recursive: true},
			want: []string{"a.json", "b.json", filepath.Join("sub", "c.json")},
		},
		{
			name: "pattern with no matches",
			opts: FindFilesOptions{Pattern: "*.xml"},
			want: nil,
		},
		{
			name:    "invalid pattern",
			opts:    FindFilesOptions{Pattern: "["},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := newTree(t)
			got, err := FindFiles(dir, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Fatalf("FindFiles() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			var rel []string
			for _, f := range got {
				r, err := filepath.Rel(dir, f)
				if err != nil {
					t.Fatal(err)
				}
				rel = append(rel, r)
			}
			sort.Strings(rel)

			if !reflect.DeepEqual(rel, tt.want) {
				t.Errorf("FindFiles() = %v, want %v", rel, tt.want)
			}
		})
	}
}

func TestFindFilesErrors(t *testing.T) {
	t.Parallel()

	t.Run("directory not found", func(t *testing.T) {
		t.Parallel()

		_, err := FindFiles(filepath.Join(t.TempDir(), "missing"), FindFilesOptions{})
		if err == nil {
			t.Error("FindFiles() with a missing directory should return an error")
		}
	})

	t.Run("path is not a directory", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "file.json")
		writeFile(t, path, "[]")
		_, err := FindFiles(path, FindFilesOptions{})
		if err == nil {
			t.Error("FindFiles() with a file path should return an error")
		}
	})
}
