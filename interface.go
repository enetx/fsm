package fsm

import "github.com/enetx/g"

type StateMachine interface {
	Trigger(Event, ...any) error
	Current() State
	Context() *Context
	SetState(State)
	Reset()
	History() g.Slice[State]
	States() g.Slice[State]
	ToDOT() g.String
	MarshalJSON() ([]byte, error)
	UnmarshalJSON(data []byte) error
}
