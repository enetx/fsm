package main

import (
	"bufio"
	"errors"
	"os"
	"time"

	. "github.com/enetx/fsm"
	"github.com/enetx/g"
)

func main() {
	// Set up scanner to read user input from the console
	scanner := bufio.NewScanner(os.Stdin)

	// Define the FSM with states, transitions, guards, and callbacks
	fsm := New("start").
		// Transition from start to ask_name
		Transition("start", "next", "ask_name").
		// After name input, go to ask_age
		Transition("ask_name", "input", "ask_age").
		// From ask_age to ask_lang only if the input is a valid number
		TransitionWhen("ask_age", "input", "ask_lang", func(ctx *Context) bool {
			return ctx.Input.(g.String).TryInt().IsOk()
		}).
		// After language input, go to confirm
		Transition("ask_lang", "input", "confirm").
		// User confirms data with "yes"
		TransitionWhen("confirm", "confirm_input", "done", func(ctx *Context) bool {
			input := ctx.Input.(g.String).Lower()
			return input.Eq("y") || input.Eq("yes")
		}).
		// User confirms data with "no"
		TransitionWhen("confirm", "confirm_input", "ask_name", func(ctx *Context) bool {
			input := ctx.Input.(g.String).Lower()
			return input.Eq("n") || input.Eq("no")
		}).
		// State entry callbacks:
		// Ask name and record session start time
		OnEnter("ask_name", func(ctx *Context) error {
			ctx.Meta.Insert("started_at", time.Now().Format(time.RFC822))
			g.Println("Hi! What's your name?")
			return nil
		}).
		// Ask age, using previously entered name
		OnEnter("ask_age", func(ctx *Context) error {
			name := ctx.Data.Get("name").UnwrapOr("<anon>")
			g.Println("Nice to meet you, {}! How old are you?", name)
			return nil
		}).
		// Ask about programming language
		OnEnter("ask_lang", func(*Context) error {
			g.Println("Cool! What programming language do you use most?")
			return nil
		}).
		// Log exit from ask_lang
		OnExit("ask_lang", func(*Context) error {
			g.Println("!!! Finished language input !!!")
			return nil
		}).
		// Display confirmation screen with entered data
		OnEnter("confirm", func(ctx *Context) error {
			name := ctx.Data.Get("name").UnwrapOr("<anon>")
			age := ctx.Data.Get("age").UnwrapOr("<unknown>")
			lang := ctx.Data.Get("lang").UnwrapOr("<none>")
			g.Println("\nPlease confirm:\n- Name: {}\n- Age: {}\n- Language: {}(y/n): ", name, age, lang)
			return nil
		}).
		// Final message when done
		OnEnter("done", func(ctx *Context) error {
			name := ctx.Data.Get("name").UnwrapOr("<anon>")
			age := ctx.Data.Get("age").UnwrapOr("<unknown>")
			lang := ctx.Data.Get("lang").UnwrapOr("<none>")
			started := ctx.Meta.Get("started_at").UnwrapOrDefault()

			g.Println(
				"\nThank you, {}! Data saved.\n- Age: {}\n- Language: {}\nStarted at: {}",
				name,
				age,
				lang,
				started,
			)

			return nil
		})

	fsm.OnTransition(func(from, to State, event Event, ctx *Context) error {
		g.Println("[transition] {} → {} via event {}", from, to, event)

		if event == "input" {
			switch from {
			case "ask_name":
				ctx.Data.Insert("name", ctx.Input)
			case "ask_age":
				ctx.Data.Insert("age", ctx.Input)
			case "ask_lang":
				ctx.Data.Insert("lang", ctx.Input)
			}
		}

		return nil
	})

	// Get FSM context and start the flow
	fsm.Trigger("next")

	// Main input loop until FSM reaches "done"
	for fsm.Current() != "done" {
		g.Print("→ ")
		if !scanner.Scan() {
			break
		}

		input := g.String(scanner.Text()).Trim()

		var err error

		switch fsm.Current() {
		case "ask_name", "ask_age", "ask_lang":
			err = fsm.Trigger("input", input)
		case "confirm":
			err = fsm.Trigger("confirm_input", input)
		}

		if err != nil {
			var invTransErr *ErrInvalidTransition

			if errors.As(err, &invTransErr) {
				switch invTransErr.From {
				case "ask_age":
					g.Println("Please enter a valid number.")
				case "confirm":
					g.Println("Please enter 'y' (yes) or 'n' (no).")
				}

				continue
			}

			g.Println("An unexpected error occurred: {}", err)
		}
	}

	// Print the history of all visited states so far.
	// Useful for debugging, logging, or audit purposes.
	fsm.History().Println()
}
