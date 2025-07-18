package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/enetx/fsm"
)

func main() {
	// 1. Define the FSM for an article's lifecycle
	articleFSM := fsm.New("draft").
		// An author submits a draft for review
		Transition("draft", "submit_for_review", "in_review").
		// An editor rejects the article, returning it to draft status
		Transition("in_review", "reject", "draft").
		// An editor approves the article
		Transition("in_review", "approve", "approved").
		// An admin publishes the approved article
		Transition("approved", "publish", "published").
		// An article can be archived from any state except draft
		Transition("in_review", "archive", "archived").
		Transition("approved", "archive", "archived").
		Transition("published", "archive", "archived").
		// Callbacks to simulate real-world actions
		OnEnter("in_review", func(*fsm.Context) error {
			fmt.Println("-> Article submitted for review. Notifying editors...")
			// Simulate a long-running operation, like sending an email
			time.Sleep(5 * time.Second)
			fmt.Println("-> Notifications sent.")
			return nil
		}).
		OnEnter("published", func(ctx *fsm.Context) error {
			// Set some metadata
			ctx.Meta.Set("published_at", time.Now().UTC())
			fmt.Println("-> ARTICLE PUBLISHED! Updating website...")
			return nil
		}).
		OnEnter("archived", func(*fsm.Context) error {
			fmt.Println("-> Article has been archived. Accessible to admins only.")
			return nil
		})

	// 2. Wrap the FSM in its thread-safe version. THIS IS THE KEY STEP!
	// All HTTP requests will work with this single, shared instance.
	syncFSM := articleFSM.Sync()

	// 3. Set up the web server
	// This handler will accept events and change the FSM's state
	http.HandleFunc("/action", func(w http.ResponseWriter, r *http.Request) {
		// Get the event from the URL query, e.g., /action?event=submit_for_review
		event := r.URL.Query().Get("event")
		if event == "" {
			http.Error(w, "event parameter is required", http.StatusBadRequest)
			return
		}

		fmt.Printf("[HTTP] Received event: %s\n", event)

		// Attempt the transition. concurrentFSM handles all the locking internally.
		err := syncFSM.Trigger(fsm.Event(event))
		if err != nil {
			fmt.Printf("[HTTP] Transition error: %v\n", err)
			// Return the error to the user if the transition is invalid
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		newState := syncFSM.Current()
		fmt.Printf("[HTTP] Transition successful. New state: %s\n", newState)
		fmt.Fprintf(w, "Action successful. New state: %s\n", newState)
	})

	// This handler simply shows the current state
	http.HandleFunc("/status", func(w http.ResponseWriter, _ *http.Request) {
		currentState := syncFSM.Current()
		fmt.Fprintf(w, "Current article state: %s\n", currentState)
	})

	fmt.Println("Server starting on http://localhost:8080")
	fmt.Println("Example requests:")
	fmt.Println("  curl http://localhost:8080/status")
	fmt.Println("  curl -X POST http://localhost:8080/action?event=submit_for_review")
	fmt.Println("  curl -X POST http://localhost:8080/action?event=approve")

	http.ListenAndServe(":8080", nil)
}
