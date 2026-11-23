package renderer_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"anchor/internal/domain/email"
	"anchor/internal/email/renderer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const renderOutputCap = 256 * 1024

func doublingTemplate(k int, leaf string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `{{define "a0"}}%s{{end}}`, leaf)
	for i := 1; i <= k; i++ {
		fmt.Fprintf(&b, `{{define "a%d"}}{{template "a%d"}}{{template "a%d"}}{{end}}`, i, i-1, i-1)
	}
	fmt.Fprintf(&b, `{{template "a%d"}}`, k)
	return b.String()
}

func TestRenderBoundsTheOutput(t *testing.T) {
	t.Run("output doubling via define/template is bounded", func(t *testing.T) {
		const k = 16
		src := doublingTemplate(k, "0123456789abcdef")
		start := time.Now()
		res, err := renderer.New().Render(&email.TemplateVersion{
			Subject: "s", BodyHTML: src, BodyText: "t",
		}, map[string]any{})
		elapsed := time.Since(start)
		t.Logf("K=%d source_bytes=%d err=%v body_html_bytes=%d elapsed=%s",
			k, len(src), err, len(res.BodyHTML), elapsed)
		require.Error(t, err)
		assert.LessOrEqual(t, len(res.BodyHTML), renderOutputCap)
	})

	t.Run("silent exponential define/template fan-out is bounded", func(t *testing.T) {
		src := doublingTemplate(22, "")
		start := time.Now()
		_, err := renderer.New().Render(&email.TemplateVersion{
			Subject: "s", BodyHTML: src, BodyText: "t",
		}, map[string]any{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeded")
		assert.Less(t, time.Since(start), 2*time.Second)
	})

	t.Run("range over integer literal is bounded", func(t *testing.T) {
		const n = 1_000_000
		src := fmt.Sprintf("{{range %d}}x{{end}}", n)
		_, err := renderer.New().Render(&email.TemplateVersion{
			Subject: "s", BodyHTML: src, BodyText: "t",
		}, map[string]any{})
		require.Error(t, err)
	})

	t.Run(
		"variable, parenthesized, underscored, declared-variable, and hex integer ranges are bounded",
		func(t *testing.T) {
			for name, src := range map[string]string{
				"variable indirection": "{{$n := 100000000}}{{range $n}}{{end}}",
				"parenthesized int":    "{{range (100000000)}}{{end}}",
				"underscored literal":  "{{range 20_000}}{{end}}",
				"single-variable int":  "{{range $i := 20000}}{{end}}",
				"hex literal":          "{{range 0x100000}}{{end}}",
			} {
				t.Run(name, func(t *testing.T) {
					start := time.Now()
					_, err := renderer.New().Render(&email.TemplateVersion{
						Subject: "s", BodyHTML: src, BodyText: "t",
					}, map[string]any{})
					require.Error(t, err)
					assert.Contains(t, err.Error(), "exceeded")
					assert.NotContains(t, err.Error(), "anchorTick")
					assert.Less(t, time.Since(start), 2*time.Second)
				})
			}
		},
	)

	t.Run("step budget boundary passes just below 10000 and fails above", func(t *testing.T) {
		_, err := renderer.New().Render(&email.TemplateVersion{
			Subject: "s", BodyHTML: "{{range 9990}}{{end}}", BodyText: "t",
		}, map[string]any{})
		require.NoError(t, err)

		_, err = renderer.New().Render(&email.TemplateVersion{
			Subject: "s", BodyHTML: "{{range 10001}}{{end}}", BodyText: "t",
		}, map[string]any{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeded")
	})

	t.Run("root template self-recursion is bounded", func(t *testing.T) {
		start := time.Now()
		_, err := renderer.New().Render(&email.TemplateVersion{
			Subject: "s", BodyHTML: `{{template "body_html"}}`, BodyText: "t",
		}, map[string]any{})
		require.Error(t, err)
		assert.Less(t, time.Since(start), 2*time.Second)
	})

	t.Run("block definition fan-out is bounded", func(t *testing.T) {
		var b strings.Builder
		b.WriteString(`{{define "b0"}}{{end}}`)
		for i := 1; i <= 18; i++ {
			fmt.Fprintf(
				&b,
				`{{define "b%d"}}{{block "call%d" .}}{{template "b%d"}}{{template "b%d"}}{{end}}{{end}}`,
				i,
				i,
				i-1,
				i-1,
			)
		}
		b.WriteString(`{{template "b18"}}`)
		start := time.Now()
		_, err := renderer.New().Render(&email.TemplateVersion{
			Subject: "s", BodyHTML: b.String(), BodyText: "t",
		}, map[string]any{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeded")
		assert.Less(t, time.Since(start), 2*time.Second)
	})

	t.Run("range over map exceeding budget is bounded", func(t *testing.T) {
		m := make(map[string]int, 150)
		for i := range 150 {
			m[fmt.Sprintf("k%d", i)] = i
		}
		start := time.Now()
		_, err := renderer.New().Render(&email.TemplateVersion{
			Subject: "s", BodyHTML: "{{range .m}}{{range $.m}}{{end}}{{end}}", BodyText: "t",
		}, map[string]any{"m": m})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeded")
		assert.Less(t, time.Since(start), 2*time.Second)
	})

	t.Run("range over dot-indirected integer variable is bounded", func(t *testing.T) {
		src := "{{range .n}}{{end}}"
		start := time.Now()
		_, err := renderer.New().Render(&email.TemplateVersion{
			Subject: "s", BodyHTML: src, BodyText: "t",
		}, map[string]any{"n": 100_000_000})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeded")
		assert.Less(t, time.Since(start), 2*time.Second)
	})

	t.Run("nested data range quadratic loop is bounded", func(t *testing.T) {
		items := make([]int, 200)
		src := "{{range $i, $v := .items}}{{range $j, $w := $.items}}{{end}}{{end}}"
		start := time.Now()
		_, err := renderer.New().Render(&email.TemplateVersion{
			Subject: "s", BodyHTML: src, BodyText: "t",
		}, map[string]any{"items": items})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeded")
		assert.Less(t, time.Since(start), 2*time.Second)
	})

	t.Run("shared budget spans subject, body_html, and body_text", func(t *testing.T) {
		loop := "{{range 4000}}{{end}}"
		_, err := renderer.New().Render(&email.TemplateVersion{
			Subject: loop, BodyHTML: loop, BodyText: loop,
		}, map[string]any{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeded")
	})

	t.Run("cancelled context aborts rendering", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := renderer.New().RenderContext(ctx, &email.TemplateVersion{
			Subject: "hello", BodyHTML: "<p>{{range 10}}a{{end}}</p>", BodyText: "hello",
		}, map[string]any{})
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("instrumentation preserves HTML contextual escaping across attributes and text", func(t *testing.T) {
		res, err := renderer.New().Render(&email.TemplateVersion{
			Subject:  "{{range .items}}{{.}}{{end}}",
			BodyHTML: `<div class="{{range .items}}{{.}} {{end}}"><a href="https://example.com/?q={{range .items}}{{.}}{{end}}">{{range .items}}<span>{{.}}</span>{{end}}</a></div>`,
			BodyText: "{{range .items}}{{.}},{{end}}",
		}, map[string]any{"items": []string{"a", "<script>"}})
		require.NoError(t, err)
		assert.Equal(t, "a<script>", res.Subject)
		assert.Equal(t, "a,<script>,", res.BodyText)
		assert.NotContains(t, res.BodyHTML, "<script>")
		assert.Contains(t, res.BodyHTML, "<span>&lt;script&gt;</span>")
	})
}
