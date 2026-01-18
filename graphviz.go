package fsm

import (
	"github.com/enetx/g"
	"github.com/enetx/g/cmp"
)

// ToDOT generates a DOT language string representation of the FSM for visualization.
func (f *FSM) ToDOT() g.String {
	b := g.NewBuilder()

	b.WriteString("digraph FSM {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString(
		"  node [shape=circle, style=filled, fillcolor=\"#f8f8f8\", color=\"#444444\", fontname=\"Helvetica\"];\n",
	)
	b.WriteString("  edge [fontname=\"Helvetica\", fontsize=10];\n\n")

	b.WriteString("  __start [shape=point, style=invis];\n")
	b.WriteString(g.Format("  __start -> \"{}\" [label=\" initial\"];\n\n", f.initial))

	grouped := g.NewMap[g.Pair[State, State], g.Slice[g.String]]()

	for from, transitions := range f.transitions.Iter() {
		for transition := range transitions.Iter() {
			key := g.Pair[State, State]{Key: from, Value: transition.to}

			label := g.String(transition.event)
			if transition.guard != nil {
				label += " (guarded)"
			}

			grouped.Entry(key).
				AndModify(func(s *g.Slice[g.String]) { s.Push(label) }).
				OrInsert(g.SliceOf(label))
		}
	}

	states := f.States()
	states.SortBy(cmp.Cmp)

	outgoing := g.NewSet[State]()
	for p := range grouped.Keys().Iter() {
		outgoing.Insert(p.Key)
	}

	for state := range states.Iter() {
		var attrs g.Slice[g.String]
		attrs.Push(g.Format("label=\"{}\"", state))

		switch {
		case state == f.current:
			attrs.Push("fillcolor=\"#90ee90\"", "shape=doublecircle")
		case !outgoing.Contains(state):
			attrs.Push("fillcolor=\"#d3d3d3\"", "shape=doublecircle")
		}

		var tooltips g.Slice[g.String]

		if f.onEnter.Contains(state) {
			tooltips.Push("OnEnter")
		}

		if f.onExit.Contains(state) {
			tooltips.Push("OnExit")
		}

		if tooltips.NotEmpty() {
			attrs.Push(g.Format("tooltip=\"{}\"", tooltips.Join("\\n")))
		}

		b.WriteString(g.Format("  \"{}\" [{}];\n", state, attrs.Join(", ")))
	}

	b.WriteByte('\n')

	for pair, labels := range grouped.Iter() {
		from, to := pair.Key, pair.Value

		var edge g.Slice[g.String]
		label := labels.Join("\\n")

		edge.Push(g.Format("label=\" {} \"", label))

		if label.Contains("(guarded)") {
			edge.Push("style=dashed", "color=red", "arrowhead=odiamond")
		}

		b.WriteString(g.Format("  \"{}\" -> \"{}\" [{}];\n", from, to, edge.Join(", ")))
	}

	b.WriteString("\n  subgraph cluster_legend {\n")
	b.WriteString("    label = \"Legend\";\n")
	b.WriteString("    style = dashed;\n")
	b.WriteString(`    key [label=<
      <table border="0" cellpadding="4" cellspacing="0" cellborder="0">
        <tr><td align="right">●</td><td>Regular state</td></tr>
        <tr><td align="right"><font color="green">◎</font></td><td>Current state</td></tr>
        <tr><td align="right"><font color="gray">◎</font></td><td>Final state</td></tr>
        <tr><td align="right"><font color="red">→</font></td><td>Guarded transition</td></tr>
      </table>
    >, shape=none];`)

	b.WriteString("  }\n")
	b.WriteString("}\n")

	return b.String()
}
