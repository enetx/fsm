package fsm_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	. "github.com/enetx/fsm"
	"github.com/enetx/g"
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
	testFSM := New("idle").
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
	testFSM := New("ready").
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
	order := g.Slice[g.String]{}

	testFSM := New("off").
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
	if !order.Eq(g.SliceOf[g.String]("exit_off", "enter_on")) {
		t.Fatalf("expected order [exit_off enter_on], got %v", order)
	}
}

func TestFSM_Reset(t *testing.T) {
	fsm := New("a").
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

	fsm := New("a").
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
	fsm := New("x").
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
	fsm := New("s").
		Transition("s", "go", "t").
		OnEnter("t", func(*Context) error {
			return fmt.Errorf("fail")
		})

	err := fsm.Trigger("go")
	assertError(t, err)
}

func TestFSM_InvalidEvent(t *testing.T) {
	fsm := New("only")
	err := fsm.Trigger("nope")
	assertError(t, err)
}

func TestFSM_Clone(t *testing.T) {
	template := New("a").
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

	fsm := New("a").
		OnEnter("b", func(*Context) error { enterCalled = true; return nil }).
		OnExit("a", func(*Context) error { exitCalled = true; return nil })

	fsm.SetState("b")

	// SetState should change the state without triggering callbacks.
	assertEqual(t, fsm.Current(), State("b"))
	assertFalse(t, enterCalled)
	assertFalse(t, exitCalled)
}

func TestFSM_CallEnter(t *testing.T) {
	enterCalled := false
	fsm := New("a").
		OnEnter("a", func(*Context) error { enterCalled = true; return nil })

	assertNoError(t, fsm.CallEnter("a"))

	// CallEnter should trigger the callback but not change the state or history.
	assertTrue(t, enterCalled)
	assertEqual(t, fsm.Current(), State("a"))
	assertEqual(t, fsm.History().Len(), 1)
}

func TestFSM_Serialization(t *testing.T) {
	template := New("a").
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
	fsm := New("a").Transition("a", "next", "b")
	invalidJSON := `{"current": "unknown_state", "history": ["a"]}`

	err := json.Unmarshal([]byte(invalidJSON), fsm)
	assertError(t, err)
	assertTrue(t, strings.Contains(err.Error(), "unknown state"))
}

func TestFSM_PanicRecovery(t *testing.T) {
	fsm := New("a").
		Transition("a", "go", "b").
		OnEnter("b", func(*Context) error {
			panic("something went wrong")
		})

	err := fsm.Trigger("go")
	assertError(t, err)
	assertTrue(t, strings.Contains(err.Error(), "panic"))
}

func TestFSM_States(t *testing.T) {
	fsm := New("a").
		Transition("a", "to_b", "b").
		Transition("b", "to_c", "c").
		Transition("b", "to_a", "a")

	states := fsm.States()
	expected := g.SetOf[State]("a", "b", "c")

	assertEqual(t, g.SetOf(states...).Len(), expected.Len())
	assertTrue(t, g.SetOf(states...).Eq(expected))
}

func TestFSM_TriggerInput(t *testing.T) {
	var received any

	fsm := New("x").
		Transition("x", "go", "y").
		OnEnter("y", func(ctx *Context) error {
			received = ctx.Input
			return nil
		})

	assertNoError(t, fsm.Trigger("go", "my_input"))
	assertEqual(t, received, "my_input")
}

func TestFSM_OnExitError(t *testing.T) {
	fsm := New("a").
		Transition("a", "go", "b").
		OnExit("a", func(*Context) error {
			return fmt.Errorf("exit error")
		})

	err := fsm.Trigger("go")
	assertError(t, err)
	assertTrue(t, strings.Contains(err.Error(), "exit error"))
}

func TestFSM_OnTransitionError(t *testing.T) {
	fsm := New("a").
		Transition("a", "go", "b").
		OnTransition(func(_, _ State, _ Event, _ *Context) error { return fmt.Errorf("hook failed") })

	err := fsm.Trigger("go")
	assertError(t, err)
	assertTrue(t, strings.Contains(err.Error(), "hook failed"))
}

// Test error types coverage
func TestFSM_ErrorTypes(t *testing.T) {
	// Test ErrInvalidTransition.Error()
	err := &ErrInvalidTransition{From: "a", Event: "invalid"}
	expected := `fsm: no matching transition for event "invalid" from state "a"`
	assertEqual(t, err.Error(), expected)

	// Test ErrCallback.Error() with state
	callbackErr := &ErrCallback{HookType: "OnEnter", State: "test", Err: fmt.Errorf("test error")}
	expected = `fsm: error in OnEnter callback for state "test": test error`
	assertEqual(t, callbackErr.Error(), expected)

	// Test ErrCallback.Error() without state
	callbackErr = &ErrCallback{HookType: "OnTransition", State: "", Err: fmt.Errorf("hook error")}
	expected = `fsm: error in OnTransition hook: hook error`
	assertEqual(t, callbackErr.Error(), expected)

	// Test ErrCallback.Unwrap()
	originalErr := fmt.Errorf("original")
	callbackErr = &ErrCallback{HookType: "OnEnter", State: "test", Err: originalErr}
	assertEqual(t, callbackErr.Unwrap(), originalErr)
}

// Test FSM.Sync() method
func TestFSM_Sync(t *testing.T) {
	fsm := New("idle").Transition("idle", "start", "running")
	syncFSM := fsm.Sync()

	// Test all SyncFSM methods
	assertEqual(t, syncFSM.Current(), State("idle"))
	assertNoError(t, syncFSM.Trigger("start"))
	assertEqual(t, syncFSM.Current(), State("running"))

	// Test Context
	ctx := syncFSM.Context()
	ctx.Data.Set("test", "value")
	assertEqual(t, syncFSM.Context().Data.Get("test").Unwrap(), "value")

	// Test SetState
	syncFSM.SetState("idle")
	assertEqual(t, syncFSM.Current(), State("idle"))

	// Test History
	history := syncFSM.History()
	assertTrue(t, history.Len() > 0)

	// Test States
	states := syncFSM.States()
	assertTrue(t, states.Contains("idle"))
	assertTrue(t, states.Contains("running"))

	// Test Reset
	syncFSM.Reset()
	assertEqual(t, syncFSM.Current(), State("idle"))
	assertEqual(t, syncFSM.History().Len(), 1)
}

// Test SyncFSM CallEnter
func TestSyncFSM_CallEnter(t *testing.T) {
	enterCalled := false
	fsm := New("a").OnEnter("a", func(*Context) error {
		enterCalled = true
		return nil
	})
	syncFSM := fsm.Sync()

	assertNoError(t, syncFSM.CallEnter("a"))
	assertTrue(t, enterCalled)
}

// Test SyncFSM CallEnter with error
func TestSyncFSM_CallEnterError(t *testing.T) {
	fsm := New("a").OnEnter("a", func(*Context) error {
		return fmt.Errorf("enter error")
	})
	syncFSM := fsm.Sync()

	err := syncFSM.CallEnter("a")
	assertError(t, err)
	assertTrue(t, strings.Contains(err.Error(), "enter error"))
}

// Test SyncFSM serialization
func TestSyncFSM_JSON(t *testing.T) {
	template := New("a").Transition("a", "next", "b")
	syncFSM := template.Sync()
	syncFSM.Context().Data.Set("key", "value")
	assertNoError(t, syncFSM.Trigger("next"))

	// Test MarshalJSON
	jsonData, err := syncFSM.MarshalJSON()
	assertNoError(t, err)

	// Test UnmarshalJSON
	newSyncFSM := template.Sync()
	err = newSyncFSM.UnmarshalJSON(jsonData)
	assertNoError(t, err)

	assertEqual(t, newSyncFSM.Current(), State("b"))
	assertEqual(t, newSyncFSM.Context().Data.Get("key").Unwrap(), "value")
}

// Test SyncFSM ToDOT
func TestSyncFSM_ToDOT(t *testing.T) {
	fsm := New("a").Transition("a", "go", "b")
	syncFSM := fsm.Sync()

	dot := syncFSM.ToDOT()
	assertTrue(t, dot.Contains("digraph FSM"))
	assertTrue(t, dot.Contains(`"a"`))
	assertTrue(t, dot.Contains(`"b"`))
}

// Test FSM.ToDOT() method
func TestFSM_ToDOT(t *testing.T) {
	fsm := New("idle").
		Transition("idle", "start", "running").
		Transition("running", "stop", "idle").
		TransitionWhen("running", "pause", "paused", func(*Context) bool { return true }).
		OnEnter("running", func(*Context) error { return nil }).
		OnExit("idle", func(*Context) error { return nil })

	dot := fsm.ToDOT()
	assertTrue(t, dot.Contains("digraph FSM"))
	assertTrue(t, dot.Contains(`"idle"`))
	assertTrue(t, dot.Contains(`"running"`))
	assertTrue(t, dot.Contains(`"paused"`))
	assertTrue(t, dot.Contains("initial"))
	assertTrue(t, dot.Contains("Legend"))
	assertTrue(t, dot.Contains("(guarded)"))
	assertTrue(t, dot.Contains("OnEnter"))
	assertTrue(t, dot.Contains("OnExit"))
}

// Test CallEnter edge cases
func TestFSM_CallEnterNonExistentState(t *testing.T) {
	fsm := New("a")
	err := fsm.CallEnter("nonexistent")
	// CallEnter should succeed even for non-existent states (no callbacks to run)
	assertNoError(t, err)
}

// Test JSON unmarshaling edge cases
func TestFSM_UnmarshalJSONInvalidData(t *testing.T) {
	fsm := New("a")

	// Test invalid JSON
	err := fsm.UnmarshalJSON([]byte("invalid json"))
	assertError(t, err)

	// Test missing fields
	err = fsm.UnmarshalJSON([]byte("{}"))
	assertError(t, err)
}

// Test Trigger edge cases - multiple valid transitions (ambiguous)
func TestFSM_AmbiguousTransition(t *testing.T) {
	fsm := New("a").
		TransitionWhen("a", "go", "b", func(*Context) bool { return true }).
		TransitionWhen("a", "go", "c", func(*Context) bool { return true })

	err := fsm.Trigger("go")
	assertError(t, err)
	assertTrue(t, strings.Contains(err.Error(), "ambiguous transition"))
}

// Test remaining coverage in Trigger method
func TestFSM_TriggerWithInput(t *testing.T) {
	var received any
	fsm := New("a").
		Transition("a", "go", "b").
		OnEnter("b", func(ctx *Context) error {
			received = ctx.Input
			return nil
		})

	// Test with single input (only first input is stored)
	assertNoError(t, fsm.Trigger("go", "input1", "input2", 123))
	assertEqual(t, received, "input1")
}

// Test OnTransition hook panic recovery
func TestFSM_OnTransitionPanic(t *testing.T) {
	fsm := New("a").
		Transition("a", "go", "b").
		OnTransition(func(_, _ State, _ Event, _ *Context) error {
			panic("hook panic")
		})

	err := fsm.Trigger("go")
	assertError(t, err)
	assertTrue(t, strings.Contains(err.Error(), "panic"))
	assertTrue(t, strings.Contains(err.Error(), "OnTransition"))
}

// Test UnmarshalJSON with unknown state in history
func TestFSM_UnmarshalJSONUnknownStateInHistory(t *testing.T) {
	fsm := New("a").Transition("a", "next", "b")

	// JSON with unknown state in history
	invalidJSON := `{"current": "a", "history": ["a", "unknown_state"], "context": {"data": {}, "meta": {}}}`

	err := fsm.UnmarshalJSON([]byte(invalidJSON))
	assertError(t, err)
	assertTrue(t, strings.Contains(err.Error(), "unknown state"))
	assertTrue(t, strings.Contains(err.Error(), "unknown_state"))
}
