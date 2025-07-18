<img width="2898" height="3401" alt="image" src="https://github.com/user-attachments/assets/90e4aba6-fff3-41b5-b0d6-0676c92339ab" />

# FSM for Go

A generic, concurrent-safe, and easy-to-use finite state machine (FSM) library for Go.

This library provides a simple yet powerful API for defining states and transitions, handling callbacks, and managing stateful logic in your applications. It is built with types and utilities from the `github.com/enetx/g` library.

[![Go Reference](https://pkg.go.dev/badge/github.com/enetx/fsm.svg)](https://pkg.go.dev/github.com/enetx/fsm)
[![Go Report Card](https://goreportcard.com/badge/github.com/enetx/fsm)](https://goreportcard.com/report/github.com/enetx/fsm)

## Features

-   **Simple & Fluent API**: Define your state machine with clear, chainable methods.
-   **Fast by Default**: The base FSM is non-blocking for maximum performance in single-threaded use cases.
-   **Drop-in Concurrency**: Get a fully thread-safe FSM by calling a single `Sync()` method.
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

Here's a simple example of a traffic light state machine:

```go
package main

import (
	"fmt"
	"time"

	"github.com/enetx/fsm"
)

func main() {
	// 1. Define states and the event
	const (
		StateGreen  = "Green"
		StateYellow = "Yellow"
		StateRed    = "Red"
		EventTimer  = "timer_expires"
	)

	// 2. Configure the FSM
	lightFSM := fsm.New(StateRed).
		Transition(StateGreen, EventTimer, StateYellow).
		Transition(StateYellow, EventTimer, StateRed).
		Transition(StateRed, EventTimer, StateGreen)

	// 3. Define callbacks for entering states
	lightFSM.OnEnter(StateGreen, func(ctx *fsm.Context) error {
		fmt.Println("LIGHT: Green -> Go!")
		return nil
	})
	lightFSM.OnEnter(StateYellow, func(ctx *fsm.Context) error {
		fmt.Println("LIGHT: Yellow -> Prepare to stop")
		return nil
	})
	lightFSM.OnEnter(StateRed, func(ctx *fsm.Context) error {
		fmt.Println("LIGHT: Red -> Stop!")
		return nil
	})

	// 4. Run the FSM loop
	fmt.Printf("Initial state: %s\n", lightFSM.Current())
	lightFSM.CallEnter(StateRed) // Manually trigger the first prompt

	for range 4  {
		time.Sleep(1 * time.Second)
		fmt.Println("\n...timer expires...")
		lightFSM.Trigger(EventTimer)
	}
}
```

### Output

```text
Initial state: Red
LIGHT: Red -> Stop!

...timer expires...
LIGHT: Green -> Go!

...timer expires...
LIGHT: Yellow -> Prepare to stop

...timer expires...
LIGHT: Red -> Stop!

...timer expires...
LIGHT: Green -> Go!
```

## API Overview

### Creating an FSM

```go
// Create a new FSM instance (not thread-safe)
fsmachine := fsm.New("initial_state")

// Get a thread-safe wrapper for concurrent use
safeFSM := fsmachine.Sync()
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
-   `ctx.Data`: A concurrent-safe map (`g.MapSafe`) for persistent data that is serialized with the FSM (e.g., user details).
-   `ctx.Meta`: A concurrent-safe map (`g.MapSafe`) for ephemeral metadata that is also serialized (e.g., temporary counters).

### Concurrency

The library is designed with performance and safety in mind, offering two distinct operating modes:

1.  **`fsm.FSM` (Default)**: The base state machine is **not** thread-safe. It is optimized for performance in single-threaded scenarios by avoiding the overhead of mutexes.

2.  **`fsm.SyncFSM` (Synchronized)**: This is a thread-safe wrapper around the base `FSM`. It protects all operations (like `Trigger`, `Current`, `Reset`) with a mutex, ensuring that all transitions are atomic and safe to use across multiple goroutines.

You should complete all configuration (`Transition`, `OnEnter`, etc.) on the base `FSM` before using it. The configuration process itself is **not** thread-safe.

#### Activating Thread-Safety

To get a thread-safe instance, simply call the `Sync()` method after you have configured your FSM:

```go
// 1. Configure the non-thread-safe FSM template
fsmTemplate := fsm.New("idle").
    Transition("idle", "start", "running").
    Transition("running", "stop", "stopped")

// 2. Get a thread-safe, synchronized instance
safeFSM := fsmTemplate.Sync()

// 3. Now you can safely use safeFSM across multiple goroutines
go func() {
    err := safeFSM.Trigger("start")
    // ...
}()

go func() {
    currentState := safeFSM.Current()
    // ...
}()
```

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
restoredFSM := fsm.New("initial_state").
    Transition(...) // ...add all transitions and callbacks

// 2. Unmarshal the JSON data into the new instance.
err := json.Unmarshal(jsonData, restoredFSM)
if err != nil {
    // handle error
}

// `restoredFSM` is now in the same state as the original was.
fmt.Println(restoredFSM.Current())
```
**Note**: Serialization only saves the FSM's state (`current`, `history`, `Data`, `Meta`). It does not save the transition rules or callbacks. You must configure the FSM template before unmarshaling. If you need a thread-safe FSM after restoring, call `.Sync()` *after* `json.Unmarshal`.

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
    fsmachine := fsm.New("Idle").
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
<img width="941" height="360" alt="graphviz" src="https://github.com/user-attachments/assets/2516c2c4-582f-4c08-81e6-4b8c36a1920c" />

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue for bugs, feature requests, or questions.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
