package fsm

import . "github.com/enetx/g"

// Interface compliance check.
var _ StateMachine = (*SyncFSM)(nil)

// Trigger is the thread-safe version of FSM.Trigger.
// It atomically executes a state transition in response to an event.
func (sf *SyncFSM) Trigger(event Event, input ...any) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	return sf.fsm.Trigger(event, input...)
}

// Current is the thread-safe version of FSM.Current.
// It returns the FSM's current state.
func (sf *SyncFSM) Current() State {
	sf.mu.RLock()
	defer sf.mu.RUnlock()

	return sf.fsm.Current()
}

// Context is the thread-safe version of FSM.Context.
// It returns a pointer to the FSM's context.
func (sf *SyncFSM) Context() *Context {
	sf.mu.RLock()
	defer sf.mu.RUnlock()

	return sf.fsm.Context()
}

// SetState is the thread-safe version of FSM.SetState.
// It forcefully sets the current state, bypassing all callbacks and guards.
// WARNING: This is a low-level method intended for specific use cases like
// state restoration. For all standard operations, use Trigger.
func (sf *SyncFSM) SetState(s State) {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	sf.fsm.SetState(s)
}

// Reset is the thread-safe version of FSM.Reset.
// It resets the FSM to its initial state and clears its context.
func (sf *SyncFSM) Reset() {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	sf.fsm.Reset()
}

// History is the thread-safe version of FSM.History.
// It returns a copy of the state transition history.
func (sf *SyncFSM) History() Slice[State] {
	sf.mu.RLock()
	defer sf.mu.RUnlock()

	return sf.fsm.History()
}

// States is the thread-safe version of FSM.States.
// It returns a slice of all unique states defined in the FSM.
func (sf *SyncFSM) States() Slice[State] {
	sf.mu.RLock()
	defer sf.mu.RUnlock()

	return sf.fsm.States()
}

// CallEnter is the thread-safe version of FSM.CallEnter.
// It manually invokes the OnEnter callbacks for a given state without a transition.
func (sf *SyncFSM) CallEnter(state State) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	return sf.fsm.CallEnter(state)
}

// ToDOT is the thread-safe version of FSM.ToDOT.
// It generates a DOT language string representation of the FSM for visualization.
func (sf *SyncFSM) ToDOT() String {
	sf.mu.RLock()
	defer sf.mu.RUnlock()

	return sf.fsm.ToDOT()
}

// MarshalJSON implements the json.Marshaler interface for thread-safe
// serialization of the FSM's state to JSON.
func (sf *SyncFSM) MarshalJSON() ([]byte, error) {
	sf.mu.RLock()
	defer sf.mu.RUnlock()

	return sf.fsm.MarshalJSON()
}

// UnmarshalJSON implements the json.Unmarshaler interface for thread-safe
// deserialization of the FSM's state from JSON.
func (sf *SyncFSM) UnmarshalJSON(data []byte) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	return sf.fsm.UnmarshalJSON(data)
}
