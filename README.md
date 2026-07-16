# golack

Slack Log Collector CLI - A command-line tool for collecting Slack messages

## Installation

### Download from Releases

Download the binary from [Releases](https://github.com/longkey1/golack/releases).

### Build from Source

```bash
git clone https://github.com/longkey1/golack.git
cd golack
make build
```

## Usage

### Configuration

golack can be configured via a config file, environment variables, or CLI flags.
Precedence is: **CLI flag > environment variable > config file**.

#### Config File

The default location is `$XDG_CONFIG_HOME/golack/config.toml` (falling back to
`~/.config/golack/config.toml`). A different path can be supplied via `--config`.

```toml
# ~/.config/golack/config.toml
token   = "xoxp-..."
author  = "your-username"
mention = ["U12345678", "@john.doe", "@team-name"]
```

Values may reference environment variables with `${VAR}`; they are expanded at
load time. This lets you keep secrets out of the file:

```toml
token   = "${SLACK_API_TOKEN}"
author  = "${SLACK_AUTHOR}"
mention = ["${SLACK_MENTION_USER}", "@team-name"]
```

#### Environment Variables

```bash
export SLACK_API_TOKEN="xoxp-..."  # Required
export SLACK_AUTHOR="your-username"  # Optional
export SLACK_MENTION="U12345678,@john.doe,@team-name"  # Optional: comma-separated
```

### Commands

#### get

Fetch a single message or thread from a Slack URL.

```bash
# Fetch a single message
golack get "https://xxx.slack.com/archives/C123/p456"

# Fetch the entire thread
golack get "https://xxx.slack.com/archives/C123/p456" --thread

# Resolve user/channel IDs to human-readable names
golack get "https://xxx.slack.com/archives/C123/p456" --resolve-ids
```

#### list

Collect messages for a date range. The result is written to stdout as a single
JSON document, or to a file with `--output`.

```bash
# Collect messages for a specific day (printed to stdout)
golack list --day 2025-01-15

# Collect messages for an entire month
golack list --month 2025-01

# Collect messages for a custom date range
golack list --from 2025-01-01 --to 2025-01-15

# Write to a file instead of stdout
golack list --month 2025-01 --output 2025-01.json

# Combine options
golack list -m 2025-01 --thread --author U12345678

# Filter by mention (multiple mentions are OR-matched: messages to ANY of them)
golack list -d 2025-01-15 --mention U111 --mention @john.doe --mention @team

# Disable the author/mention defaults from the config file for this run
golack list -d 2025-01-15 --no-mention
golack list -d 2025-01-15 --no-author --no-mention

# Filter by channels
golack list -m 2025-01 --channel general --channel random
golack list -d 2025-01-15 --exclude-channel announcements
golack list -m 2025-01 --channel general,random --exclude-channel bot-logs

# Parallel execution, piped to jq
golack list -m 2025-01 --parallel 4 | jq '.[].channel'

# Resolve user/channel IDs to human-readable names
golack list -d 2025-01-15 --resolve-ids
```

Output is a single JSON array of threads, written to stdout by default (progress
is written to stderr, so it never mixes with the JSON). Use `--output FILE` to
write to a file instead.

> **Note:** `--mention` filters via Slack's `search.messages` API, which cannot
> match `@channel`/`@here` broadcasts. Use the `history` command to collect those.

#### history

Collect **all** messages in the given channels for a date range. Unlike `list`,
no author/mention filtering is applied, so `@channel`/`@here` broadcasts and
every other message in the channel are included.

```bash
# Collect a channel's full history for a day (by name)
golack history --day 2025-01-15 --channel general

# Multiple channels, by name or ID (names resolved via conversations.list)
golack history -m 2025-01 --channel general,random
golack history -d 2025-01-15 --channel C0123ABCD

# Expand threads and run in parallel
golack history -m 2025-01 --channel general --thread --parallel 4

# Write to a file instead of stdout
golack history -m 2025-01 --channel general --output history.json

# Resolve user/channel IDs to human-readable names
golack history -d 2025-01-15 --channel general --resolve-ids
```

`--channel` and a date range are both required. Like `list`, the result is a
single JSON array of threads written to stdout by default; use `--output FILE`
to write to a file.

#### merge

Merge multiple JSON files and deduplicate threads/messages.

```bash
# Merge all JSON files in a directory
golack merge ./logs

# With explicit --dir flag
golack merge --dir ./logs

# Filter by file pattern
golack merge ./logs --pattern "slack*.json"
golack merge ./logs -p "2025-*.json"

# Recursive search (include subdirectories)
golack merge ./logs --recursive
golack merge ./logs -r -p "*.json"
```

Output is written to stdout.

#### resolve

Convert between Slack IDs and names for users, channels, and usergroups —
useful for finding the values that other commands and the config take as input
(`--channel`, `--author`, `mention`).

```bash
# ID -> name (type detected from the ID prefix: U/W = user, C/G/D = channel, S = usergroup)
golack resolve U0123ABCD
golack resolve C0123ABCD S0123ABCD

# name -> ID ("#" searches channels, "@" searches users and usergroups)
golack resolve "#general"
golack resolve "@john.doe"

# Look up a user by email (users.lookupByEmail)
golack resolve john.doe@example.com

# A bare name searches all types; restrict with --type
golack resolve general
golack resolve --type user john.doe

# JSON output
golack resolve --json "#general"
```

Each match is printed as a TSV line (`type<TAB>id<TAB>name`), or as a JSON
array with `--json`. Name matching is exact (username, display name, or real
name for users; channel name; usergroup handle or name). Queries that cannot
be resolved are reported on stderr; the command fails only when nothing could
be resolved at all.

Note: name -> ID lookups list the whole workspace directory for the queried
type (`users.list` / `conversations.list`), which can take a while on large
workspaces. Channel name lookups skip archived channels by default to keep
the list small; pass `--include-archived` to search them too (slower).
ID -> name lookups use the direct info APIs and are fast (archived channels
included).

#### config

Show the effective configuration values, resolved from the config file,
environment variables, and flags (in increasing order of precedence) — the same
resolution the other commands use.

```bash
# List all configuration values as key=value lines
golack config list

# Get a single configuration value
golack config get token
golack config get author
golack config get mention
```

Valid keys are `token`, `author`, and `mention`.

#### version

```bash
golack version
```

### Global Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--token` | Slack API token | `$SLACK_API_TOKEN` |
| `--config` | Path to config file | `$XDG_CONFIG_HOME/golack/config.toml` |
| `--resolve-ids` | Resolve Slack user/channel IDs in message content to human-readable names | `false` |

### get Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--thread` | Fetch the entire thread | `false` |

### list Flags

**Date Range (mutually exclusive):**

| Flag | Short | Description |
|------|-------|-------------|
| `--day` | `-d` | Single day (YYYY-MM-DD) |
| `--month` | `-m` | Entire month (YYYY-MM) |
| `--from` | | Start date (YYYY-MM-DD, inclusive) |
| `--to` | | End date (YYYY-MM-DD, inclusive) |

**Other Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--thread` | | Fetch entire threads | `false` |
| `--author` | | Filter by author | `$SLACK_AUTHOR` |
| `--mention` | | Filter by mention (User ID or `@username`/`@group-name`, repeatable; OR matched) | `$SLACK_MENTION` |
| `--no-author` | | Disable the author filter even if set in config | `false` |
| `--no-mention` | | Disable the mention filter even if set in config | `false` |
| `--channel` | | Filter by channel name (repeatable, comma-separated) | |
| `--exclude-channel` | | Exclude channel name (repeatable, comma-separated) | |
| `--parallel` | `-p` | Number of parallel workers | `1` |
| `--output` | `-o` | Write JSON to this file (default: stdout) | |

### history Flags

**Date Range (mutually exclusive, one required):**

| Flag | Short | Description |
|------|-------|-------------|
| `--day` | `-d` | Single day (YYYY-MM-DD) |
| `--month` | `-m` | Entire month (YYYY-MM) |
| `--from` | | Start date (YYYY-MM-DD, inclusive) |
| `--to` | | End date (YYYY-MM-DD, inclusive) |

**Other Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--channel` | | Channels to collect, by name or ID (repeatable, comma-separated); **required** | |
| `--thread` | | Fetch entire threads | `false` |
| `--parallel` | `-p` | Number of parallel workers | `1` |
| `--output` | `-o` | Write JSON to this file (default: stdout) | |

### merge Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--dir` | `-d` | Target directory | |
| `--pattern` | `-p` | File name glob pattern | `*.json` |
| `--recursive` | `-r` | Search subdirectories recursively | `false` |

### resolve Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--type` | Restrict lookup to one type: `user`, `channel`, or `usergroup` | |
| `--json` | Output as a JSON array instead of TSV lines | `false` |
| `--include-archived` | Include archived channels in channel name lookups (slower) | `false` |

## Required Permissions

The Slack API token requires the following scopes:

- `search:read` - Search messages
- `channels:history` - Read channel history
- `channels:read` - Read channel information
- `groups:history` - Read private channel history (optional)
- `groups:read` - Read private channel information (optional)
- `users:read` - Resolve user IDs to display names (required for `--resolve-ids` and `resolve`)
- `users:read.email` - Look up users by email (required for `resolve` with an email query)
- `usergroups:read` - Resolve user group IDs to display names (required for `--resolve-ids` and `resolve`)

## Output Format

Output is in JSON format, grouped by thread.

**Note:** The output structure is not the raw Slack API response. Messages are transformed into a simplified, consistent format:

| golack field | Source |
|-------------|--------|
| `id` | Message timestamp (`ts`) |
| `content` | Message text |
| `author` | User ID |
| `timestamp` | Parsed to ISO 8601 format |
| `mentions` | Extracted from `<@USER\|name>` patterns in text |
| `attached_links` | Extracted from text and attachments |
| `is_thread_parent` | Calculated from `thread_ts` |

```json
[
  {
    "thread_id": "1716192523.567890",
    "thread_permalink": "https://xxx.slack.com/archives/C123/p456",
    "channel": "general",
    "channel_id": "C12345678",
    "messages": [
      {
        "id": "1716192523.567890",
        "type": "slack_message",
        "content": "Hello, World!",
        "author": "U12345678",
        "timestamp": "2025-01-15T10:30:00Z",
        "channel": "general",
        "channel_id": "C12345678",
        "thread_ts": "1716192523.567890",
        "is_thread_parent": true
      }
    ],
    "message_count": 1
  }
]
```

## Development

```bash
# Build
make build

# Test
make test

# Format code
make fmt

# Vet code
make vet

# Lint (golangci-lint, version managed by go.mod tool directive)
make lint

# Tidy dependencies
make tidy

# Clean
make clean
```

### Release

```bash
# Dry run (default): shows what would happen
make release type=patch|minor|major

# Create and push a new tag
make release type=patch dryrun=false

# Re-release an existing tag
make re-release [tag=vX.Y.Z] dryrun=false
```

Pushing a `v*` tag triggers the GitHub Actions release workflow, which builds
multi-platform binaries with GoReleaser and uploads them to GitHub Releases.

## License

MIT
