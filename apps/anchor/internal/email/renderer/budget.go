package renderer

import (
	"context"
	"fmt"
	texttemplate "text/template"
	"text/template/parse"
)

const (
	// maxRenderSteps caps the combined number of range iterations and template
	// invocations across Subject, BodyHTML, and BodyText in a single Render call
	// (anchor#410).
	maxRenderSteps = 10_000

	budgetFuncName = "anchorTick"
)

var errRenderStepLimit = fmt.Errorf("template rendering exceeded %d iterations or template calls", maxRenderSteps)

func parseStepNode() parse.Node {
	t := texttemplate.Must(
		texttemplate.New("anchor_budget_probe").
			Funcs(texttemplate.FuncMap{budgetFuncName: func() (bool, error) { return false, nil }}).
			Parse("{{if " + budgetFuncName + "}}{{end}}"),
	)
	return t.Tree.Root.Nodes[0]
}

type renderBudget struct {
	ctx      context.Context
	steps    int
	stepNode parse.Node
}

func newRenderBudget(ctx context.Context, stepNode parse.Node) *renderBudget {
	if ctx == nil {
		ctx = context.Background()
	}
	if stepNode == nil {
		stepNode = parseStepNode()
	}
	return &renderBudget{ctx: ctx, stepNode: stepNode}
}

func (b *renderBudget) tick() (bool, error) {
	if err := b.ctx.Err(); err != nil {
		return false, err
	}
	b.steps++
	if b.steps > maxRenderSteps {
		return false, errRenderStepLimit
	}
	return false, nil
}

func (b *renderBudget) funcMap() map[string]any {
	return map[string]any{
		budgetFuncName: b.tick,
	}
}

func (b *renderBudget) newStepNode() parse.Node {
	return b.stepNode.Copy()
}

// instrumentTree injects a zero-output {{if anchorTick}}{{end}} guard at the
// start of every template root (main and define/block sub-templates) and at the
// start of every parse.RangeNode body. That bounds loops (including
// dot-indirected integers, variable/hex ranges, and nested O(n^2) data ranges),
// 2^k define/template fan-out, and self-recursive template calls even when they
// emit zero bytes.
func (b *renderBudget) instrumentTree(tree *parse.Tree) {
	if tree == nil || tree.Root == nil {
		return
	}
	b.instrumentNode(tree.Root)
	tree.Root.Nodes = append([]parse.Node{b.newStepNode()}, tree.Root.Nodes...)
}

func (b *renderBudget) instrumentNode(node parse.Node) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return
		}
		for _, child := range n.Nodes {
			b.instrumentNode(child)
		}
	case *parse.RangeNode:
		b.instrumentNode(n.List)
		if n.List != nil {
			n.List.Nodes = append([]parse.Node{b.newStepNode()}, n.List.Nodes...)
		}
		b.instrumentNode(n.ElseList)
	case *parse.IfNode:
		b.instrumentNode(n.List)
		b.instrumentNode(n.ElseList)
	case *parse.WithNode:
		b.instrumentNode(n.List)
		b.instrumentNode(n.ElseList)
	}
}
