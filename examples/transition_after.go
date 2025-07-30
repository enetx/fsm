package main

import (
	"context"
	"time"

	"github.com/enetx/fsm"
	"github.com/enetx/g"
)

// timerCancelFuncs is a thread-safe map to store cancellation functions for active timers.
// We need a way to associate a running timer with a specific FSM instance so we can
// cancel it if the FSM leaves the 'waiting' state prematurely. A global map provides a
// simple approach for this example; in a larger application, this might be managed
// by a dedicated service.
var timerCancelFuncs = g.NewMapSafe[fsm.StateMachine, context.CancelFunc]()

// Defining states as constants is a best practice. It prevents typos and makes the
// FSM configuration easier to read and maintain.
const (
	StateIdle      = "Idle"
	StateWaiting   = "WaitingForConfirmation"
	StateConfirmed = "Confirmed"
	StateTimedOut  = "TimedOut"
	StateCanceled  = "Canceled"
)

// Defining events as constants follows the same best practice.
const (
	EventRequest = "request"
	EventConfirm = "confirm"
	EventTimeout = "timeout" // This is our "internal" event fired by the timer.
	EventCancel  = "cancel"
)

func main() {
	// 1. Configure the FSM template with all possible transitions.
	fsmTemplate := fsm.New(StateIdle).
		Transition(StateIdle, EventRequest, StateWaiting).
		Transition(StateWaiting, EventConfirm, StateConfirmed).
		Transition(StateWaiting, EventCancel, StateCanceled).
		Transition(StateWaiting, EventTimeout, StateTimedOut)

	// 2. Get a thread-safe version of the FSM. THIS IS MANDATORY.
	// Because the timer runs in a separate goroutine, its call to Trigger() will be
	// concurrent with the main program flow. Using the synchronized FSM wrapper
	// is essential to prevent data races.
	safeFSM := fsmTemplate.Sync()

	// 3. Set up the callbacks that manage the timer's lifecycle.
	// This OnEnter callback is fired whenever the FSM enters the 'Waiting' state.
	fsmTemplate.OnEnter(StateWaiting, func(*fsm.Context) error {
		g.Println(">> Entered Waiting state. You have 3 seconds to confirm...")

		// Use context.WithTimeout to create a context that automatically cancels after a
		// specified duration.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

		// We store the `cancel` function so the OnExit callback can abort the timer early.
		timerCancelFuncs.Set(safeFSM, cancel)

		// Launch the timer logic in a new goroutine so it doesn't block the FSM transition.
		go func() {
			// We only need to wait for the context's Done channel. It will close
			// either when the timeout is reached or when cancel() is explicitly called.
			<-ctx.Done()

			// Check the context's error to determine why it finished.
			switch ctx.Err() {
			case context.DeadlineExceeded:
				// The 3-second timeout was reached.
				g.Println(">> Context deadline exceeded. Firing timeout event...")

				// Trigger the timeout event to move the FSM to the TimedOut state.
				if err := safeFSM.Trigger(EventTimeout); err != nil {
					// This error is expected if the FSM has already left the Waiting state
					// (e.g., via cancellation). We log it for clarity.
					g.Println("Error triggering timeout: {} (This is ok if we already left the state)", err)
				}
			case context.Canceled:
				// This means cancel() was called from our OnExit callback.
				// The timer was successfully aborted, so we do nothing.
				g.Println(">> Context was canceled externally.")
			}
		}()

		return nil
	})

	// This OnExit callback is fired just before the FSM *leaves* the `Waiting` state
	// for any reason (timeout, confirmation, or cancellation). Its crucial job is to
	// clean up by calling the timer's `cancel` function. This prevents an "orphaned"
	// timer from firing later and causing unexpected side effects.
	fsmTemplate.OnExit(StateWaiting, func(*fsm.Context) error {
		g.Println("<< Exiting Waiting state. Cleaning up timer...")

		// This pattern is both concise and safe for cleaning up the timer.
		//
		// 1. `timerCancelFuncs.Entry(safeFSM).Delete()`: This is an atomic operation
		//    that finds the entry for our FSM, removes it from the map, and returns
		//    the `context.CancelFunc` wrapped in an `Option` type.
		//
		// 2. `if cancel := ...; cancel.IsSome()`: This is the standard Go "if with a short
		//    statement" combined with the `Option`'s safety check. The `IsSome()`
		//    check ensures we only enter the `if` block if a cancel function was
		//    actually found and removed. This prevents a panic if the key was missing.
		//
		// 3. `cancel.Some()()`: Inside the safe block, we unwrap the Option with `Some()`
		//    and execute the `cancel` function, stopping the background goroutine.
		if cancel := timerCancelFuncs.Entry(safeFSM).Delete(); cancel.IsSome() {
			cancel.Some()()
		}

		return nil
	})

	// --- DEMONSTRATION ---

	g.Println("--- Scenario 1: Let the timer run out ---")
	safeFSM.Trigger(EventRequest)
	time.Sleep(4 * time.Second) // Wait for more than the 3-second timeout.
	g.Println("Final state: {.Current}\n", safeFSM)

	g.Println("--- Scenario 2: User cancels before the timeout ---")
	safeFSM.Reset()             // Reset the FSM for the second scenario.
	safeFSM.SetState(StateIdle) // Ensure it's back to the beginning.

	safeFSM.Trigger(EventRequest)
	time.Sleep(1 * time.Second) // Wait for a short period.
	g.Println(">> User clicks 'cancel'!")
	safeFSM.Trigger(EventCancel) // Cancel the operation before the timer fires.

	// Wait long enough to prove the orphaned timer didn't fire after being canceled.
	time.Sleep(3 * time.Second)
	g.Println("Final state: {.Current}", safeFSM)
}
