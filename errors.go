package fsm

import "fmt"

// ErrAmbiguousTransition is returned when a trigger event results in more than one
// valid transition. This typically happens due to a configuration error where multiple
// guards for the same event from the same state return true. The FSM's behavior
// is ambiguous, so the transition is aborted to prevent non-deterministic behavior.
type ErrAmbiguousTransition struct {
	From  State
	Event Event
}

func (e *ErrAmbiguousTransition) Error() string {
	return fmt.Sprintf("fsm: ambiguous transition from state %q on event %q; multiple guards returned true",
		e.From, e.Event)
}

// ErrCallback is returned when a callback (OnEnter, OnExit) or a hook (OnTransition)
// returns an error or panics. It wraps the original error, allowing it to be
// inspected using functions like errors.Is and errors.As.
type ErrCallback struct {
	// HookType is the type of callback or hook where the error occurred (e.g., "OnEnter", "OnTransition").
	HookType string
	// State is the state associated with the callback. It may be empty for global hooks.
	State State
	// Err is the original error returned by the callback or the error created after recovering from a panic.
	Err error
}

func (e *ErrCallback) Error() string {
	if e.State != "" {
		return fmt.Sprintf("fsm: error in %s callback for state %q: %v", e.HookType, e.State, e.Err)
	}

	return fmt.Sprintf("fsm: error in %s hook: %v", e.HookType, e.Err)
}

// Unwrap provides compatibility with the standard library's errors package,
// allowing the use of errors.Is and errors.As to inspect the wrapped error.
func (e *ErrCallback) Unwrap() error { return e.Err }

// ErrInvalidTransition is returned when no matching transition is found for the given event
// from the current state.
type ErrInvalidTransition struct {
	From  State
	Event Event
}

func (e *ErrInvalidTransition) Error() string {
	return fmt.Sprintf("fsm: no matching transition for event %q from state %q", e.Event, e.From)
}

// ErrUnknownState is returned when attempting to unmarshal a state that has not
// been defined in the FSM's configuration. This prevents the FSM from entering
// an invalid, undeclared state.
type ErrUnknownState struct {
	State State
}

func (e *ErrUnknownState) Error() string {
	return fmt.Sprintf("fsm: unknown state %q encountered during unmarshaling", e.State)
}
