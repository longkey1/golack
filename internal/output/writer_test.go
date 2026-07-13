package output

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONWriter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		indent bool
		data   any
		want   string
	}{
		{
			name:   "compact output",
			indent: false,
			data:   map[string]string{"key": "value"},
			want:   "{\"key\":\"value\"}\n",
		},
		{
			name:   "indented output",
			indent: true,
			data:   map[string]string{"key": "value"},
			want:   "{\n  \"key\": \"value\"\n}\n",
		},
		{
			name:   "array output",
			indent: false,
			data:   []int{1, 2, 3},
			want:   "[1,2,3]\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := NewJSONWriter(&buf, tt.indent).Write(tt.data); err != nil {
				t.Fatalf("Write() error = %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("Write() output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFileWriter(t *testing.T) {
	t.Parallel()

	// The parent directory does not exist yet; NewFileWriter must create it.
	path := filepath.Join(t.TempDir(), "nested", "dir", "out.json")

	fw, err := NewFileWriter(path)
	if err != nil {
		t.Fatalf("NewFileWriter() error = %v", err)
	}
	if fw.Path() != path {
		t.Errorf("Path() = %q, want %q", fw.Path(), path)
	}

	if err := fw.Write([]string{"a", "b"}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := fw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}

	// File output is indented JSON.
	if !strings.Contains(string(data), "\n  ") {
		t.Errorf("file output is not indented: %q", string(data))
	}

	var got []string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("round-tripped data = %v, want [a b]", got)
	}
}
