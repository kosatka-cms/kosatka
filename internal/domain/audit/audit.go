// Package audit - the actions done on the site that an action log keeps
// (who deleted content, changed a role's permissions, signed in): the
// core and the modules Record them, a listener (the action log module,
// OnAction) stores them. Without a listener recording costs nothing.
// Who acted comes from the context (WithActor - the HTTP layer puts the
// client's address and the signed-in user there).
package audit

import (
	"context"
	"slices"
	"sync"
	"time"
)

// Actor - who acted: the signed-in user (0 - none: a guest, or the site
// itself, a background job) and the client's address ("" - not over
// HTTP).
type Actor struct {
	UserID int
	IP     string
}

type actorKey struct{}

// WithActor - ctx carrying a as who acts.
func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, a)
}

// WithUser - ctx with the signed-in user as who acts, the address kept.
func WithUser(ctx context.Context, userID int) context.Context {
	a := ActorFrom(ctx)
	a.UserID = userID
	return WithActor(ctx, a)
}

// ActorFrom - who acts in ctx (the zero Actor - nobody known).
func ActorFrom(ctx context.Context) Actor {
	if ctx == nil {
		return Actor{}
	}
	a, _ := ctx.Value(actorKey{}).(Actor)
	return a
}

// Object - what an action was done to: Kind (node, comment, user, role,
// module, settings or a module's own), its ID, its name at the time
// (Label; an action log keeps it for an object deleted since), what kind
// of it (Subtype - a node's content type, by machine name; "" - none)
// and where it is (URL, "" - nowhere: a deleted one).
type Object struct {
	Kind    string
	ID      string
	Label   string
	Subtype string
	URL     string
}

// Change - one field an action changed: its name and its values before
// and after, as shown to people.
type Change struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// Action - something done on the site: its Type (a registered one,
// RegisterType), what it was done to, what it changed (optional).
type Action struct {
	Type    string
	Object  Object
	Changes []Change
}

// Event - a recorded action: the action, who did it and when.
type Event struct {
	Action
	Actor Actor
	At    time.Time
}

// Group - a kind of actions an action log shows and turns on or off
// together (content, users and permissions, a module's): Name, the module
// it belongs to ("" - the core's), Label and Description (i18n.N).
type Group struct {
	Name        string
	Module      string
	Label       string
	Description string
}

// Type - an action of a group: Name ("node.delete"), Group, Label (i18n.N:
// "Deleted") and Tone - what kind of change it is, for its look in a log:
// ToneCreate, ToneChange, ToneDelete, ToneAccess, ToneLogin, ToneFail.
type Type struct {
	Name  string
	Group string
	Label string
	Tone  string
}

// The tones of action types (Type.Tone).
const (
	ToneCreate = "create"
	ToneChange = "change"
	ToneDelete = "delete"
	ToneAccess = "access"
	ToneLogin  = "login"
	ToneFail   = "fail"
)

var (
	mu        sync.RWMutex
	groups    []Group
	types     = map[string]Type{}
	listeners []func(ctx context.Context, e Event)
)

// RegisterGroup adds a group of actions (one of the same name replaces
// it).
func RegisterGroup(g Group) {
	mu.Lock()
	defer mu.Unlock()
	for i := range groups {
		if groups[i].Name == g.Name {
			groups[i] = g
			return
		}
	}
	groups = append(groups, g)
}

// RegisterType adds an action type.
func RegisterType(t Type) {
	mu.Lock()
	defer mu.Unlock()
	types[t.Name] = t
}

// Groups - the groups, in registration order (the core's first).
func Groups() []Group {
	mu.RLock()
	defer mu.RUnlock()
	return slices.Clone(groups)
}

// TypeOf - the action type of name (false - not registered).
func TypeOf(name string) (Type, bool) {
	mu.RLock()
	defer mu.RUnlock()
	t, ok := types[name]
	return t, ok
}

// OnAction adds a listener of the recorded actions (an action log). It
// is called in the acting request: it should be quick, its errors its
// own to log.
func OnAction(fn func(ctx context.Context, e Event)) {
	mu.Lock()
	defer mu.Unlock()
	listeners = append(listeners, fn)
}

// ResetListeners forgets the listeners (an application built again - the
// tests build it more than once).
func ResetListeners() {
	mu.Lock()
	defer mu.Unlock()
	listeners = nil
}

// Record tells the listeners of a: who did it (ctx's actor) and now.
func Record(ctx context.Context, a Action) {
	mu.RLock()
	ls := slices.Clone(listeners)
	mu.RUnlock()
	if len(ls) == 0 {
		return
	}
	e := Event{Action: a, Actor: ActorFrom(ctx), At: time.Now()}
	for _, fn := range ls {
		fn(ctx, e)
	}
}
