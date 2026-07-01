package slack

import (
	"regexp"
	"sync"

	"github.com/longkey1/gosla/internal/model"
)

// slackRefRe matches Slack mrkdwn reference tokens:
//
//	group 1,2: <@USERID|name>
//	group 3,4: <#CHANNELID|name>
//	group 5,6: <!subteam^ID|label>
//	group 7:   <!here|channel|everyone>
var slackRefRe = regexp.MustCompile(
	`<@([A-Z0-9]+)(?:\|([^>]+))?>` +
		`|<#([A-Z0-9]+)(?:\|([^>]+))?>` +
		`|<!subteam\^([A-Z0-9]+)(?:\|([^>]+))?>` +
		`|<!(here|channel|everyone)>`,
)

// Resolver resolves Slack IDs in message content to human-readable names.
type Resolver struct {
	client         *Client
	mu             sync.RWMutex
	userCache      map[string]string
	channelCache   map[string]string
	usergroupCache map[string]string
}

// NewResolver creates a new Resolver.
func NewResolver(client *Client) *Resolver {
	return &Resolver{
		client:         client,
		userCache:      make(map[string]string),
		channelCache:   make(map[string]string),
		usergroupCache: make(map[string]string),
	}
}

// ResolveThreads resolves IDs in all messages of the given threads.
func (r *Resolver) ResolveThreads(threads []model.Thread) []model.Thread {
	resolved := make([]model.Thread, len(threads))
	for i, t := range threads {
		msgs := make([]model.Message, len(t.Messages))
		for j, msg := range t.Messages {
			msgs[j] = r.resolveMessage(msg)
		}
		t.Messages = msgs
		resolved[i] = t
	}
	return resolved
}

func (r *Resolver) resolveMessage(msg model.Message) model.Message {
	msg.Content = r.resolveContent(msg.Content)
	if msg.Author != "" {
		msg.Author = r.lookupUser(msg.Author)
	}
	return msg
}

func (r *Resolver) resolveContent(text string) string {
	return slackRefRe.ReplaceAllStringFunc(text, func(match string) string {
		sub := slackRefRe.FindStringSubmatch(match)
		switch {
		case sub[1] != "": // <@USERID> or <@USERID|name>
			if sub[2] != "" {
				return "@" + sub[2]
			}
			return "@" + r.lookupUser(sub[1])
		case sub[3] != "": // <#CHANNELID> or <#CHANNELID|name>
			if sub[4] != "" {
				return "#" + sub[4]
			}
			return "#" + r.lookupChannel(sub[3])
		case sub[5] != "": // <!subteam^ID> or <!subteam^ID|label>
			if sub[6] != "" {
				return sub[6]
			}
			return r.lookupUsergroup(sub[5])
		case sub[7] != "": // <!here>, <!channel>, <!everyone>
			return "@" + sub[7]
		}
		return match
	})
}

func (r *Resolver) lookupUser(userID string) string {
	r.mu.RLock()
	if name, ok := r.userCache[userID]; ok {
		r.mu.RUnlock()
		return name
	}
	r.mu.RUnlock()

	name := userID
	if user, err := r.client.api.GetUserInfo(userID); err == nil {
		switch {
		case user.Profile.DisplayName != "":
			name = user.Profile.DisplayName
		case user.Profile.RealName != "":
			name = user.Profile.RealName
		case user.Name != "":
			name = user.Name
		}
	}

	r.mu.Lock()
	r.userCache[userID] = name
	r.mu.Unlock()
	return name
}

func (r *Resolver) lookupUsergroup(usergroupID string) string {
	r.mu.RLock()
	if name, ok := r.usergroupCache[usergroupID]; ok {
		r.mu.RUnlock()
		return name
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	// double-check after acquiring write lock
	if name, ok := r.usergroupCache[usergroupID]; ok {
		return name
	}

	if groups, err := r.client.api.GetUserGroups(); err == nil {
		for _, g := range groups {
			name := g.Handle
			if name == "" {
				name = g.Name
			}
			r.usergroupCache[g.ID] = "@" + name
		}
	}

	if name, ok := r.usergroupCache[usergroupID]; ok {
		return name
	}
	return usergroupID
}

func (r *Resolver) lookupChannel(channelID string) string {
	r.mu.RLock()
	if name, ok := r.channelCache[channelID]; ok {
		r.mu.RUnlock()
		return name
	}
	r.mu.RUnlock()

	name := r.client.GetChannelName(channelID)

	r.mu.Lock()
	r.channelCache[channelID] = name
	r.mu.Unlock()
	return name
}
