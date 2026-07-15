package slack

import (
	"testing"

	"github.com/slack-go/slack"
)

func testUser(id, username, displayName, realName string) slack.User {
	u := slack.User{ID: id, Name: username}
	u.Profile.DisplayName = displayName
	u.Profile.RealName = realName
	return u
}

func TestIsUserID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "user ID", in: "U0123ABCD", want: true},
		{name: "enterprise user ID", in: "W0123ABCD", want: true},
		{name: "channel ID", in: "C0123ABCD", want: false},
		{name: "usergroup ID", in: "S0123ABCD", want: false},
		{name: "lowercase name", in: "john.doe", want: false},
		{name: "empty", in: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsUserID(tt.in); got != tt.want {
				t.Errorf("IsUserID(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsUsergroupID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "usergroup ID", in: "S0123ABCD", want: true},
		{name: "user ID", in: "U0123ABCD", want: false},
		{name: "lowercase name", in: "backend", want: false},
		{name: "empty", in: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsUsergroupID(tt.in); got != tt.want {
				t.Errorf("IsUsergroupID(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestUserEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		user slack.User
		want Entry
	}{
		{
			name: "display name preferred",
			user: testUser("U123", "alice", "Alice D.", "Alice Doe"),
			want: Entry{Type: TypeUser, ID: "U123", Name: "Alice D."},
		},
		{
			name: "real name when display name empty",
			user: testUser("U123", "alice", "", "Alice Doe"),
			want: Entry{Type: TypeUser, ID: "U123", Name: "Alice Doe"},
		},
		{
			name: "username as last resort",
			user: testUser("U123", "alice", "", ""),
			want: Entry{Type: TypeUser, ID: "U123", Name: "alice"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := userEntry(tt.user); got != tt.want {
				t.Errorf("userEntry() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestUsergroupEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		group slack.UserGroup
		want  Entry
	}{
		{
			name:  "handle preferred",
			group: slack.UserGroup{ID: "S123", Handle: "backend", Name: "Backend Team"},
			want:  Entry{Type: TypeUsergroup, ID: "S123", Name: "backend"},
		},
		{
			name:  "name when handle empty",
			group: slack.UserGroup{ID: "S123", Name: "Backend Team"},
			want:  Entry{Type: TypeUsergroup, ID: "S123", Name: "Backend Team"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := usergroupEntry(tt.group); got != tt.want {
				t.Errorf("usergroupEntry() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMatchUser(t *testing.T) {
	t.Parallel()

	user := testUser("U123", "alice", "Alice D.", "Alice Doe")

	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{name: "matches username", query: "alice", want: true},
		{name: "matches display name", query: "Alice D.", want: true},
		{name: "matches real name", query: "Alice Doe", want: true},
		{name: "partial does not match", query: "Alice", want: false},
		{name: "case sensitive", query: "ALICE", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := matchUser(user, tt.query); got != tt.want {
				t.Errorf("matchUser(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestMatchUsergroup(t *testing.T) {
	t.Parallel()

	group := slack.UserGroup{ID: "S123", Handle: "backend", Name: "Backend Team"}

	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{name: "matches handle", query: "backend", want: true},
		{name: "matches name", query: "Backend Team", want: true},
		{name: "partial does not match", query: "back", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := matchUsergroup(group, tt.query); got != tt.want {
				t.Errorf("matchUsergroup(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

// TestDirectoryFindWithPreloadedLists exercises the Find* methods against
// pre-populated lists so that no Slack API call is ever made.
func TestDirectoryFindWithPreloadedLists(t *testing.T) {
	t.Parallel()

	d := &Directory{
		users: []slack.User{
			testUser("U123", "alice", "Alice D.", "Alice Doe"),
			testUser("U456", "bob", "", "Bob Roe"),
		},
		usersLoaded: true,
		channels: []slack.Channel{
			func() slack.Channel {
				var ch slack.Channel
				ch.ID = "C123"
				ch.Name = "general"
				return ch
			}(),
		},
		channelsLoaded: true,
		usergroups: []slack.UserGroup{
			{ID: "S123", Handle: "backend", Name: "Backend Team"},
		},
		usergroupsLoaded: true,
	}

	t.Run("user found", func(t *testing.T) {
		t.Parallel()
		got, err := d.FindUsersByName("alice")
		if err != nil {
			t.Fatalf("FindUsersByName() error = %v", err)
		}
		want := []Entry{{Type: TypeUser, ID: "U123", Name: "Alice D."}}
		if len(got) != 1 || got[0] != want[0] {
			t.Errorf("FindUsersByName() = %+v, want %+v", got, want)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		t.Parallel()
		got, err := d.FindUsersByName("carol")
		if err != nil {
			t.Fatalf("FindUsersByName() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("FindUsersByName() = %+v, want empty", got)
		}
	})

	t.Run("channel found", func(t *testing.T) {
		t.Parallel()
		got, err := d.FindChannelsByName("general")
		if err != nil {
			t.Fatalf("FindChannelsByName() error = %v", err)
		}
		want := Entry{Type: TypeChannel, ID: "C123", Name: "general"}
		if len(got) != 1 || got[0] != want {
			t.Errorf("FindChannelsByName() = %+v, want %+v", got, want)
		}
	})

	t.Run("usergroup found by handle", func(t *testing.T) {
		t.Parallel()
		got, err := d.FindUsergroupsByName("backend")
		if err != nil {
			t.Fatalf("FindUsergroupsByName() error = %v", err)
		}
		want := Entry{Type: TypeUsergroup, ID: "S123", Name: "backend"}
		if len(got) != 1 || got[0] != want {
			t.Errorf("FindUsergroupsByName() = %+v, want %+v", got, want)
		}
	})

	t.Run("usergroup lookup by ID", func(t *testing.T) {
		t.Parallel()
		got, err := d.LookupUsergroupByID("S123")
		if err != nil {
			t.Fatalf("LookupUsergroupByID() error = %v", err)
		}
		want := Entry{Type: TypeUsergroup, ID: "S123", Name: "backend"}
		if got != want {
			t.Errorf("LookupUsergroupByID() = %+v, want %+v", got, want)
		}
	})

	t.Run("usergroup lookup by unknown ID", func(t *testing.T) {
		t.Parallel()
		if _, err := d.LookupUsergroupByID("S999"); err == nil {
			t.Error("LookupUsergroupByID() error = nil, want error")
		}
	})
}
