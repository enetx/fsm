// Package fsm provides a generic finite state machine (FSM) implementation
// with support for transitions, guards, and enter/exit callbacks. It is built
// with types and utilities from the github.com/enetx/g library.
package fsm

import (
	"fmt"

	. "github.com/enetx/g"
)

// NewFSM creates a new FSM with the given initial state.
func NewFSM(initial State) *FSM {
	return &FSM{
		initial:      initial,
		current:      initial,
		history:      Slice[State]{initial},
		transitions:  NewMap[State, Slice[transition]](),
		onEnter:      NewMap[State, Slice[Callback]](),
		onExit:       NewMap[State, Slice[Callback]](),
		onTransition: NewSlice[TransitionHook](),
		ctx:          newContext(initial),
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
		ctx:          newContext(f.initial),
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
	f.ctx = newContext(f.initial)
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
	f.mu.Lock()
	defer f.mu.Unlock()

	entry := f.transitions.Entry(from)
	entry.OrDefault()
	entry.Transform(func(s Slice[transition]) Slice[transition] {
		return s.Append(transition{event: event, to: to, guard: guard})
	})

	return f
}

// OnEnter registers a callback for when entering a given state.
func (f *FSM) OnEnter(state State, cb Callback) *FSM {
	f.mu.Lock()
	defer f.mu.Unlock()

	entry := f.onEnter.Entry(state)
	entry.OrDefault()
	entry.Transform(func(cbs Slice[Callback]) Slice[Callback] { return cbs.Append(cb) })

	return f
}

// OnExit registers a callback for when exiting a given state.
func (f *FSM) OnExit(state State, cb Callback) *FSM {
	f.mu.Lock()
	defer f.mu.Unlock()

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
