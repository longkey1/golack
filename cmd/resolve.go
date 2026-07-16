package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/longkey1/golack/internal/config"
	"github.com/longkey1/golack/internal/output"
	"github.com/longkey1/golack/internal/slack"
	"github.com/spf13/cobra"
)

var (
	resolveTypeFilter      string
	resolveAsJSON          bool
	resolveIncludeArchived bool
)

// queryKind classifies how a resolve query should be looked up.
type queryKind int

const (
	kindID queryKind = iota
	kindName
	kindEmail
)

// resolveQuery is a classified resolve argument.
type resolveQuery struct {
	raw   string
	kind  queryKind
	types []string // lookup targets (slack.TypeUser etc.); single element for kindID/kindEmail
	value string   // normalized value (prefix stripped)
}

func newResolveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve <query>...",
		Short: "Resolve Slack IDs to names and names to IDs",
		Long: `Resolve Slack user/channel/usergroup IDs to names, and names to IDs.

Queries are classified automatically:
  - IDs by prefix: U/W... (user), C/G/D... (channel), S... (usergroup)
  - "#name" searches channels, "@name" searches users and usergroups
  - "name@example.com" looks up a user by email
  - a bare name searches all types

Output is one line per match: type<TAB>id<TAB>name (or a JSON array with --json).
Queries that cannot be resolved are reported on stderr; the command fails only
when nothing could be resolved at all.

Examples:
  golack resolve U0123ABCD
  golack resolve "#general" "@john.doe"
  golack resolve --type user john.doe
  golack resolve --json C0123ABCD S0123ABCD`,
		Args: cobra.MinimumNArgs(1),
		RunE: runResolve,
	}

	cmd.Flags().StringVar(&resolveTypeFilter, "type", "", "Restrict lookup to one type: user, channel, or usergroup")
	cmd.Flags().BoolVar(&resolveAsJSON, "json", false, "Output as a JSON array instead of TSV lines")
	cmd.Flags().BoolVar(&resolveIncludeArchived, "include-archived", false, "Include archived channels in channel name lookups (slower)")

	return cmd
}

func runResolve(cmd *cobra.Command, args []string) error {
	if err := validateTypeFilter(resolveTypeFilter); err != nil {
		return err
	}

	queries := make([]resolveQuery, 0, len(args))
	for _, arg := range args {
		q, err := classifyQuery(arg, resolveTypeFilter)
		if err != nil {
			return err
		}
		queries = append(queries, q)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override token from flag if provided
	if token != "" {
		cfg.Token = token
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	// Create Slack client
	client := slack.NewClient(cfg.Token)
	directory := slack.NewDirectory(client)
	directory.IncludeArchivedChannels = resolveIncludeArchived

	var entries []slack.Entry
	for _, q := range queries {
		matches, err := resolveOne(directory, q)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] %s: %v\n", q.raw, err)
			continue
		}
		if len(matches) == 0 {
			fmt.Fprintf(os.Stderr, "[WARN] %s: not found\n", q.raw)
			continue
		}
		entries = append(entries, matches...)
	}

	if len(entries) == 0 {
		return fmt.Errorf("no queries could be resolved")
	}

	if resolveAsJSON {
		writer := output.NewStdoutWriter()
		return writer.Write(entries)
	}
	for _, e := range entries {
		fmt.Printf("%s\t%s\t%s\n", e.Type, e.ID, e.Name)
	}
	return nil
}

// resolveOne resolves a single classified query into matching entries.
func resolveOne(directory *slack.Directory, q resolveQuery) ([]slack.Entry, error) {
	switch q.kind {
	case kindEmail:
		entry, err := directory.LookupUserByEmail(q.value)
		if err != nil {
			return nil, err
		}
		return []slack.Entry{entry}, nil
	case kindID:
		var entry slack.Entry
		var err error
		switch q.types[0] {
		case slack.TypeUser:
			entry, err = directory.LookupUserByID(q.value)
		case slack.TypeChannel:
			entry, err = directory.LookupChannelByID(q.value)
		case slack.TypeUsergroup:
			entry, err = directory.LookupUsergroupByID(q.value)
		}
		if err != nil {
			return nil, err
		}
		return []slack.Entry{entry}, nil
	default: // kindName
		var entries []slack.Entry
		for _, t := range q.types {
			var matches []slack.Entry
			var err error
			switch t {
			case slack.TypeUser:
				matches, err = directory.FindUsersByName(q.value)
			case slack.TypeChannel:
				matches, err = directory.FindChannelsByName(q.value)
			case slack.TypeUsergroup:
				matches, err = directory.FindUsergroupsByName(q.value)
			}
			if err != nil {
				return nil, err
			}
			entries = append(entries, matches...)
		}
		return entries, nil
	}
}

// validateTypeFilter checks a --type flag value.
func validateTypeFilter(typeFilter string) error {
	switch typeFilter {
	case "", slack.TypeUser, slack.TypeChannel, slack.TypeUsergroup:
		return nil
	}
	return fmt.Errorf("invalid --type %q (must be user, channel, or usergroup)", typeFilter)
}

// classifyQuery classifies a raw resolve argument. typeFilter restricts which
// types are considered ("" means no restriction); a query whose explicit form
// (ID prefix, "#name", "@name", email) contradicts typeFilter is an error.
func classifyQuery(raw string, typeFilter string) (resolveQuery, error) {
	if raw == "" || raw == "#" || raw == "@" {
		return resolveQuery{}, fmt.Errorf("empty query")
	}

	// Explicit prefixes
	if name, ok := strings.CutPrefix(raw, "#"); ok {
		types, err := filterTypes([]string{slack.TypeChannel}, typeFilter, raw)
		if err != nil {
			return resolveQuery{}, err
		}
		return resolveQuery{raw: raw, kind: kindName, types: types, value: name}, nil
	}
	if name, ok := strings.CutPrefix(raw, "@"); ok {
		types, err := filterTypes([]string{slack.TypeUser, slack.TypeUsergroup}, typeFilter, raw)
		if err != nil {
			return resolveQuery{}, err
		}
		return resolveQuery{raw: raw, kind: kindName, types: types, value: name}, nil
	}

	// Email
	if strings.Contains(raw, "@") {
		if _, err := filterTypes([]string{slack.TypeUser}, typeFilter, raw); err != nil {
			return resolveQuery{}, err
		}
		return resolveQuery{raw: raw, kind: kindEmail, types: []string{slack.TypeUser}, value: raw}, nil
	}

	// IDs by prefix (only for types allowed by the filter, so that e.g.
	// --type user treats "C0123ABCD" as a user name, not a channel ID)
	if typeAllowed(slack.TypeUser, typeFilter) && slack.IsUserID(raw) {
		return resolveQuery{raw: raw, kind: kindID, types: []string{slack.TypeUser}, value: raw}, nil
	}
	if typeAllowed(slack.TypeChannel, typeFilter) && slack.IsChannelID(raw) {
		return resolveQuery{raw: raw, kind: kindID, types: []string{slack.TypeChannel}, value: raw}, nil
	}
	if typeAllowed(slack.TypeUsergroup, typeFilter) && slack.IsUsergroupID(raw) {
		return resolveQuery{raw: raw, kind: kindID, types: []string{slack.TypeUsergroup}, value: raw}, nil
	}

	// Bare name: search all allowed types
	types := []string{slack.TypeUser, slack.TypeChannel, slack.TypeUsergroup}
	if typeFilter != "" {
		types = []string{typeFilter}
	}
	return resolveQuery{raw: raw, kind: kindName, types: types, value: raw}, nil
}

// filterTypes intersects the types implied by a query's form with typeFilter.
// An empty intersection means the query contradicts --type.
func filterTypes(implied []string, typeFilter string, raw string) ([]string, error) {
	if typeFilter == "" {
		return implied, nil
	}
	for _, t := range implied {
		if t == typeFilter {
			return []string{typeFilter}, nil
		}
	}
	return nil, fmt.Errorf("query %q implies type %s, which contradicts --type %s",
		raw, strings.Join(implied, "/"), typeFilter)
}

// typeAllowed reports whether t passes typeFilter ("" allows all).
func typeAllowed(t, typeFilter string) bool {
	return typeFilter == "" || t == typeFilter
}
