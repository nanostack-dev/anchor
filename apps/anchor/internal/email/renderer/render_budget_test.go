package renderer_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"anchor/internal/domain/email"
	"anchor/internal/email/renderer"
)

const renderOutputCap = 256 * 1024

func doublingTemplate(k int) string {
	var b strings.Builder
	b.WriteString(`{{define "a0"}}0123456789abcdef{{end}}`)
	for i := 1; i <= k; i++ {
		fmt.Fprintf(&b, `{{define "a%d"}}{{template "a%d"}}{{template "a%d"}}{{end}}`, i, i-1, i-1)
	}
	fmt.Fprintf(&b, `{{template "a%d"}}`, k)
	return b.String()
}

func TestRenderBoundsTheOutput(t *testing.T) {
	t.Run("output doubling via define/template is bounded", func(t *testing.T) {
		const k = 16
		src := doublingTemplate(k)
		start := time.Now()
		res, err := renderer.New().Render(&email.TemplateVersion{
			Subject: "s", BodyHTML: src, BodyText: "t",
		}, map[string]any{})
		elapsed := time.Since(start)
		t.Logf("K=%d source_bytes=%d err=%v body_html_bytes=%d elapsed=%s",
			k, len(src), err, len(res.BodyHTML), elapsed)
		if err == nil && len(res.BodyHTML) > renderOutputCap {
			t.Errorf("render produced %d bytes (16*2^%d=%d) from %d source bytes with no output cap (cap %d)",
				len(res.BodyHTML), k, 16<<k, len(src), renderOutputCap)
		}
	})

	t.Run("range over integer literal is bounded", func(t *testing.T) {
		const n = 1_000_000
		src := fmt.Sprintf("{{range %d}}x{{end}}", n)
		start := time.Now()
		res, err := renderer.New().Render(&email.TemplateVersion{
			Subject: "s", BodyHTML: src, BodyText: "t",
		}, map[string]any{})
		elapsed := time.Since(start)
		t.Logf("N=%d source_bytes=%d err=%v body_html_bytes=%d elapsed=%s",
			n, len(src), err, len(res.BodyHTML), elapsed)
		if err == nil && len(res.BodyHTML) > renderOutputCap {
			t.Errorf("range %d ran every iteration (%d bytes) from %d source bytes with no iteration/output cap",
				n, len(res.BodyHTML), len(src))
		}
	})
}
