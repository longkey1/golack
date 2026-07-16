package cmd

import (
	"fmt"
	"os"

	"github.com/longkey1/golack/internal/collector"
	"github.com/longkey1/golack/internal/input"
	"github.com/longkey1/golack/internal/model"
	"github.com/longkey1/golack/internal/output"
	"github.com/spf13/cobra"
)

var (
	mergeDir       string
	mergePattern   string
	mergeRecursive bool
)

func newMergeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merge [directory]",
		Short: "Merge multiple JSON files and deduplicate threads/messages",
		Long: `Merge multiple JSON files from a directory, deduplicate threads and messages,
and output the result to stdout.

Thread deduplication: Threads with the same ThreadID are merged.
Message deduplication: Messages with the same ID keep the one with the latest timestamp.

Examples:
  golack merge ./logs
  golack merge --dir ./logs
  golack merge ./logs --pattern "slack*.json"
  golack merge ./logs -p "2025-*.json"
  golack merge ./logs --recursive
  golack merge ./logs -r -p "*.json"`,
		Args: cobra.MaximumNArgs(1),
		RunE: runMerge,
	}

	cmd.Flags().StringVarP(&mergeDir, "dir", "d", "", "Target directory")
	cmd.Flags().StringVarP(&mergePattern, "pattern", "p", "*.json", "File name glob pattern")
	cmd.Flags().BoolVarP(&mergeRecursive, "recursive", "r", false, "Search subdirectories recursively")

	return cmd
}

func runMerge(cmd *cobra.Command, args []string) error {
	// Determine directory from args or --dir flag
	var directory string
	if len(args) > 0 {
		directory = args[0]
	} else if mergeDir != "" {
		directory = mergeDir
	} else {
		return fmt.Errorf("directory required: specify as argument or use --dir flag")
	}

	// Find files
	files, err := input.FindFiles(directory, input.FindFilesOptions{
		Pattern:   mergePattern,
		Recursive: mergeRecursive,
	})
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no matching files found in %s", directory)
	}

	fmt.Fprintf(os.Stderr, "Found %d file(s) to merge\n", len(files))

	// Read all threads
	reader := input.NewFileReader()
	var allThreads []model.Thread
	successCount := 0
	failCount := 0

	for _, file := range files {
		threads, err := reader.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] %s: %v\n", file, err)
			failCount++
			continue
		}
		allThreads = append(allThreads, threads...)
		successCount++
	}

	if successCount == 0 {
		return fmt.Errorf("all files failed to read")
	}

	fmt.Fprintf(os.Stderr, "Read %d file(s) successfully, %d failed\n", successCount, failCount)

	// Merge threads
	result := collector.Merge(collector.MergeOptions{
		Threads: allThreads,
	})

	fmt.Fprintf(os.Stderr, "Merged: %d threads -> %d threads (%d duplicates removed)\n",
		result.OriginalThreadCount, result.MergedThreadCount, result.DuplicateThreads)
	fmt.Fprintf(os.Stderr, "Merged: %d messages -> %d messages (%d duplicates removed)\n",
		result.OriginalMessageCount, result.MergedMessageCount, result.DuplicateMessages)

	// Output to stdout
	writer := output.NewStdoutWriter()
	return writer.Write(result.Threads)
}
