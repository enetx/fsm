package main

import (
	"log"

	"github.com/enetx/fsm"
)

// Logger defines a minimal log interface (compatible with log.Logger, zap.SugaredLogger, etc).
type Logger interface {
	Printf(format string, args ...any)
}

// attachLogger adds logging callbacks to the FSM for debugging.
// It logs transitions, state entries and exits.
func attachLogger(f *fsm.FSM, l Logger) {
	f.OnTransition(func(from, to fsm.State, event fsm.Event, _ *fsm.Context) error {
		l.Printf("[FSM] %s --(%s)--> %s", from, event, to)
		return nil
	})

	f.States().Iter().ForEach(func(state fsm.State) {
		f.OnEnter(state, func(state fsm.State) fsm.Callback {
			return func(*fsm.Context) error {
				l.Printf("[ENTER] %s", state)
				return nil
			}
		}(state))

		f.OnExit(state, func(state fsm.State) fsm.Callback {
			return func(*fsm.Context) error {
				l.Printf("[EXIT]  %s", state)
				return nil
			}
		}(state))
	})
}

func main() {
	f := fsm.New("idle").
		Transition("idle", "start", "active").
		Transition("active", "reset", "idle")

	// Attach standard logger (prints to stdout)
	attachLogger(f, log.Default())

	f.Trigger("start")
	f.Trigger("reset")

	// Output:
	// [EXIT]  idle
	// [FSM] idle --(start)--> active
	// [ENTER] active
	// [EXIT]  active
	// [FSM] active --(reset)--> idle
	// [ENTER] idle
}
