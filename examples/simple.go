package main

import (
	"bufio"
	"os"

	. "github.com/enetx/fsm"
	. "github.com/enetx/g"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	fsm := NewFSM("start").
		Transition("start", "next", "ask_name").
		Transition("ask_name", "input", "ask_age").
		Transition("ask_age", "input", "ask_lang").
		Transition("ask_lang", "input", "done").
		OnEnter("ask_name", func(*Context) error {
			Println("Hi! What's your name?")
			return nil
		}).
		OnEnter("ask_age", func(ctx *Context) error {
			name := ctx.Data.Get("name").UnwrapOr("<anon>")
			Println("Nice to meet you, {}! How old are you?", name)
			return nil
		}).
		OnEnter("ask_lang", func(*Context) error {
			Println("Cool! What programming language do you use most?")
			return nil
		}).
		OnEnter("done", func(ctx *Context) error {
			name := ctx.Data.Get("name").UnwrapOr("<anon>")
			age := ctx.Data.Get("age").UnwrapOr("<unknown>")
			lang := ctx.Data.Get("lang").UnwrapOr("<none>")
			Println("\nSummary:\n- Name: {}\n- Age: {}\n- Language: {}", name, age, lang)
			return nil
		})

	ctx := fsm.Context()
	fsm.Trigger("next")

	for fsm.Current() != "done" {
		Print("→ ")

		if !scanner.Scan() {
			break
		}

		input := String(scanner.Text()).Trim()
		ctx.Input = input

		switch fsm.Current() {
		case "ask_name":
			ctx.Data.Set("name", input)
		case "ask_age":
			ctx.Data.Set("age", input)
		case "ask_lang":
			ctx.Data.Set("lang", input)
		}

		fsm.Trigger("input")
	}
}
