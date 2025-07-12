package main

import (
	"bufio"
	"os"
	"time"

	. "github.com/enetx/fsm"
	. "github.com/enetx/g"
)

func main() {
	// Set up scanner to read user input from the console
	scanner := bufio.NewScanner(os.Stdin)

	// Define the FSM with states, transitions, guards, and callbacks
	fsm := NewFSM("start").
		// Transition from start to ask_name
		Transition("start", "next", "ask_name").
		// After name input, go to ask_age
		Transition("ask_name", "input", "ask_age").
		// From ask_age to ask_lang only if the input is a valid number
		TransitionWhen("ask_age", "input", "ask_lang", func(ctx *Context) bool {
			return ctx.Input.(String).ToInt().IsOk()
		}).
		// On invalid age input, stay in ask_age
		Transition("ask_age", "invalid", "ask_age").
		// After language input, go to confirm
		Transition("ask_lang", "input", "confirm").
		// User confirms data with "yes" or restarts with "no"
		Transition("confirm", "yes", "done").
		Transition("confirm", "no", "ask_name").
		// State entry callbacks:
		// Ask name and record session start time
		OnEnter("ask_name", func(ctx *Context) error {
			ctx.Values.Set("started_at", time.Now().Format(time.RFC822))
			Println("Hi! What's your name?")
			return nil
		}).
		// Ask age, using previously entered name
		OnEnter("ask_age", func(ctx *Context) error {
			name := ctx.Data.Get("name").UnwrapOr(String("<anon>"))
			Println("Nice to meet you, {}! How old are you?", name)
			return nil
		}).
		// Ask about programming language
		OnEnter("ask_lang", func(*Context) error {
			Println("Cool! What programming language do you use most?")
			return nil
		}).
		// Log exit from ask_lang
		OnExit("ask_lang", func(*Context) error {
			Println("!!! Finished language input !!!")
			return nil
		}).
		// Display confirmation screen with entered data
		OnEnter("confirm", func(ctx *Context) error {
			name := ctx.Data.Get("name").UnwrapOr(String("<anon>"))
			age := ctx.Data.Get("age").UnwrapOr(String("<unknown>"))
			lang := ctx.Data.Get("lang").UnwrapOr(String("<none>"))
			Println("\nPlease confirm:\n- Name: {}\n- Age: {}\n- Language: {}(y/n): ", name, age, lang)
			return nil
		}).
		// Final message when done
		OnEnter("done", func(ctx *Context) error {
			name := ctx.Data.Get("name").UnwrapOr(String("<anon>"))
			age := ctx.Data.Get("age").UnwrapOr(String("<unknown>"))
			lang := ctx.Data.Get("lang").UnwrapOr(String("<none>"))
			started := ctx.Values.Get("started_at").UnwrapOrDefault()
			Println("\nThank you, {}! Data saved.\n- Age: {}\n- Language: {}\nStarted at: {}", name, age, lang, started)
			return nil
		})

	fsm.OnTransition(func(from, to State, event Event, _ *Context) error {
		// Called on every successful transition:
		// - after OnExit callbacks of `from`
		// - before OnEnter callbacks of `to`
		// You can use this hook to:
		// - log transitions
		// - validate consistency between states
		// - collect metrics or analytics
		// - trigger side effects like saving to DB
		Println("[transition] {} → {} via event {}", from, to, event)
		return nil
	})

	// Get FSM context and start the flow
	ctx := fsm.Context()
	fsm.Trigger("next")

	// Main input loop until FSM reaches "done"
	for fsm.Current() != "done" {
		Print("→ ")
		if !scanner.Scan() {
			break
		}

		input := String(scanner.Text()).Trim()
		ctx.Input = input

		switch fsm.Current() {
		// Handle name input
		case "ask_name":
			ctx.Data.Set("name", input)
			fsm.Trigger("input")
			// Validate age input and decide transition
		case "ask_age":
			age := input.ToInt()
			switch {
			case age.IsOk():
				ctx.Data.Set("age", input)
				fsm.Trigger("input")
			case age.IsErr():
				Println("Please enter a valid number.")
				fsm.Trigger("invalid")
			}
		// Handle language input
		case "ask_lang":
			ctx.Data.Set("lang", input)
			fsm.Trigger("input")
			// Confirm user data
		case "confirm":
			if input.Lower().Eq("y") || input.Lower().Eq("yes") {
				fsm.Trigger("yes")
			} else {
				fsm.Trigger("no")
			}
		}
	}

	// Print the history of all visited states so far.
	// Useful for debugging, logging, or audit purposes.
	fsm.History().Println()
}
