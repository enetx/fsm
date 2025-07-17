package main

import (
	"errors"
	"fmt"

	"github.com/enetx/fsm"
)

// This is a custom error that our application's business logic might produce.
// We'll return this from a callback to show how the FSM wraps it.
var ErrReviewerOnVacation = errors.New("cannot review, reviewer is on vacation")

func main() {
	// --- Setup FSM ---
	// We create a new FSM for a document workflow.
	workflowFSM := fsm.New("Draft").
		// Define valid transitions
		Transition("Draft", "Submit", "InReview").
		Transition("InReview", "Approve", "Approved").
		Transition("InReview", "Reject", "Rejected").
		// Define callbacks for specific state events
		OnEnter("InReview", func(*fsm.Context) error {
			// This callback simulates a bug by panicking.
			// Our FSM should recover from this and return a proper error.
			fmt.Println("   -> Entering 'InReview' state... but something will go wrong!")
			panic("database connection lost")
		}).
		OnEnter("Rejected", func(*fsm.Context) error {
			// This callback returns our custom application error.
			// The FSM should wrap this error in an ErrCallback.
			fmt.Println("   -> Entering 'Rejected' state...")
			return ErrReviewerOnVacation
		}).
		OnEnter("Approved", func(*fsm.Context) error {
			fmt.Println("   -> Document has been approved! Congratulations.")
			return nil
		})

	// --- 1. Demonstrating ErrInvalidTransition ---
	// Let's try to trigger an event that is not valid from the current "Draft" state.
	fmt.Println("--- 1. Testing an invalid transition ---")
	err := workflowFSM.Trigger("Approve") // You can't approve a document from "Draft".
	handleFSMError(err)
	fmt.Printf("Current state is still: %s\n\n", workflowFSM.Current())

	// --- 2. Demonstrating ErrCallback caused by a Panic ---
	// Now, let's trigger a valid transition that leads to a panicking callback.
	fmt.Println("--- 2. Testing a callback that panics ---")
	// The FSM needs to be reset for this part of the demo, otherwise it's still in "Draft"
	// but the `OnEnter("InReview", ...)` callback will not be re-defined. We'll make a new one.

	// Re-create the FSM for this test case
	panickingFSM := fsm.New("Draft").
		Transition("Draft", "Submit", "InReview").
		OnEnter("InReview", func(*fsm.Context) error {
			fmt.Println("   -> Entering 'InReview'... about to panic!")
			panic("database connection lost")
		})

	err = panickingFSM.Trigger("Submit")
	handleFSMError(err)
	// Note: Because the error happened during the transition, the state might not have changed.
	fmt.Printf("Current state is still: %s\n\n", panickingFSM.Current())

	// --- 3. Demonstrating ErrCallback caused by a returned error ---
	// Finally, let's test a transition where the callback returns our specific business error.
	fmt.Println("--- 3. Testing a callback that returns a specific error ---")

	// Create a new FSM for this final test case
	businessErrorFSM := fsm.New("Draft").
		Transition("Draft", "Submit", "InReview").
		Transition("InReview", "Reject", "Rejected").
		OnEnter("InReview", func(*fsm.Context) error {
			fmt.Println("   -> Entering 'InReview' successfully.")
			return nil
		}).
		OnEnter("Rejected", func(*fsm.Context) error {
			fmt.Println("   -> Entering 'Rejected', returning a business error.")
			return ErrReviewerOnVacation
		})

	_ = businessErrorFSM.Trigger("Submit")   // This one will succeed.
	err = businessErrorFSM.Trigger("Reject") // This will trigger the callback that returns our error.
	handleFSMError(err)
	fmt.Printf("Current state is now: %s\n\n", businessErrorFSM.Current())
}

// handleFSMError is a helper function to inspect and print details about FSM errors.
// This shows how a user of your library can programmatically handle different failure modes.
func handleFSMError(err error) {
	if err == nil {
		fmt.Println("   SUCCESS: No error occurred.")
		return
	}

	fmt.Println("   ERROR: An error occurred.")

	var invTransErr *fsm.ErrInvalidTransition
	var cbErr *fsm.ErrCallback

	// Use errors.As to check if the error is of a specific type.
	switch {
	// Case 1: The error is an invalid transition.
	case errors.As(err, &invTransErr):
		fmt.Printf("   - Type: Invalid Transition\n")
		fmt.Printf("   - Details: Cannot trigger event '%s' from state '%s'.\n", invTransErr.Event, invTransErr.From)
	// Case 2: The error came from a callback (either a panic or a returned error).
	case errors.As(err, &cbErr):
		fmt.Printf("   - Type: Callback Error\n")
		fmt.Printf("   - Details: Hook '%s' for state '%s' failed.\n", cbErr.HookType, cbErr.State)

		// Now we can inspect the original error that was wrapped inside ErrCallback.
		// Use errors.Is to check if the wrapped error is our specific business error.
		if errors.Is(cbErr.Unwrap(), ErrReviewerOnVacation) {
			fmt.Println("   - Root Cause: This was our specific business error! The reviewer is on vacation.")
		} else {
			// Otherwise, just print the generic wrapped error message. This will show the panic message.
			fmt.Printf("   - Root Cause: %v\n", cbErr.Unwrap())
		}
	// Default case for any other kind of error.
	default:
		fmt.Printf("   - An unknown error occurred: %v\n", err)
	}
}
