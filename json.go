package fsm

import (
	"encoding/json"
	"fmt"

	. "github.com/enetx/g"
)

// FSMState is a serializable representation of the FSM's state.
// It uses standard map types for robust JSON handling.
type FSMState struct {
	Current State            `json:"current"`
	History Slice[State]     `json:"history"`
	Data    Map[String, any] `json:"data"`
	Meta    Map[String, any] `json:"meta"`
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
