package fsm

import (
	"encoding/json"
	"fmt"

	"github.com/enetx/g"
)

// FSMState is a serializable representation of the FSM's state.
// It uses standard map types for robust JSON handling.
type FSMState struct {
	Current State                `json:"current"`
	History g.Slice[State]       `json:"history"`
	Data    g.Map[g.String, any] `json:"data"`
	Meta    g.Map[g.String, any] `json:"meta"`
}

// MarshalJSON implements the json.Marshaler interface.
func (f *FSM) MarshalJSON() ([]byte, error) {
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
	var state FSMState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal fsm state: %w", err)
	}

	states := f.States()
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
	f.ctx.Data = state.Data.Safe()
	f.ctx.Meta = state.Meta.Safe()

	return nil
}
