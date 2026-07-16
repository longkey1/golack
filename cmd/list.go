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
	listDay             string
	listMonth           string
	listFrom            string
	listTo              string
	listThread          bool
	listAuthor          string
	listMentions        []string
	listNoAuthor        bool
	listNoMention       bool
	listChannels        []string
	listExcludeChannels []string
	listParallel        int
	listOutput          string
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Collect messages for a date range and save to files",
		Long: `Collect Slack messages for a date range and save to JSON files.

Output is saved to logs/YYYY/MM/DD/slack.json for each day.

Date range options (mutually exclusive):
  --day      Single day (YYYY-MM-DD)
  --month    Entire month (YYYY-MM)
  --from/--to Custom range (both required)

Examples:
  golack list --day 2025-01-15
  golack list --month 2025-01
  golack list --from 2025-01-01 --to 2025-01-15
  golack list -m 2025-01 --thread --author U12345678
  golack list -d 2025-01-15 --mention U111 --mention @team
  golack list -m 2025-01 --channel general --channel random
  golack list -d 2025-01-15 --exclude-channel announcements`,
		RunE: runList,
	}

	cmd.Flags().StringVarP(&listDay, "day", "d", "", "Day to collect (YYYY-MM-DD)")
	cmd.Flags().StringVarP(&listMonth, "month", "m", "", "Month to collect (YYYY-MM)")
	cmd.Flags().StringVar(&listFrom, "from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&listTo, "to", "", "End date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&listThread, "thread", false, "Get entire threads")
	cmd.Flags().StringVar(&listAuthor, "author", "", "Filter by author")
	cmd.Flags().StringSliceVar(&listMentions, "mention", nil, "Filter by mention (comma-separated User IDs or @group-names; OR matched)")
	cmd.Flags().BoolVar(&listNoAuthor, "no-author", false, "Disable the author filter even if set in config")
	cmd.Flags().BoolVar(&listNoMention, "no-mention", false, "Disable the mention filter even if set in config")
	cmd.Flags().StringSliceVar(&listChannels, "channel", nil, "Filter by channel (comma-separated channel names)")
	cmd.Flags().StringSliceVar(&listExcludeChannels, "exclude-channel", nil, "Exclude channels (comma-separated channel names)")
	cmd.Flags().IntVarP(&listParallel, "parallel", "p", 1, "Number of parallel workers")
	cmd.Flags().StringVarP(&listOutput, "output", "o", "", "Write JSON to this file (default: stdout)")

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override from flags
	if token != "" {
		cfg.Token = token
	}
	// --no-author/--no-mention explicitly disable the filter, taking precedence
	// over both the config default and any value given on the command line.
	if listNoAuthor {
		listAuthor = ""
	} else if listAuthor == "" {
		listAuthor = cfg.Author
	}
	if listNoMention {
		listMentions = nil
	} else if len(listMentions) == 0 {
		listMentions = cfg.Mention
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	// Parse date range
	dateRange, err := parseDateRange(listDay, listMonth, listFrom, listTo)
	if err != nil {
		return err
	}

	// Create Slack client
	client := slack.NewClient(cfg.Token)

	// Get all days to process
	days := dateRange.Days()
	if len(days) == 0 {
		return fmt.Errorf("no days to process")
	}

	fmt.Fprintf(os.Stderr, "Collecting messages for %d day(s)...\n", len(days))

	// Process days with parallelism
	results := processdays(client, days, listParallel)

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

	return writeThreads(allThreads, listOutput)
}

// parseDateRange validates and parses the mutually exclusive date options
// shared by the list and history commands.
func parseDateRange(day, month, from, to string) (dateutil.DateRange, error) {
	// Count how many date options are specified
	count := 0
	if day != "" {
		count++
	}
	if month != "" {
		count++
	}
	if from != "" || to != "" {
		count++
	}

	if count == 0 {
		return dateutil.DateRange{}, fmt.Errorf("date range required: use --day, --month, or --from/--to")
	}
	if count > 1 {
		return dateutil.DateRange{}, fmt.Errorf("only one date range option allowed: --day, --month, or --from/--to")
	}

	if day != "" {
		d, err := dateutil.ParseDay(day)
		if err != nil {
			return dateutil.DateRange{}, err
		}
		return dateutil.DayRange(d), nil
	}

	if month != "" {
		return dateutil.ParseMonth(month)
	}

	if from != "" && to != "" {
		return dateutil.CustomRange(from, to)
	}

	return dateutil.DateRange{}, fmt.Errorf("--from and --to must both be specified")
}

func processdays(client *slack.Client, days []time.Time, parallel int) []collector.DayResult {
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
				results <- processDay(client, day)
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

func processDay(client *slack.Client, day time.Time) collector.DayResult {
	opts := collector.ListOptions{
		Date:            day,
		Author:          listAuthor,
		Mentions:        listMentions,
		Channels:        listChannels,
		ExcludeChannels: listExcludeChannels,
		WithThread:      listThread,
	}

	result, err := collector.List(client, opts)
	if err != nil {
		return collector.DayResult{
			Date:  day,
			Error: err,
		}
	}

	return *result
}
