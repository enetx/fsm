package fsm_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	. "github.com/enetx/fsm"
	. "github.com/enetx/g"
)

func assertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func assertTrue(t *testing.T, cond bool) {
	t.Helper()
	if !cond {
		t.Fatalf("expected true, got false")
	}
}

func assertFalse(t *testing.T, cond bool) {
	t.Helper()
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
			return ctx.Meta.Get("ok").UnwrapOr(false).(bool)
		}).
		OnEnter("done", func(*Context) error {
			called = true
			return nil
		})

	ctx := testFSM.Context()
	ctx.Meta.Set("ok", false)
	assertError(t, testFSM.Trigger("go"))
	assertFalse(t, called)

	ctx.Meta.Set("ok", true)
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

func TestFSM_OnTransition(t *testing.T) {
	var called bool
	var from, to State
	var event Event

	fsm := NewFSM("a").
		Transition("a", "go", "b").
		OnTransition(func(f, t State, e Event, _ *Context) error {
			called = true
			from, to, event = f, t, e
			return nil
		})

	assertNoError(t, fsm.Trigger("go"))
	assertTrue(t, called)
	assertEqual(t, from, "a")
	assertEqual(t, to, "b")
	assertEqual(t, event, "go")
}

func TestFSM_History(t *testing.T) {
	fsm := NewFSM("x").
		Transition("x", "next", "y").
		Transition("y", "next", "z")

	assertNoError(t, fsm.Trigger("next"))
	assertNoError(t, fsm.Trigger("next"))

	h := fsm.History()
	assertEqual(t, h.Len(), 3)
	assertEqual(t, h[0], State("x"))
	assertEqual(t, h[1], State("y"))
	assertEqual(t, h[2], State("z"))
}

func TestFSM_OnEnterError(t *testing.T) {
	fsm := NewFSM("s").
		Transition("s", "go", "t").
		OnEnter("t", func(*Context) error {
			return fmt.Errorf("fail")
		})

	err := fsm.Trigger("go")
	assertError(t, err)
}

func TestFSM_InvalidEvent(t *testing.T) {
	fsm := NewFSM("only")
	err := fsm.Trigger("nope")
	assertError(t, err)
}

func TestFSM_Clone(t *testing.T) {
	template := NewFSM("a").
		Transition("a", "next", "b")

	fsm1 := template.Clone()
	fsm2 := template.Clone()

	assertNoError(t, fsm1.Trigger("next"))

	// Verify that fsm1's state changed, but fsm2 and the template remain unchanged.
	assertEqual(t, fsm1.Current(), State("b"))
	assertEqual(t, fsm2.Current(), State("a"))
	assertEqual(t, template.Current(), State("a"))
}

func TestFSM_SetState(t *testing.T) {
	enterCalled := false
	exitCalled := false

	fsm := NewFSM("a").
		OnEnter("b", func(ctx *Context) error { enterCalled = true; return nil }).
		OnExit("a", func(ctx *Context) error { exitCalled = true; return nil })

	fsm.SetState("b")

	// SetState should change the state without triggering callbacks.
	assertEqual(t, fsm.Current(), State("b"))
	assertFalse(t, enterCalled)
	assertFalse(t, exitCalled)
}

func TestFSM_CallEnter(t *testing.T) {
	enterCalled := false
	fsm := NewFSM("a").
		OnEnter("a", func(ctx *Context) error { enterCalled = true; return nil })

	assertNoError(t, fsm.CallEnter("a"))

	// CallEnter should trigger the callback but not change the state or history.
	assertTrue(t, enterCalled)
	assertEqual(t, fsm.Current(), State("a"))
	assertEqual(t, fsm.History().Len(), 1)
}

func TestFSM_Serialization(t *testing.T) {
	template := NewFSM("a").
		Transition("a", "next", "b")

	fsm := template.Clone()
	fsm.Context().Data.Set("user_id", 123)
	assertNoError(t, fsm.Trigger("next"))

	// Marshal the FSM to JSON.
	jsonData, err := json.Marshal(fsm)
	assertNoError(t, err)

	// Create a new FSM and unmarshal the state into it.
	newFSM := template.Clone()
	err = json.Unmarshal(jsonData, newFSM)
	assertNoError(t, err)

	// Verify the state was restored correctly.
	assertEqual(t, newFSM.Current(), State("b"))
	assertEqual(t, newFSM.History().Len(), 2)
	assertEqual(t, newFSM.History()[1], State("b"))
	assertEqual(t, newFSM.Context().Data.Get("user_id").Unwrap().(float64), 123)
}

func TestFSM_SerializationUnknownState(t *testing.T) {
	fsm := NewFSM("a").Transition("a", "next", "b")
	invalidJSON := `{"current": "unknown_state", "history": ["a"]}`

	err := json.Unmarshal([]byte(invalidJSON), fsm)
	assertError(t, err)
	assertTrue(t, strings.Contains(err.Error(), "unknown state"))
}

func TestFSM_PanicRecovery(t *testing.T) {
	fsm := NewFSM("a").
		Transition("a", "go", "b").
		OnEnter("b", func(ctx *Context) error {
			panic("something went wrong")
		})

	err := fsm.Trigger("go")
	assertError(t, err)
	assertTrue(t, strings.Contains(err.Error(), "panic"))
}

func TestFSM_States(t *testing.T) {
	fsm := NewFSM("a").
		Transition("a", "to_b", "b").
		Transition("b", "to_c", "c").
		Transition("b", "to_a", "a")

	states := fsm.States()
	expected := SetOf[State]("a", "b", "c")

	assertEqual(t, SetOf(states...).Len(), expected.Len())
	assertTrue(t, SetOf(states...).Eq(expected))
}
