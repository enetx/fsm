package fsm

import . "github.com/enetx/g"

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

func newContext(initial State) *Context {
	return &Context{
		State: initial,
		Data:  NewMapSafe[String, any](),
		Meta:  NewMapSafe[String, any](),
	}
}
