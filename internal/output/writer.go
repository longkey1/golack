package output

import (
	"encoding/json"
	"io"
)

// Writer interface for writing output
type Writer interface {
	Write(data any) error
}

// JSONWriter writes JSON formatted output
type JSONWriter struct {
	w      io.Writer
	indent bool
}

// NewJSONWriter creates a new JSON writer
func NewJSONWriter(w io.Writer, indent bool) *JSONWriter {
	return &JSONWriter{
		w:      w,
		indent: indent,
	}
}

// Write writes the data as JSON
func (jw *JSONWriter) Write(data any) error {
	encoder := json.NewEncoder(jw.w)
	if jw.indent {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(data)
}
