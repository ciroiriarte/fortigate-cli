package topology

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RenderMermaid renders the graph as a Mermaid `graph LR` diagram. It pastes
// into mermaid.live or a GitHub ```mermaid block. Labels escape `"` and use
// `<br/>` for line breaks.
func RenderMermaid(g Graph) string {
	var b strings.Builder
	b.WriteString("graph LR\n")
	for _, n := range g.Nodes {
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", n.ID, mermaidLabel(n.Label))
	}
	for _, e := range g.Edges {
		fmt.Fprintf(&b, "  %s -->|\"%s\"| %s\n", e.From, mermaidLabel(e.Label), e.To)
	}
	return b.String()
}

// RenderDOT renders the graph as Graphviz DOT. Pipe it to Graphviz, e.g.
// `fgt network topology --format dot | dot -Tsvg -o topo.svg`.
func RenderDOT(g Graph) string {
	var b strings.Builder
	b.WriteString("digraph topology {\n")
	b.WriteString("  rankdir=LR;\n")
	for _, n := range g.Nodes {
		fmt.Fprintf(&b, "  %s [label=\"%s\"];\n", n.ID, dotLabel(n.Label))
	}
	for _, e := range g.Edges {
		fmt.Fprintf(&b, "  %s -> %s [label=\"%s\"];\n", e.From, e.To, dotLabel(e.Label))
	}
	b.WriteString("}\n")
	return b.String()
}

// RenderJSON renders the graph as deterministic, indented JSON.
func RenderJSON(g Graph) (string, error) {
	out, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// mermaidLabel escapes a label for a quoted Mermaid string.
func mermaidLabel(s string) string {
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "\n", "<br/>")
	return s
}

// dotLabel escapes a label for a quoted DOT string.
func dotLabel(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}
