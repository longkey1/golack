package cmd

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/longkey1/golack/internal/collector"
	"github.com/longkey1/golack/internal/config"
	"github.com/longkey1/golack/internal/dateutil"
	"github.com/longkey1/golack/internal/model"
	"github.com/longkey1/golack/internal/slack"
	"github.com/spf13/cobra"
)

var (
	historyDay      string
	historyMonth    string
	historyFrom     string
	historyTo       string
	historyChannels []string
	historyThread   bool
	historyParallel int
	historyOutput   string
)

func newHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Collect all messages in channels for a date range",
		Long: `Collect every Slack message in the given channels for a date range.
Unlike list, no author/mention filtering is applied: the full history is
collected, including @channel/@here broadcasts.

The result is written to stdout as a single JSON document, or to a file with
--output.

Date range options (mutually exclusive, one required):
  --day      Single day (YYYY-MM-DD)
  --month    Entire month (YYYY-MM)
  --from/--to Custom range (both required)

Channels (--channel) are required and accept either channel names or IDs;
names are resolved to IDs via conversations.list.

Examples:
  golack history --day 2025-01-15 --channel general
  golack history --month 2025-01 --channel general,random
  golack history -d 2025-01-15 --channel C0123ABCD --thread
  golack history -m 2025-01 --channel general --output history.json`,
		RunE: runHistory,
	}

	cmd.Flags().StringVarP(&historyDay, "day", "d", "", "Day to collect (YYYY-MM-DD)")
	cmd.Flags().StringVarP(&historyMonth, "month", "m", "", "Month to collect (YYYY-MM)")
	cmd.Flags().StringVar(&historyFrom, "from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&historyTo, "to", "", "End date (YYYY-MM-DD)")
	cmd.Flags().StringSliceVar(&historyChannels, "channel", nil, "Channels to collect (comma-separated names or IDs); required")
	cmd.Flags().BoolVar(&historyThread, "thread", false, "Get entire threads")
	cmd.Flags().IntVarP(&historyParallel, "parallel", "p", 1, "Number of parallel workers")
	cmd.Flags().StringVarP(&historyOutput, "output", "o", "", "Write JSON to this file (default: stdout)")

	return cmd
}

func runHistory(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override from flags
	if token != "" {
		cfg.Token = token
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	if len(historyChannels) == 0 {
		return fmt.Errorf("--channel is required")
	}

	// Parse date range
	dateRange, err := parseDateRange(historyDay, historyMonth, historyFrom, historyTo)
	if err != nil {
		return err
	}

	// Create Slack client
	client := slack.NewClient(cfg.Token)

	// Resolve channel names/IDs once up front.
	channels, err := client.ResolveChannels(historyChannels)
	if err != nil {
		return fmt.Errorf("failed to resolve channels: %w", err)
	}

	// Get all days to process
	days := dateRange.Days()
	if len(days) == 0 {
		return fmt.Errorf("no days to process")
	}

	fmt.Fprintf(os.Stderr, "Collecting history for %d channel(s) over %d day(s)...\n", len(channels), len(days))

	// Process days with parallelism
	results := processHistoryDays(client, channels, days, historyParallel)

	// Collect threads and errors from all days
	var allThreads []model.Thread
	var errs []error
	for _, result := range results {
		if result.Error != nil {
			errs = append(errs, fmt.Errorf("%s: %w", dateutil.FormatDate(result.Date), result.Error))
			continue
		}
		allThreads = append(allThreads, result.Threads...)
	}

	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		}
		return fmt.Errorf("%d day(s) failed", len(errs))
	}

	sortThreads(allThreads)

	if resolveIDs {
		resolver := slack.NewResolver(client)
		allThreads = resolver.ResolveThreads(allThreads)
	}

	return writeThreads(allThreads, historyOutput)
}

func processHistoryDays(client *slack.Client, channels []slack.ChannelRef, days []time.Time, parallel int) []collector.DayResult {
	if parallel < 1 {
		parallel = 1
	}

	// Create work channel
	work := make(chan time.Time, len(days))
	for _, day := range days {
		work <- day
	}
	close(work)

	// Create results channel
	results := make(chan collector.DayResult, len(days))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < parallel; i++ {
		wg.Go(func() {
			for day := range work {
				results <- processHistoryDay(client, channels, day)
			}
		})
	}

	// Wait for all workers to complete
	wg.Wait()
	close(results)

	// Collect results
	var allResults []collector.DayResult
	for result := range results {
		allResults = append(allResults, result)
	}

	return allResults
}

func processHistoryDay(client *slack.Client, channels []slack.ChannelRef, day time.Time) collector.DayResult {
	opts := collector.HistoryOptions{
		Date:       day,
		Channels:   channels,
		WithThread: historyThread,
	}

	result, err := collector.History(client, opts)
	if err != nil {
		return collector.DayResult{
			Date:  day,
			Error: err,
		}
	}

	return *result
}
