package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/enetx/fsm"
	"github.com/enetx/g"
)

// --- Step 1: Define the FSM's Vocabulary (States and Events) ---
// First, we define all possible states and events as typed constants. This is a
// crucial best practice for FSMs. It prevents typos (from using "magic strings")
// that can lead to runtime errors and makes the state machine's "API" — its
// valid states and triggers — clear and self-documenting.

const (
	StateIdle      fsm.State = "Idle"      // The NPC is calm, outside of combat.
	StateAttacking fsm.State = "Attacking" // The NPC is actively fighting.
	StateDefending fsm.State = "Defending" // The NPC is hurt and taking a defensive stance.
	StateEnraged   fsm.State = "Enraged"   // The NPC is near death and fighting desperately.
	StateDefeated  fsm.State = "Defeated"  // The NPC has been defeated.
)

const (
	EventEngage     fsm.Event = "Engage"     // Triggers the start of the battle.
	EventTakeDamage fsm.Event = "TakeDamage" // Triggers when the NPC is hit. This is the main driver of state changes.
	EventRecover    fsm.Event = "Recover"    // Triggers after the NPC has defended, allowing it to attack again.
	EventDefeated   fsm.Event = "BeDefeated" // Triggers when the NPC's HP drops to 0 or below.
)

// --- Step 2: Define Our Data Structures ---
// Next, we define a simple struct to hold the state for our combatants.
// We use the `g.Int` type for HP to seamlessly integrate with the g library's utility methods.
type Character struct {
	Name string
	HP   g.Int
}

// --- Step 3: Define the NPC's "Intelligence" (Guard Functions) ---
// Here we define our Guard functions. A Guard is a function that returns `true` or `false`,
// determining if a transition is allowed to happen. This is where the "intelligence" of our
// NPC comes from. Both guards access the FSM's Context to get the NPC's current HP and make a decision.

// isBadlyWounded checks if the NPC's HP is in the "danger zone", prompting a defensive reaction.
func isBadlyWounded(ctx *fsm.Context) bool {
	// We retrieve the HP from the context. `UnwrapOrDefault` from the `g` library is used
	// for safety, providing a zero value if the key doesn't exist.
	npcHP := ctx.Data.Get("npc_hp").UnwrapOrDefault().(g.Int)
	return npcHP > 20 && npcHP <= 50
}

// isNearDeath checks if the NPC is about to be defeated, triggering a last-ditch enraged state.
func isNearDeath(ctx *fsm.Context) bool {
	npcHP := ctx.Data.Get("npc_hp").UnwrapOrDefault().(g.Int)
	return npcHP > 0 && npcHP <= 20
}

// --- Step 4: The Main Application Logic ---

func main() {
	// Seed the random number generator to ensure each battle is different.
	rand.New(rand.NewSource(time.Now().UnixNano()))

	// --- Step 5: Configure the FSM (The NPC's "Brain") ---
	// Here, we create an instance of the FSM. We use a fluent API to chain method
	// calls, defining all the possible transitions and reactions in a readable way.
	npcBrain := fsm.New(StateIdle).
		// When in Idle, the Engage event transitions the NPC to Attacking.
		Transition(StateIdle, EventEngage, StateAttacking).

		// This block defines the NPC's reaction to taking damage. Notice how multiple
		// `TransitionWhen` calls are chained for the same event. The FSM will evaluate
		// them in order, and the first one whose Guard function returns `true` will be executed.
		TransitionWhen(StateAttacking, EventTakeDamage, StateDefending, isBadlyWounded). // If moderately hurt, defend.
		TransitionWhen(StateAttacking, EventTakeDamage, StateEnraged, isNearDeath).
		// If critically hurt, get angry.
		TransitionWhen(StateDefending, EventTakeDamage, StateEnraged, isNearDeath).
		// Getting hit while defending can also trigger rage.
		TransitionWhen(StateEnraged, EventTakeDamage, StateEnraged, isNearDeath).
		// If already enraged, taking more damage keeps it enraged.

		// After a turn of defending, the Recover event brings the NPC back to attacking.
		Transition(StateDefending, EventRecover, StateAttacking).

		// Defeat is an absolute transition that can occur from any active combat state.
		Transition(StateAttacking, EventDefeated, StateDefeated).
		Transition(StateDefending, EventDefeated, StateDefeated).
		Transition(StateEnraged, EventDefeated, StateDefeated).

		// OnEnter callbacks are used for "side effects" — actions that happen when a new state
		// is entered. Here, we use them to print descriptive text, making the battle feel more alive.
		OnEnter(StateAttacking, func(*fsm.Context) error {
			g.Println("-> The Orc growls and prepares to attack!")
			return nil
		}).
		OnEnter(StateDefending, func(*fsm.Context) error {
			g.Println("-> The Orc raises its shield, trying to recover.")
			return nil
		}).
		OnEnter(StateEnraged, func(*fsm.Context) error {
			g.Println("-> Wounded and cornered, the Orc flies into a rage!")
			return nil
		}).
		OnEnter(StateDefeated, func(*fsm.Context) error {
			g.Println("-> The Orc collapses, defeated.")
			return nil
		})

	// --- Step 6: Initialize the Battle ---
	player := &Character{Name: "Hero", HP: 100}
	npc := &Character{Name: "Orc Grunt", HP: 100}

	// We store the health values in the FSM's shared Context. This is how our Guard
	// functions will be able to access the NPC's HP to make decisions.
	npcBrain.Context().Data.Insert("player_hp", player.HP)
	npcBrain.Context().Data.Insert("npc_hp", npc.HP)

	g.Println("A wild {} appears!", npc.Name)
	npcBrain.Trigger(EventEngage) // This trigger officially starts the fight.

	// --- Step 7: The Main Game Loop ---
	turn := 1

	for player.HP > 0 && npc.HP > 0 {
		g.Println("\n--- Turn {} ---", turn)
		g.Println("Your HP: {}, {}'s HP: {}", player.HP, npc.Name, npc.HP)
		g.Println("Your move: (1) Attack, (2) Defend")

		// --- Player's Turn ---
		var choice string
		fmt.Scanln(&choice)

		if choice == "1" { // The player attacks the NPC.
			damage := g.Int(5).RandomRange(20)
			g.Println("You strike the {} for {} damage!", npc.Name, damage)
			npc.HP -= damage
			npcBrain.Context().Data.Insert("npc_hp", npc.HP) // Crucially, we update the HP in the FSM context.

			if npc.HP <= 0 {
				npcBrain.Trigger(EventDefeated)
			} else {
				// This is the core interaction: the player's action triggers an event in the NPC's FSM.
				// The FSM will now run its logic: check guards for the `TakeDamage` event from its
				// current state, and transition accordingly.
				npcBrain.Trigger(EventTakeDamage)
			}
		} else { // The player defends.
			heal := g.Int(5).RandomRange(15)
			g.Println("You brace yourself and recover {} HP.", heal)
			player.HP += heal
		}

		// --- NPC's Turn ---
		// The NPC's action is not random; it is determined by its CURRENT FSM state.
		if npc.HP > 0 {
			time.Sleep(1 * time.Second)

			switch npcBrain.Current() {
			case StateAttacking:
				damage := g.Int(5).RandomRange(15)
				g.Println("The {} retaliates, dealing {} damage!", npc.Name, damage)
				player.HP -= damage
			case StateDefending:
				heal := g.Int(10).RandomRange(20)
				g.Println("The {} focuses and recovers {} HP.", npc.Name, heal)
				npc.HP += heal
				npcBrain.Trigger(EventRecover) // After healing, it tries to go back to attacking.
			case StateEnraged:
				damage := g.Int(10).RandomRange(25)
				g.Println("The {} attacks with fury, dealing {} damage!", npc.Name, damage)
				player.HP -= damage
			}
		}
		turn++
	}

	// --- Step 8: End of the Battle ---
	g.Println("\n--- Battle Over ---")
	if player.HP > 0 {
		g.Println("You are victorious!")
	} else {
		g.Println("You have been defeated...")
	}
}
