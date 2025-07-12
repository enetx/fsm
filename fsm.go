// Package fsm provides a generic finite state machine (FSM) implementation
// with support for transitions, guards, and enter/exit callbacks. It is built
// with types and utilities from the github.com/enetx/g library.
package fsm

import (
	"fmt"
	"sync/atomic"

	. "github.com/enetx/g"
	"github.com/enetx/g/cmp"
)

type (
	// State represents a finite state in the FSM.
	State String
	// Event represents an event that triggers a transition.
	Event String
)

type (
	// Callback is a function called on entering or exiting a state.
	Callback func(ctx *Context) error
	// GuardFunc determines whether a transition is allowed.
	GuardFunc func(ctx *Context) bool
	// TransitionHook is a global callback called after a transition between states.
	// It runs after OnExit and before OnEnter.
	TransitionHook func(from, to State, event Event, ctx *Context) error
)

type transition struct {
	event Event
	to    State
	guard GuardFunc
}

// Context holds FSM state, input, persistent and temporary data
// Data is for long-lived values (e.g. "name"),
// Values is for ephemeral metadata (e.g. timestamps, counters)
type Context struct {
	State  State
	Data   *MapSafe[String, any]
	Input  any
	Values *MapSafe[String, any]
}

type FSM struct {
	initial         State
	current         State
	history         *MapSafe[int64, State]
	transitions     *MapSafe[State, Slice[transition]]
	onEnter         *MapSafe[State, Slice[Callback]]
	onExit          *MapSafe[State, Slice[Callback]]
	onTransition    *MapSafe[int64, TransitionHook]
	historyCount    atomic.Int64
	transitionCount atomic.Int64

	ctx *Context
}

// NewFSM creates a new FSM with the given initial state.
//
// Example usage:
//
//	fsm := fsm.NewFSM("start").
//	    Transition("start", "begin", "step1").
//	    TransitionWhen("step1", "proceed", "done", func(ctx *fsm.Context) bool {
//	        return ctx.Input.(string) == "ok"
//	    }).
//	    OnEnter("done", func(ctx *fsm.Context) error {
//	        fmt.Println("Finished!")
//	        return nil
//	    })
//
//	fsm.Trigger("begin")
//	fsm.Context().Input = "ok"
//	fsm.Trigger("proceed")
func NewFSM(initial State) *FSM {
	fsm := &FSM{
		initial:      initial,
		current:      initial,
		history:      NewMapSafe[int64, State](),
		transitions:  NewMapSafe[State, Slice[transition]](),
		onEnter:      NewMapSafe[State, Slice[Callback]](),
		onExit:       NewMapSafe[State, Slice[Callback]](),
		onTransition: NewMapSafe[int64, TransitionHook](),
		ctx: &Context{
			State:  initial,
			Data:   NewMapSafe[String, any](),
			Values: NewMapSafe[String, any](),
		},
	}

	fsm.history.Set(0, initial)

	return fsm
}

func (f *FSM) Clone() *FSM {
	fsm := &FSM{
		initial:      f.initial,
		current:      f.initial,
		history:      NewMapSafe[int64, State](),
		transitions:  f.transitions,
		onEnter:      f.onEnter,
		onExit:       f.onExit,
		onTransition: f.onTransition,
		ctx: &Context{
			State:  f.initial,
			Data:   NewMapSafe[String, any](),
			Values: NewMapSafe[String, any](),
		},
	}

	fsm.history.Set(0, f.initial)

	return fsm
}

// Context returns the FSM's context.
func (f *FSM) Context() *Context {
	return f.ctx
}

// Current returns the FSM's current state.
func (f *FSM) Current() State {
	return f.current
}

// History returns the list of previously visited states.
func (f *FSM) History() Slice[State] {
	ordered := NewMapOrd[int64, State]()
	f.history.Iter().ForEach(func(id int64, state State) { ordered.Set(id, state) })
	ordered.SortByKey(cmp.Cmp)

	return ordered.Values()
}

// SetContext allows injecting an external context into the FSM.
func (f *FSM) SetContext(ctx *Context) {
	f.ctx = ctx
	ctx.State = f.current
}

// Reset resets the FSM to its initial state and clears all context.
func (f *FSM) Reset() {
	f.current = f.initial

	f.ctx = &Context{
		State:  f.initial,
		Data:   NewMapSafe[String, any](),
		Values: NewMapSafe[String, any](),
	}

	f.history.Clear()
	f.history.Set(0, f.initial)
	f.historyCount.Store(0)
}

// SetState sets the current state manually, without triggering callbacks.
func (f *FSM) SetState(s State) {
	f.current = s
	f.ctx.State = s
}

// Transition adds a basic transition (without guard) from -> event -> to.
func (f *FSM) Transition(from State, event Event, to State) *FSM {
	return f.TransitionWhen(from, event, to, nil)
}

// TransitionWhen adds a guarded transition from -> event -> to.
func (f *FSM) TransitionWhen(from State, event Event, to State, guard GuardFunc) *FSM {
	entry := f.transitions.Entry(from)
	entry.OrDefault()
	entry.Transform(func(s Slice[transition]) Slice[transition] {
		return s.Append(transition{event: event, to: to, guard: guard})
	})

	return f
}

// OnEnter registers a callback for when entering a given state.
func (f *FSM) OnEnter(state State, cb Callback) *FSM {
	entry := f.onEnter.Entry(state)
	entry.OrDefault()
	entry.Transform(func(cbs Slice[Callback]) Slice[Callback] { return cbs.Append(cb) })

	return f
}

// OnExit registers a callback for when exiting a given state.
func (f *FSM) OnExit(state State, cb Callback) *FSM {
	entry := f.onExit.Entry(state)
	entry.OrDefault()
	entry.Transform(func(cbs Slice[Callback]) Slice[Callback] { return cbs.Append(cb) })

	return f
}

// OnTransition registers a global transition hook called on every successful transition.
// Called after exit callbacks and before enter callbacks.
func (f *FSM) OnTransition(hook TransitionHook) *FSM {
	f.onTransition.Set(f.transitionCount.Add(1), hook)
	return f
}

// Trigger attempts to transition using the given event from the current state.
// It evaluates guards, invokes exit/enter callbacks, and updates current state.
func (f *FSM) Trigger(event Event) error {
	transitions := f.transitions.Get(f.current)
	if transitions.IsNone() {
		return fmt.Errorf("no transition for event %q from state %q", event, f.current)
	}

	matched := transitions.Some().
		Iter().
		Exclude(func(t transition) bool { return t.event != event || (t.guard != nil && !t.guard(f.ctx)) }).
		Collect()

	if matched.Empty() {
		return fmt.Errorf("no transition for event %q from state %q", event, f.current)
	}

	t := matched[0]

	if cbs := f.onExit.Get(f.current); cbs.IsSome() {
		for cb := range cbs.Some().Iter() {
			if err := cb(f.ctx); err != nil {
				return err
			}
		}
	}

	previous := f.current
	f.current = t.to
	f.ctx.State = t.to
	f.history.Set(f.historyCount.Add(1), t.to)

	for hook := range f.onTransition.Values().Iter() {
		if err := hook(previous, t.to, t.event, f.ctx); err != nil {
			return err
		}
	}

	return f.CallEnter(t.to)
}

// CallEnter manually invokes all OnEnter callbacks associated with the given state.
// This is useful for explicitly entering a state without triggering a transition event.
// It does not change the current state or modify FSM history.
func (f *FSM) CallEnter(state State) error {
	if cbs := f.onEnter.Get(state); cbs.IsSome() {
		for cb := range cbs.Some().Iter() {
			if err := cb(f.ctx); err != nil {
				return err
			}
		}
	}

	return nil
}
