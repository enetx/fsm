package main

import (
	"encoding/json"
	"fmt"

	"github.com/enetx/fsm"
	. "github.com/enetx/g"
)

func defineFSM() fsm.StateMachine {
	return fsm.New("idle").
		Transition("idle", "start", "running").
		Transition("running", "pause", "paused").
		Transition("running", "stop", "stopped").
		Transition("paused", "resume", "running").
		OnEnter("running", func(*fsm.Context) error {
			fmt.Println("State machine is now running.")
			return nil
		}).Sync()
}

func main() {
	myFSM := defineFSM()

	myFSM.Context().Data.Set("processID", 12345)

	if err := myFSM.Trigger("start"); err != nil {
		panic(err)
	}

	if err := myFSM.Trigger("pause"); err != nil {
		panic(err)
	}

	Println("Original FSM state: {}", myFSM.Current())
	Println("Original FSM history: {}", myFSM.History())
	Println("--------------------")

	jsonData, err := json.MarshalIndent(myFSM, "", "  ")
	if err != nil {
		panic(err)
	}

	Println("Serialized FSM:\n{}", String(jsonData))
	Println("--------------------")

	restoredFSM := defineFSM()

	if err := json.Unmarshal(jsonData, restoredFSM); err != nil {
		panic(err)
	}

	Println("Restored FSM state: {}", restoredFSM.Current())
	Println("Restored FSM history: {}", restoredFSM.History())

	pid := restoredFSM.Context().Data.Get("processID")
	Println("Restored context data 'processID': {}", pid.Some())

	fmt.Println("--------------------")
	fmt.Println("Resuming restored FSM...")

	if err := restoredFSM.Trigger("resume"); err != nil {
		panic(err)
	}

	Println("New state after resume: {}", restoredFSM.Current())
}
