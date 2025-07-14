package fsm

import "fmt"

type ErrInvalidTransition struct {
	From  State
	Event Event
}

func (e *ErrInvalidTransition) Error() string {
	return fmt.Sprintf("no matching transition for event %q from state %q", e.Event, e.From)
}

type ErrCallback struct {
	HookType string
	State    State
	Err      error
}

func (e *ErrCallback) Error() string {
	if e.State != "" {
		return fmt.Sprintf("fsm: error in %s callback for state %q: %v", e.HookType, e.State, e.Err)
	}

	return fmt.Sprintf("fsm: error in %s hook: %v", e.HookType, e.Err)
}

func (e *ErrCallback) Unwrap() error {
	return e.Err
}

// ErrUnknownState is returned when trying to unmarshal a state not defined in the FSM.
type ErrUnknownState struct {
	State State
}

func (e *ErrUnknownState) Error() string {
	return fmt.Sprintf("unknown state '%s' encountered during unmarshaling", e.State)
}
