package slack

import (
	"fmt"
	"os"
	"regexp"

	"github.com/slack-go/slack"
)

// Entry types for Directory lookups.
const (
	TypeUser      = "user"
	TypeChannel   = "channel"
	TypeUsergroup = "usergroup"
)

// Entry is one resolved ID/name pair.
type Entry struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

var (
	// userIDPattern matches Slack user IDs (e.g. U0123ABCD, W0123ABCD).
	userIDPattern = regexp.MustCompile(`^[UW][A-Z0-9]+$`)
	// usergroupIDPattern matches Slack usergroup (subteam) IDs (e.g. S0123ABCD).
	usergroupIDPattern = regexp.MustCompile(`^S[A-Z0-9]+$`)
)

// IsUserID reports whether s looks like a Slack user ID rather than a name.
func IsUserID(s string) bool {
	return userIDPattern.MatchString(s)
}

// IsUsergroupID reports whether s looks like a Slack usergroup ID rather than a name.
func IsUsergroupID(s string) bool {
	return usergroupIDPattern.MatchString(s)
}

// Directory provides bidirectional ID<->name lookups for users, channels, and
// usergroups. ID lookups hit the per-object info APIs directly; name lookups
// lazily build a full list per type (users.list / conversations.list /
// usergroups.list) on first use and reuse it afterwards.
type Directory struct {
	client *Client

	// IncludeArchivedChannels includes archived channels in channel name
	// lookups. Off by default: skipping them cuts the conversations.list
	// page count considerably on large workspaces. ID lookups always work
	// regardless (conversations.info covers archived channels).
	IncludeArchivedChannels bool

	users       []slack.User
	usersLoaded bool

	channels       []slack.Channel
	channelsLoaded bool

	usergroups       []slack.UserGroup
	usergroupsLoaded bool
}

// NewDirectory creates a new Directory.
func NewDirectory(client *Client) *Directory {
	return &Directory{client: client}
}

// LookupUserByID resolves a user ID via users.info.
func (d *Directory) LookupUserByID(id string) (Entry, error) {
	user, err := d.client.api.GetUserInfo(id)
	if err != nil {
		return Entry{}, fmt.Errorf("users.info API error: %w", err)
	}
	return userEntry(*user), nil
}

// LookupUserByEmail resolves an email address via users.lookupByEmail.
func (d *Directory) LookupUserByEmail(email string) (Entry, error) {
	user, err := d.client.api.GetUserByEmail(email)
	if err != nil {
		return Entry{}, fmt.Errorf("users.lookupByEmail API error: %w", err)
	}
	return userEntry(*user), nil
}

// LookupChannelByID resolves a channel ID via conversations.info.
func (d *Directory) LookupChannelByID(id string) (Entry, error) {
	channel, err := d.client.GetChannelInfo(id)
	if err != nil {
		return Entry{}, err
	}
	return channelEntry(*channel), nil
}

// LookupUsergroupByID resolves a usergroup ID via a cached usergroups.list.
func (d *Directory) LookupUsergroupByID(id string) (Entry, error) {
	if err := d.ensureUsergroups(); err != nil {
		return Entry{}, err
	}
	for _, g := range d.usergroups {
		if g.ID == id {
			return usergroupEntry(g), nil
		}
	}
	return Entry{}, fmt.Errorf("usergroup not found: %q", id)
}

// FindUsersByName returns all users whose username, display name, or real
// name exactly matches name.
func (d *Directory) FindUsersByName(name string) ([]Entry, error) {
	if err := d.ensureUsers(); err != nil {
		return nil, err
	}
	var entries []Entry
	for _, u := range d.users {
		if matchUser(u, name) {
			entries = append(entries, userEntry(u))
		}
	}
	return entries, nil
}

// FindChannelsByName returns all channels whose name exactly matches name.
func (d *Directory) FindChannelsByName(name string) ([]Entry, error) {
	if err := d.ensureChannels(); err != nil {
		return nil, err
	}
	var entries []Entry
	for _, ch := range d.channels {
		if matchChannel(ch, name) {
			entries = append(entries, channelEntry(ch))
		}
	}
	return entries, nil
}

// FindUsergroupsByName returns all usergroups whose handle or name exactly
// matches name.
func (d *Directory) FindUsergroupsByName(name string) ([]Entry, error) {
	if err := d.ensureUsergroups(); err != nil {
		return nil, err
	}
	var entries []Entry
	for _, g := range d.usergroups {
		if matchUsergroup(g, name) {
			entries = append(entries, usergroupEntry(g))
		}
	}
	return entries, nil
}

func (d *Directory) ensureUsers() error {
	if d.usersLoaded {
		return nil
	}
	fmt.Fprintln(os.Stderr, "Fetching user list (this may take a while on large workspaces)...")
	// GetUsers pages through users.list and retries on rate limits internally.
	users, err := d.client.api.GetUsers(slack.GetUsersOptionLimit(1000))
	if err != nil {
		return fmt.Errorf("users.list API error: %w", err)
	}
	d.users = make([]slack.User, 0, len(users))
	for _, u := range users {
		if u.Deleted {
			continue
		}
		d.users = append(d.users, u)
	}
	d.usersLoaded = true
	return nil
}

func (d *Directory) ensureChannels() error {
	if d.channelsLoaded {
		return nil
	}
	fmt.Fprintln(os.Stderr, "Fetching channel list...")
	channels, err := d.client.ListAllChannels(!d.IncludeArchivedChannels)
	if err != nil {
		return err
	}
	d.channels = channels
	d.channelsLoaded = true
	return nil
}

func (d *Directory) ensureUsergroups() error {
	if d.usergroupsLoaded {
		return nil
	}
	usergroups, err := d.client.api.GetUserGroups()
	if err != nil {
		return fmt.Errorf("usergroups.list API error: %w", err)
	}
	d.usergroups = usergroups
	d.usergroupsLoaded = true
	return nil
}

// userEntry converts a slack.User into an Entry, preferring the display name
// over the real name over the username (same priority as Resolver).
func userEntry(u slack.User) Entry {
	name := u.Name
	switch {
	case u.Profile.DisplayName != "":
		name = u.Profile.DisplayName
	case u.Profile.RealName != "":
		name = u.Profile.RealName
	}
	return Entry{Type: TypeUser, ID: u.ID, Name: name}
}

// channelEntry converts a slack.Channel into an Entry.
func channelEntry(ch slack.Channel) Entry {
	return Entry{Type: TypeChannel, ID: ch.ID, Name: ch.Name}
}

// usergroupEntry converts a slack.UserGroup into an Entry, preferring the
// mention handle over the descriptive name.
func usergroupEntry(g slack.UserGroup) Entry {
	name := g.Handle
	if name == "" {
		name = g.Name
	}
	return Entry{Type: TypeUsergroup, ID: g.ID, Name: name}
}

// matchUser reports whether name exactly matches the user's username,
// display name, or real name.
func matchUser(u slack.User, name string) bool {
	return u.Name == name || u.Profile.DisplayName == name || u.Profile.RealName == name
}

// matchChannel reports whether name exactly matches the channel name.
func matchChannel(ch slack.Channel, name string) bool {
	return ch.Name == name
}

// matchUsergroup reports whether name exactly matches the usergroup's handle
// or descriptive name.
func matchUsergroup(g slack.UserGroup, name string) bool {
	return g.Handle == name || g.Name == name
}
