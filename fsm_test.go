package fsm_test

import (
	"testing"

	. "github.com/enetx/fsm"
	. "github.com/enetx/g"
)

func assertEqual[T comparable](t *testing.T, got, want T) {
	if got != want {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func assertNoError(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertError(t *testing.T, err error) {
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func assertTrue(t *testing.T, cond bool) {
	if !cond {
		t.Fatalf("expected true, got false")
	}
}

func assertFalse(t *testing.T, cond bool) {
	if cond {
		t.Fatalf("expected false, got true")
	}
}

func TestFSM_BasicTransition(t *testing.T) {
	testFSM := NewFSM("idle").
		Transition("idle", "start", "running").
		Transition("running", "stop", "idle")

	assertEqual(t, testFSM.Current(), State("idle"))
	assertNoError(t, testFSM.Trigger("start"))
	assertEqual(t, testFSM.Current(), State("running"))
	assertNoError(t, testFSM.Trigger("stop"))
	assertEqual(t, testFSM.Current(), State("idle"))
}

func TestFSM_Guard(t *testing.T) {
	called := false
	testFSM := NewFSM("ready").
		TransitionWhen("ready", "go", "done", func(ctx *Context) bool {
			return ctx.Values.Get("ok").UnwrapOr(false).(bool)
		}).
		OnEnter("done", func(*Context) error {
			called = true
			return nil
		})

	ctx := testFSM.Context()
	ctx.Values.Set("ok", false)
	assertError(t, testFSM.Trigger("go"))
	assertFalse(t, called)

	ctx.Values.Set("ok", true)
	assertNoError(t, testFSM.Trigger("go"))
	assertTrue(t, called)
	assertEqual(t, testFSM.Current(), State("done"))
}

func TestFSM_OnEnterExit(t *testing.T) {
	order := Slice[String]{}

	testFSM := NewFSM("off").
		Transition("off", "toggle", "on").
		Transition("on", "toggle", "off").
		OnExit("off", func(*Context) error {
			order.Push("exit_off")
			return nil
		}).
		OnEnter("on", func(*Context) error {
			order.Push("enter_on")
			return nil
		})

	assertNoError(t, testFSM.Trigger("toggle"))
	if !order.Eq(SliceOf[String]("exit_off", "enter_on")) {
		t.Fatalf("expected order [exit_off enter_on], got %v", order)
	}
}

func TestFSM_Reset(t *testing.T) {
	fsm := NewFSM("a").
		Transition("a", "next", "b")

	fsm.Context().Data.Set("x", 123)
	assertNoError(t, fsm.Trigger("next"))
	assertEqual(t, fsm.Current(), State("b"))
	assertEqual(t, fsm.Context().Data.Get("x").Unwrap(), 123)

	fsm.Reset()
	assertEqual(t, fsm.Current(), State("a"))
	assertTrue(t, fsm.Context().Data.Get("x").IsNone())
}
