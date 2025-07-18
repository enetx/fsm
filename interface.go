package fsm

import . "github.com/enetx/g"

type StateMachine interface {
	Trigger(Event, ...any) error
	Current() State
	Context() *Context
	SetState(State)
	Reset()
	History() Slice[State]
	States() Slice[State]
	ToDOT() String
	MarshalJSON() ([]byte, error)
	UnmarshalJSON(data []byte) error
}
