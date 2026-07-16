package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/longkey1/golack/internal/model"
	"github.com/longkey1/golack/internal/output"
)

// newOutputWriter returns a writer for the given path. An empty path writes the
// result to stdout; otherwise a single JSON file is created at path. The
// returned closer must be called when writing is done.
func newOutputWriter(path string) (output.Writer, func() error, error) {
	if path == "" {
		return output.NewStdoutWriter(), func() error { return nil }, nil
	}
	fw, err := output.NewFileWriter(path)
	if err != nil {
		return nil, nil, err
	}
	return fw, fw.Close, nil
}

// sortThreads orders threads by the timestamp of their first message.
func sortThreads(threads []model.Thread) {
	sort.Slice(threads, func(i, j int) bool {
		if len(threads[i].Messages) == 0 || len(threads[j].Messages) == 0 {
			return false
		}
		return threads[i].Messages[0].Timestamp.Before(threads[j].Messages[0].Timestamp)
	})
}

// writeThreads writes the collected threads to the given output path (empty for
// stdout). Progress is reported on stderr so it never mixes with the JSON.
func writeThreads(threads []model.Thread, outputPath string) error {
	writer, closeOutput, err := newOutputWriter(outputPath)
	if err != nil {
		return err
	}
	defer closeOutput()

	fmt.Fprintf(os.Stderr, "Collected %d thread(s)\n", len(threads))
	if err := writer.Write(threads); err != nil {
		return err
	}
	if outputPath != "" {
		fmt.Fprintf(os.Stderr, "Saved to %s\n", outputPath)
	}
	return nil
}
