package main

import (
	"encoding/json"

	"github.com/enetx/fsm"
	"github.com/enetx/g"
)

func defFSM() *fsm.FSM {
	return fsm.New("idle").
		Transition("idle", "start", "running").
		Transition("running", "pause", "paused").
		Transition("running", "stop", "stopped").
		Transition("paused", "resume", "running")
}

func main() {
	g.Println("--- Scenario 1: Successful Restore ---")

	validJSON := []byte(`{
		"current": "paused",
		"history": ["idle", "running", "paused"],
		"data": {"processID": 54321},
		"values": {}
	}`)

	fsm1 := defFSM()

	err := json.Unmarshal(validJSON, fsm1)
	if err != nil {
		g.Println("Error restoring FSM: {}", err)
	} else {
		g.Println("Successfully restored FSM to state: {}", fsm1.Current())
		pid := fsm1.Context().Data.Get("processID")
		g.Println("Restored context data 'processID': {}", pid.Some())
	}

	g.Println("\n----------------------------------------\n")

	g.Println("--- Scenario 2: Restore with Unknown State ---")

	invalidJSON := []byte(`{
		"current": "crashed",
		"history": ["idle", "running", "crashed"],
		"data": {},
		"values": {}
	}`)

	fsm2 := defFSM()

	err = json.Unmarshal(invalidJSON, fsm2)
	if err != nil {
		g.Println("Error restoring FSM: {}", err)
		g.Println("FSM remains in initial state: {}", fsm2.Current())
	} else {
		g.Println("FSM restored unexpectedly.")
	}
}
