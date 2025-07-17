// Package fsm provides a generic finite state machine (FSM) implementation
// with support for transitions, guards, and enter/exit callbacks. It is built
// with types and utilities from the github.com/enetx/g library.
package fsm

import (
	"encoding/json"
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

// transition is an internal struct representing a possible path between states.
type transition struct {
	event Event
	to    State
	guard GuardFunc
}

// Context holds FSM state, input, persistent and temporary data.
// Data is for long-lived values (e.g. user ID, settings) and is serialized.
// Meta is for ephemeral metadata (e.g. timestamps, counters) and is also serialized.
// Input holds data specific to the current trigger event and is NOT serialized.
// State holds the state for which a callback is being executed.
type Context struct {
	State State
	Input any
	Data  *MapSafe[String, any]
	Meta  *MapSafe[String, any]
}

// FSM is the main state machine struct.
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

// FSMState is a serializable representation of the FSM's state.
// It uses standard map types for robust JSON handling.
type FSMState struct {
	Current State            `json:"current"`
	History Slice[State]     `json:"history"`
	Data    Map[String, any] `json:"data"`
	Meta    Map[String, any] `json:"meta"`
}

// NewFSM creates a new FSM with the given initial state.
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
			State: initial,
			Data:  NewMapSafe[String, any](),
			Meta:  NewMapSafe[String, any](),
		},
	}
}

// Clone creates a new FSM instance with the same configuration but a fresh state.
func (f *FSM) Clone() *FSM {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return &FSM{
		initial:      f.initial,
		current:      f.initial,
		history:      Slice[State]{f.initial},
		transitions:  f.transitions,
		onEnter:      f.onEnter,
		onExit:       f.onExit,
		onTransition: f.onTransition,
		ctx: &Context{
			State: f.initial,
			Data:  NewMapSafe[String, any](),
			Meta:  NewMapSafe[String, any](),
		},
	}
}

// Context returns the FSM's context for managing data.
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

// History returns a copy of the list of previously visited states.
func (f *FSM) History() Slice[State] {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.history.Clone()
}

// Reset resets the FSM to its initial state and clears all context data.
func (f *FSM) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.current = f.initial

	f.ctx = &Context{
		State: f.initial,
		Data:  NewMapSafe[String, any](),
		Meta:  NewMapSafe[String, any](),
	}

	f.history = Slice[State]{f.initial}
}

// SetState sets the current state manually, without triggering any callbacks.
func (f *FSM) SetState(s State) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.current = s
	f.ctx.State = s
}

// states is the internal, non-locking implementation for retrieving defined states.
func (f *FSM) states() Slice[State] {
	stateSet := NewSet[State]()
	stateSet.Insert(f.initial)

	for state, transitions := range f.transitions.Iter() {
		stateSet.Insert(state)
		for transition := range transitions.Iter() {
			stateSet.Insert(transition.to)
		}
	}

	return stateSet.ToSlice()
}

// States returns a slice of all unique states defined in the FSM's transitions.
func (f *FSM) States() Slice[State] {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.states()
}

// Transition adds a basic transition (without a guard) from -> event -> to.
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

// OnTransition registers a global transition hook.
func (f *FSM) OnTransition(hook TransitionHook) *FSM {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.onTransition.Push(hook)
	return f
}

// Trigger attempts to transition using the given event.
// It accepts an optional single 'input' argument to pass data to guards and callbacks.
// This input is only valid for the duration of this specific trigger cycle.
func (f *FSM) Trigger(event Event, input ...any) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(input) > 0 {
		f.ctx.Input = input[0]
	} else {
		f.ctx.Input = nil
	}

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

	f.ctx.State = previousState

	if cbs := f.onExit.Get(previousState); cbs.IsSome() {
		for cb := range cbs.Some().Iter() {
			if err := f.executeCallback(cb, "OnExit", previousState); err != nil {
				return err
			}
		}
	}

	f.ctx.State = nextState

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
	f.history.Push(nextState)

	return nil
}

// executeCallback safely executes a callback, recovering from panics.
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

// CallEnter manually invokes all OnEnter callbacks for a state without a transition.
// Note: It does not set ctx.Input. Use ctx.Values/Meta for pre-loading data.
func (f *FSM) CallEnter(state State) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ctx.State = state
	if cbs := f.onEnter.Get(state); cbs.IsSome() {
		for cb := range cbs.Some().Iter() {
			if err := f.executeCallback(cb, "OnEnter", state); err != nil {
				return err
			}
		}
	}

	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (f *FSM) MarshalJSON() ([]byte, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	state := FSMState{
		Current: f.current,
		History: f.history.Clone(),
		Data:    f.ctx.Data.Iter().Collect(),
		Meta:    f.ctx.Meta.Iter().Collect(),
	}

	return json.Marshal(state)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (f *FSM) UnmarshalJSON(data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var state FSMState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal fsm state: %w", err)
	}

	states := f.states()
	if !states.Contains(state.Current) {
		return &ErrUnknownState{State: state.Current}
	}

	for state := range state.History.Iter() {
		if !states.Contains(state) {
			return &ErrUnknownState{State: state}
		}
	}

	f.current = state.Current
	f.history = state.History
	f.ctx.State = state.Current
	f.ctx.Data = state.Data.ToMapSafe()
	f.ctx.Meta = state.Meta.ToMapSafe()

	return nil
}
