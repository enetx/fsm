package main

import (
	"github.com/enetx/fsm"
	. "github.com/enetx/g"
)

const (
	start   fsm.State = "start"
	name    fsm.State = "ask_name"
	age     fsm.State = "ask_age"
	lang    fsm.State = "ask_lang"
	confirm fsm.State = "confirm"
	done    fsm.State = "done"
)

const (
	input fsm.Event = "input"
	next  fsm.Event = "next"
	ok    fsm.Event = "confirm_input"
)

func main() {
	f := fsm.New(start).
		Transition(start, next, name).
		Transition(name, input, age).
		Transition(age, input, lang).
		TransitionWhen(age, input, lang, func(ctx *fsm.Context) bool { return ctx.Data.Contains("valid_age") }).
		Transition(lang, input, confirm).
		TransitionWhen(confirm, ok, done, func(*fsm.Context) bool { return true }).
		TransitionWhen(confirm, ok, name, func(*fsm.Context) bool { return false }).
		OnEnter(name, func(*fsm.Context) error { Println("Entered ask_name"); return nil }).
		OnExit(name, func(*fsm.Context) error { Println("Leaving ask_name"); return nil }).
		OnEnter(done, func(*fsm.Context) error { Println("Process done"); return nil })

	// https://edotor.net/
	f.ToDOT().Println()
}
