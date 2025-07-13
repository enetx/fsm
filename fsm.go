// Package fsm provides a generic finite state machine (FSM) implementation
// with support for transitions, guards, and enter/exit callbacks. It is built
// with types and utilities from the github.com/enetx/g library.
package fsm

import (
	"fmt"
	"sync"

	. "github.com/enetx/g"
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
	initial      State
	current      State
	history      Slice[State]
	transitions  *MapSafe[State, Slice[transition]]
	onEnter      *MapSafe[State, Slice[Callback]]
	onExit       *MapSafe[State, Slice[Callback]]
	onTransition Slice[TransitionHook]

	ctx *Context
	mu  sync.RWMutex
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
	return &FSM{
		initial:      initial,
		current:      initial,
		history:      Slice[State]{initial},
		transitions:  NewMapSafe[State, Slice[transition]](),
		onEnter:      NewMapSafe[State, Slice[Callback]](),
		onExit:       NewMapSafe[State, Slice[Callback]](),
		onTransition: NewSlice[TransitionHook](),
		ctx: &Context{
			State:  initial,
			Data:   NewMapSafe[String, any](),
			Values: NewMapSafe[String, any](),
		},
	}
}

func (f *FSM) Clone() *FSM {
	return &FSM{
		initial:      f.initial,
		current:      f.initial,
		history:      Slice[State]{f.initial},
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
}

// Context returns the FSM's context.
func (f *FSM) Context() *Context {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.ctx
}

// Current returns the FSM's current state.
func (f *FSM) Current() State {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.current
}

// History returns the list of previously visited states.
func (f *FSM) History() Slice[State] {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.history.Clone()
}

// SetContext allows injecting an external context into the FSM.
func (f *FSM) SetContext(ctx *Context) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ctx = ctx
	ctx.State = f.current
}

// Reset resets the FSM to its initial state and clears all context.
func (f *FSM) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.current = f.initial

	f.ctx = &Context{
		State:  f.initial,
		Data:   NewMapSafe[String, any](),
		Values: NewMapSafe[String, any](),
	}

	f.history = Slice[State]{f.initial}
}

// SetState sets the current state manually, without triggering callbacks.
func (f *FSM) SetState(s State) {
	f.mu.Lock()
	defer f.mu.Unlock()

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
	f.mu.Lock()
	defer f.mu.Unlock()

	f.onTransition.Push(hook)
	return f
}

// Trigger attempts to transition using the given event from the current state.
// It evaluates guards, invokes exit/enter callbacks, and updates current state.
func (f *FSM) Trigger(event Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	transitions := f.transitions.Get(f.current)
	if transitions.IsNone() {
		return &ErrInvalidTransition{From: f.current, Event: event}
	}

	matched := transitions.Some().
		Iter().
		Exclude(func(t transition) bool { return t.event != event || (t.guard != nil && !t.guard(f.ctx)) }).
		Collect()

	if matched.Empty() {
		return &ErrInvalidTransition{From: f.current, Event: event}
	}

	t := matched[0]
	previousState := f.current
	nextState := t.to

	if cbs := f.onExit.Get(previousState); cbs.IsSome() {
		for cb := range cbs.Some().Iter() {
			if err := f.executeCallback(cb, "OnExit", previousState); err != nil {
				return err
			}
		}
	}

	for hook := range f.onTransition.Iter() {
		if err := func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = &ErrCallback{HookType: "OnTransition", Err: fmt.Errorf("panic: %v", r)}
				}
			}()
			if hookErr := hook(previousState, nextState, event, f.ctx); hookErr != nil {
				err = &ErrCallback{HookType: "OnTransition", Err: hookErr}
			}
			return err
		}(); err != nil {
			return err
		}
	}

	if cbs := f.onEnter.Get(nextState); cbs.IsSome() {
		for cb := range cbs.Some().Iter() {
			if err := f.executeCallback(cb, "OnEnter", nextState); err != nil {
				return err
			}
		}
	}

	f.current = nextState
	f.ctx.State = nextState
	f.history.Push(nextState)

	return nil
}

func (f *FSM) executeCallback(cb Callback, hookType string, state State) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &ErrCallback{HookType: hookType, State: state, Err: fmt.Errorf("panic: %v", r)}
		}
	}()

	if cbErr := cb(f.ctx); cbErr != nil {
		err = &ErrCallback{HookType: hookType, State: state, Err: cbErr}
	}

	return err
}

// CallEnter manually invokes all OnEnter callbacks associated with the given state.
// This is useful for explicitly entering a state without triggering a transition event.
// It does not change the current state or modify FSM history.
func (f *FSM) CallEnter(state State) error {
	if cbs := f.onEnter.Get(state); cbs.IsSome() {
		for cb := range cbs.Some().Iter() {
			if err := f.executeCallback(cb, "OnEnter", state); err != nil {
				return err
			}
		}
	}

	return nil
}
