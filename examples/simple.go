package main

import (
	"bufio"
	"os"

	. "github.com/enetx/fsm"
	"github.com/enetx/g"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	fsm := New("start").
		Transition("start", "next", "ask_name").
		Transition("ask_name", "input", "ask_age").
		Transition("ask_age", "input", "ask_lang").
		Transition("ask_lang", "input", "done").
		OnEnter("ask_name", func(*Context) error {
			g.Println("Hi! What's your name?")
			return nil
		}).
		OnEnter("ask_age", func(ctx *Context) error {
			name := ctx.Data.Get("name").UnwrapOr("<anon>")
			g.Println("Nice to meet you, {}! How old are you?", name)
			return nil
		}).
		OnEnter("ask_lang", func(*Context) error {
			g.Println("Cool! What programming language do you use most?")
			return nil
		}).
		OnEnter("done", func(ctx *Context) error {
			name := ctx.Data.Get("name").UnwrapOr("<anon>")
			age := ctx.Data.Get("age").UnwrapOr("<unknown>")
			lang := ctx.Data.Get("lang").UnwrapOr("<none>")
			g.Println("\nSummary:\n- Name: {}\n- Age: {}\n- Language: {}", name, age, lang)
			return nil
		})

	ctx := fsm.Context()
	fsm.Trigger("next")

	for fsm.Current() != "done" {
		g.Print("→ ")

		if !scanner.Scan() {
			break
		}

		input := g.String(scanner.Text()).Trim()

		switch fsm.Current() {
		case "ask_name":
			ctx.Data.Insert("name", input)
		case "ask_age":
			ctx.Data.Insert("age", input)
		case "ask_lang":
			ctx.Data.Insert("lang", input)
		}

		fsm.Trigger("input")
	}
}
