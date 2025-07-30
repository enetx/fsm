package fsm

import (
	"sync"

	"github.com/enetx/g"
)

type (
	// State represents a finite state in the FSM.
	State g.String
	// Event represents an event that triggers a transition.
	Event g.String

	// Callback is a function called on entering or exiting a state.
	Callback func(ctx *Context) error
	// GuardFunc determines whether a transition is allowed.
	GuardFunc func(ctx *Context) bool
	// TransitionHook is a global callback called after a transition between states.
	// It runs after OnExit and before OnEnter.
	TransitionHook func(from, to State, event Event, ctx *Context) error

	// transition is an internal struct representing a possible path between states.
	transition struct {
		event Event
		to    State
		guard GuardFunc
	}

	// FSM is the main state machine struct.
	FSM struct {
		initial      State
		current      State
		history      g.Slice[State]
		transitions  g.Map[State, g.Slice[transition]]
		onEnter      g.Map[State, g.Slice[Callback]]
		onExit       g.Map[State, g.Slice[Callback]]
		onTransition g.Slice[TransitionHook]

		ctx *Context
	}

	// SyncFSM is a thread-safe wrapper around an FSM.
	// It protects all state-mutating and state-reading operations with a sync.RWMutex,
	// making it safe for use across multiple goroutines.
	// All methods on SyncFSM are the thread-safe counterparts to the methods on the base FSM.
	SyncFSM struct {
		fsm *FSM
		mu  sync.RWMutex
	}
)
