# AGENTS.md

This file provides guidance to AI coding agents (Claude Code, etc.) when working with code in this repository.

## Project Overview

`golack` is a Slack log collector CLI built with Cobra: it fetches Slack messages — a single message or thread by URL (`get`), a filtered search over a date range (`list`), or the complete history of given channels (`history`) — transforms them into a simplified thread-grouped JSON format (not the raw Slack API shape), and can merge and deduplicate previously collected JSON files (`merge`). `resolve` is an input-discovery helper: it converts between the IDs and names (users, channels, usergroups) that the other commands and the config take as input. Configuration (`token`, `author`, `mention`) is resolved with Viper from `$XDG_CONFIG_HOME/golack/config.toml` (fallback `~/.config/golack/config.toml`, override with `--config`), environment variables (`SLACK_API_TOKEN`, `SLACK_AUTHOR`, `SLACK_MENTION`), and CLI flags — in increasing order of precedence. Config file values may reference environment variables with `${VAR}`, expanded at load time.

## Build Commands

```sh
make build   # Build binary to ./bin/golack
make test    # Run tests (go test ./...)
make fmt     # Format code
make vet     # Vet code
make tidy    # Tidy dependencies
make clean   # Remove build artifacts
```

Binary name is read from `.product_name`.

## Release

```sh
make release type=patch|minor|major            # dry run (default)
make release type=patch dryrun=false           # create and push tag
make re-release [tag=vX.Y.Z] dryrun=false      # re-release an existing tag
```

Pushing a `v*` tag triggers `.github/workflows/gorelease.yml`, which builds multi-platform binaries with GoReleaser (version/commit/build time injected into `internal/version` via ldflags) and uploads them to GitHub Releases.

## Architecture

- `main.go` — entry point, calls `cmd.Execute()`
- `cmd/` — Cobra commands
  - `root.go` — root command and global flags (`--token`, `--config`, `--resolve-ids`); `--config` is registered with the config package in `PersistentPreRun`
  - `get.go` — `golack get <url>`: fetch a single message, or the whole thread with `--thread` (or automatically when the URL carries `thread_ts=`)
  - `list.go` — `golack list`: date-range search. `parseDateRange` validates the mutually exclusive `--day` / `--month` / `--from`+`--to` options (shared with `history`). Author/mention filters default from config; `--no-author` / `--no-mention` explicitly disable them. `processdays` runs one `collector.List` per day through a worker pool (`--parallel`)
  - `history.go` — `golack history`: full channel history for a date range; `--channel` is required (names or IDs, resolved once up front via `ResolveChannels`), same date-range and worker-pool handling as `list`
  - `merge.go` — `golack merge [dir]`: find JSON files (`--pattern`, `--recursive`), read them, merge via `collector.Merge`, print to stdout
  - `resolve.go` — `golack resolve <query>...`: bidirectional ID<->name conversion for users/channels/usergroups. `classifyQuery` classifies each argument (ID by prefix, `#name`, `@name`, email, bare name) and intersects it with `--type`; lookups run through `slack.Directory`. Output is TSV lines (`type\tid\tname`) or a JSON array with `--json`; unresolved queries are stderr warnings, and the command fails only when nothing resolved
  - `config.go` — `golack config get <key>` / `golack config list`: show the effective configuration (valid keys: `token`, `author`, `mention`)
  - `output.go` — shared output helpers: `newOutputWriter` (stdout or `--output` file), `sortThreads` (by first message timestamp), `writeThreads` (progress to stderr, JSON to the writer)
  - `version.go` — `golack version`
- `internal/config/` — Viper-based loading (`Load`): TOML file (explicit path via `SetConfigFile`, else `defaultConfigPaths` = `$XDG_CONFIG_HOME/golack` then `~/.config/golack`), env bindings, `${VAR}` expansion via `os.ExpandEnv`. A missing file is only an error when `--config` was given explicitly. `Validate` requires a non-empty token
- `internal/dateutil/` — `ParseDay` (YYYY-MM-DD), `ParseMonth` (YYYY-MM, expands to first–last day), `CustomRange` (validates from ≤ to), `DateRange.Days()` enumeration, `FormatDate`
- `internal/model/` — output types: `Message`, `Thread`, `SearchResult` (JSON tags define the output format)
- `internal/slack/` — wrapper around `github.com/slack-go/slack`
  - `client.go` — `Client` wrapping the API client
  - `url.go` — `ParseURL` extracts channel ID / message TS / thread TS from a Slack archive URL; `normalizeTimestamp` converts `p1716192523567890` to `1716192523.567890`
  - `search.go` — `SearchMessages` (search.messages): `buildSearchQueries` emits one query per mention (Slack search has no OR for `to:`, so mentions are OR-merged and deduplicated), `buildBaseQueryParts` adds `from:` / `in:` / `-in:` / `after:` / `before:` and always excludes DMs (`-is:dm -is:mpdm`); thread hits are expanded via `GetThreadReplies`; also message conversion and mention/link extraction from search matches
  - `thread.go` — `GetThreadReplies` / `GetThread` (conversations.replies, paged), `convertReplyMessage` and mention/link extraction from raw messages
  - `history.go` — `GetChannelHistory` (conversations.history, paged, inclusive unix bounds), `ListAllChannels` (conversations.list, optional archived-channel exclusion), `ResolveChannels` / `IsChannelID` (channel names vs `C…` IDs)
  - `channel.go` — `GetChannelInfo` / `GetChannelName` (falls back to the ID on error)
  - `resolver.go` — `Resolver` rewrites Slack mrkdwn tokens (`<@U…>`, `<#C…>`, `<!subteam^…>`, `<!here|channel|everyone>`) in message content to human-readable names, with mutex-protected per-type caches; used by `--resolve-ids`
  - `directory.go` — `Directory` for bidirectional ID<->name lookups (used by `resolve`): ID lookups hit per-object info APIs (users.info / conversations.info), name lookups lazily build a full per-type list (users.list / conversations.list / usergroups.list) and match exactly; channel name lookups exclude archived channels unless `IncludeArchivedChannels` is set; also `Entry`, `IsUserID` / `IsUsergroupID`, and email lookup via users.lookupByEmail
- `internal/collector/` — orchestration between the slack client and the model
  - `get.go` — `Get`: parse URL, fetch single message or whole thread, wrap in a `Thread`
  - `list.go` — `List`: search one day (searching `after:` day-1 / `before:` day+1), optional `fetchThreads` expansion, `groupByThread` (groups by `ThreadTS`, falls back to message ID; sorts messages within threads and threads by first message timestamp)
  - `history.go` — `History`: full-day channel history; day boundaries are interpreted in the **local** timezone
  - `merge.go` — `Merge`: dedupe threads by `ThreadID` (messages appended), dedupe messages by ID keeping the latest timestamp, sort, and report original/merged/duplicate counts
- `internal/input/` — `FileReader.ReadFile` (JSON `[]Thread`), `FindFiles` (glob pattern match on file names, optional recursive walk)
- `internal/output/` — `Writer` interface, `JSONWriter` (optional indent), `StdoutWriter`, `FileWriter` (creates parent directories, indented JSON)
- `internal/version/` — version info injected via ldflags at build time

## Key behavior

- Precedence is CLI flag > environment variable > config file. `SLACK_MENTION` is documented as comma-separated; TOML `mention` is an array. All values go through `os.ExpandEnv`
- Every Slack API call retries up to 5 times with exponential backoff on `RateLimitedError` (honoring `Retry-After` when present) and sleeps 1s between result pages
- `list` filters via search.messages, so `--mention` cannot match `@channel`/`@here` broadcasts; `history` collects everything in a channel and exists for exactly that case
- Progress and warnings are written to stderr; stdout carries only the JSON document, so output is always pipeable (`--output FILE` writes to a file instead)
- Failed thread fetches during expansion are warnings, not fatal errors: the parent message is kept and collection continues
- `--resolve-ids` post-processes collected threads through the `Resolver` (requires `users:read` / `usergroups:read` scopes); IDs that cannot be resolved fall back to the raw ID

## Testing

```sh
make test    # or: go test ./...
```

Tests use only the standard library `testing` package, are table-driven with `t.Run` subtests, and live beside the code under test. Pure logic is covered — URL/timestamp parsing, date ranges, search query building, mention/link extraction, resolver content rewriting (with pre-populated caches), merge/dedup/grouping, config loading (with `t.TempDir()` and `t.Setenv` for isolation), file discovery, and JSON writers. Nothing calls the live Slack API.
