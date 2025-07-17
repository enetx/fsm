<img width="2898" height="3401" alt="image" src="https://github.com/user-attachments/assets/90e4aba6-fff3-41b5-b0d6-0676c92339ab" />

# FSM for Go

A generic, concurrent-safe, and easy-to-use finite state machine (FSM) library for Go.

This library provides a simple yet powerful API for defining states and transitions, handling callbacks, and managing stateful logic in your applications. It is built with types and utilities from the `github.com/enetx/g` library.

[![Go Reference](https://pkg.go.dev/badge/github.com/enetx/fsm.svg)](https://pkg.go.dev/github.com/enetx/fsm)
[![Go Report Card](https://goreportcard.com/badge/github.com/enetx/fsm)](https://goreportcard.com/report/github.com/enetx/fsm)

## Features

-   **Simple & Fluent API**: Define your state machine with clear, chainable methods.
-   **Concurrent-Safe**: Designed for use in multi-threaded applications. All state transitions are atomic.
-   **State Callbacks**: Execute code on entering (`OnEnter`) or exiting (`OnExit`) a state.
-   **Global Transition Hooks**: `OnTransition` allows you to monitor and log all state changes globally.
-   **Guarded Transitions**: Control transitions with `TransitionWhen` based on custom logic.
-   **JSON Serialization**: Easily save and restore the FSM's state with built-in `json.Marshaler` and `json.Unmarshaler` support.
-   **Graphviz Visualization**: Generate DOT-format graphs to visualize your FSM.
-   **Zero Dependencies** (besides `github.com/enetx/g`).

## Installation

```sh
go get github.com/enetx/fsm
```

## Quick Start

Here's a simple example of a user survey bot's logic:

```go
package main

import (
	"fmt"

	"github.com/enetx/fsm"
)

func main() {
	// 1. Define the FSM states and events
	const (
		StateAskName = "ask_name"
		StateAskAge  = "ask_age"
		StateDone    = "done"

		EventAnswer = "answer"
	)

	// 2. Configure the FSM template
	fsmachine := fsm.NewFSM(StateAskName).
		Transition(StateAskName, EventAnswer, StateAskAge).
		Transition(StateAskAge, EventAnswer, StateDone)

	// 3. Define callbacks for entering states
	fsmachine.OnEnter(StateAskName, func(ctx *fsm.Context) error {
		fmt.Println("Bot: Hello! What is your name?")
		return nil
	})

	fsmachine.OnEnter(StateAskAge, func(ctx *fsm.Context) error {
		name := ctx.Input.(string)
		ctx.Data.Set("name", name) // Store the name
		fmt.Printf("Bot: Nice to meet you, %s! How old are you?\n", name)
		return nil
	})

	fsmachine.OnEnter(StateDone, func(ctx *fsm.Context) error {
		name := ctx.Data.Get("name").Some()
		age := ctx.Input.(string)
		fmt.Printf("Bot: Got it! Your name is %s and you are %s years old.\n", name, age)
		return nil
	})

	// 4. Run the FSM
	fmt.Printf("Current state: %s\n", fsmachine.Current())
	fsmachine.CallEnter(StateAskName) // Manually trigger the first prompt

	fmt.Println("\nUser: Alice")
	fsmachine.Trigger(EventAnswer, "Alice")
	fmt.Printf("Current state: %s\n", fsmachine.Current())

	fmt.Println("\nUser: 30")
	fsmachine.Trigger(EventAnswer, "30")
	fmt.Printf("Current state: %s\n", fsmachine.Current())
}
```

### Output

```text
Current state: ask_name
Bot: Hello! What is your name?

User: Alice
Bot: Nice to meet you, Alice! How old are you?
Current state: ask_age

User: 30
Bot: Got it! Your name is Alice and you are 30 years old.
Current state: done
```

## API Overview

### Creating an FSM

```go
fsmachine := fsm.NewFSM("initial_state")
```

### Defining Transitions

-   **`Transition(from, event, to)`**: A direct, unconditional transition.
-   **`TransitionWhen(from, event, to, guard)`**: A transition that only occurs if the `guard` function returns `true`.

```go
fsmachine.Transition("idle", "start", "running")

fsmachine.TransitionWhen("running", "stop", "stopped", func(ctx *fsm.Context) bool {
    // Only allow stopping if a specific condition is met
    return ctx.Data.Get("can_stop").UnwrapOr(false).(bool)
})
```

### Callbacks and Hooks

-   **`OnEnter(state, callback)`**: Called when the FSM enters `state`.
-   **`OnExit(state, callback)`**: Called before the FSM exits `state`.
-   **`OnTransition(hook)`**: Called on *every* successful transition, after `OnExit` and before `OnEnter`.

```go
fsmachine.OnEnter("running", func(ctx *fsm.Context) error {
    fmt.Println("Job started!")
    return nil
})

fsmachine.OnExit("running", func(ctx *fsm.Context) error {
    fmt.Println("Cleaning up job...")
    return nil
})

fsmachine.OnTransition(func(from, to fsm.State, event fsm.Event, ctx *fsm.Context) error {
    log.Printf("STATE CHANGE: %s -> %s (on event %s)", from, to, event)
    return nil
})
```

### Triggering Events

The `Trigger` method drives the state machine.

```go
// Simple trigger
err := fsmachine.Trigger("start")

// Trigger with data payload
// The data will be available in the context as `ctx.Input`.
err := fsmachine.Trigger("process", someDataObject)
```
Any error returned from a callback will halt the transition and be returned by `Trigger`.

### Context

The `Context` is passed to every callback and guard. It's the primary way to manage data associated with an FSM instance.

-   `ctx.Input`: Holds the data passed with the current `Trigger` call. It's ephemeral and lasts for one transition only.
-   `ctx.Data`: A concurrent-safe map for persistent data (e.g., user details).
-   `ctx.Meta`: A concurrent-safe map for ephemeral metadata (e.g., temporary counters).

### Concurrency

This library is designed to be safe for concurrent use. All methods that modify the FSM's state are protected by a mutex, ensuring that transitions are atomic.

-   Configuration methods like `TransitionWhen`, `OnEnter`, and `OnExit` are safe to call concurrently as they operate on a thread-safe map.
-   State-changing methods like `Trigger`, `Reset`, and `SetState` acquire a lock to ensure atomicity.

### Serialization

You can easily save and restore the FSM's state using `encoding/json`, as `FSM` implements the `json.Marshaler` and `json.Unmarshaler` interfaces.

**Saving State:**
```go
// Assume `fsmachine` is in some state.
jsonData, err := json.Marshal(fsmachine)
if err != nil {
    // handle error
}
// Now you can save `jsonData` to a database, file, etc.
```

**Restoring State:**
```go
// 1. Create a new FSM with the same configuration as the original.
restoredFSM := fsm.NewFSM("initial_state").
    Transition(...) // ...add all transitions and callbacks

// 2. Unmarshal the JSON data into the new instance.
err := json.Unmarshal(jsonData, restoredFSM)
if err != nil {
    // handle error
}

// `restoredFSM` is now in the same state as the original was.
fmt.Println(restoredFSM.Current())
```
**Note**: Serialization only saves the FSM's state (`current`, `history`, `Data`, `Meta`). It does not save the transition rules or callbacks. You must configure the FSM template before unmarshaling.

### Visualization

The library includes a `ToDOT()` method to generate a graph of your state machine in the [DOT language](https://graphviz.org/doc/info/lang.html). This is extremely useful for debugging, documentation, and sharing your FSM's logic with your team.

You can render the output into an image using various tools:

*   **Online Editors (Recommended for quick use):**
    *   [**Graphviz Online**](https://dreampuf.github.io/GraphvizOnline/) - A simple and effective web-based viewer.
    *   [**Edotor**](https://edotor.net/) - Another powerful online editor with different layout engines.
    *   Simply paste the output of `ToDOT()` into one of these sites to see your diagram instantly.

*   **Local Installation:**
    *   For more advanced use or integration into build scripts, you can install [**Graphviz**](https://graphviz.org/download/) locally.

**Example:**

```go
func main() {
    fsmachine := fsm.NewFSM("Idle").
        Transition("Idle", "start", "Running").
        TransitionWhen("Running", "suspend", "Suspended", func(ctx *fsm.Context) bool {
            return true
        }).
        Transition("Suspended", "resume", "Running").
        Transition("Running", "finish", "Done")

    // Generate the DOT string
    fsmachine.ToDOT().Println() // Copy this output
}
```
<img width="2091" height="800" alt="graphviz" src="https://github.com/user-attachments/assets/dc021969-5a82-4260-9cfe-462d21c84fa8" />

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue for bugs, feature requests, or questions.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
