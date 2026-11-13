package renderer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/email"
	"anchor/internal/email/renderer"
)

func renderFallbackText(t *testing.T, bodyHTML string) string {
	t.Helper()
	result, err := renderer.New().Render(&email.TemplateVersion{
		Subject:  "s",
		BodyHTML: bodyHTML,
	}, map[string]any{})
	require.NoError(t, err)
	return result.BodyText
}

// A receipt laid out the way real email is: a head with a title and CSS, a
// table of items, a list whose items omit </li>, and a footer address.
const receiptHTML = `<!doctype html><html><head><meta charset='utf-8'>` +
	`<title>Receipt NW-1</title><style>body{margin:0}@media (max-width:600px){.card{width:100%}}</style></head>` +
	`<body><div class='preheader' style='display:none'>Thanks Ada &mdash; your beans are roasting.</div>` +
	`<table><tr><td><h1>Receipt for order NW-1</h1><p>Hello <strong>Ada</strong>,<br>thanks for your order.</p></td></tr></table>` +
	`<table class='items'>` +
	`<tr><td>Ethiopia Yirgacheffe <span>250&nbsp;g</span></td><td>2</td><td>&euro;24.00</td></tr>` +
	`<tr><td colspan='2'><b>Total</b></td><td><b>&euro;91.40</b></td></tr></table>` +
	`<ul><li>We roast on Friday<li>We ship on Monday</ul>` +
	`<p>Your pickup code is <strong>482913</strong></p>` +
	`<p><a href='https://shop.example/orders/NW-1?utm_source=email&amp;utm_medium=receipt'>Track your order</a> &middot; ` +
	`<a href='mailto:help@shop.example'>Contact support</a></p>` +
	`<p>12 Rue de la Paix<br>75002 Paris</p></body></html>`

func TestRenderFallbackTextDropsHeadTitleAndStyle(t *testing.T) {
	text := renderFallbackText(t, receiptHTML)

	assert.NotContains(t, text, "Receipt NW-1\n")
	assert.NotContains(t, text, "margin")
	assert.NotContains(t, text, "@media")
	assert.Equal(t, "Thanks Ada — your beans are roasting.", firstLine(text))
}

func TestRenderFallbackTextSeparatesTableCells(t *testing.T) {
	text := renderFallbackText(t, receiptHTML)

	assert.Contains(t, text, "\nEthiopia Yirgacheffe 250 g 2 €24.00\n")
	assert.Contains(t, text, "\nTotal €91.40\n")
}

func TestRenderFallbackTextPutsEachListItemOnItsOwnLine(t *testing.T) {
	text := renderFallbackText(t, receiptHTML)

	assert.Contains(t, text, "\n- We roast on Friday\n- We ship on Monday\nYour pickup code is 482913\n")
}

func TestRenderFallbackTextNumbersAnOrderedList(t *testing.T) {
	text := renderFallbackText(t, `<ol start='3'><li>Open the app<li>Paste the code</ol>`)

	assert.Equal(t, "3. Open the app\n4. Paste the code", text)
}

func TestRenderFallbackTextBreaksLinesAtBr(t *testing.T) {
	text := renderFallbackText(t, receiptHTML)

	assert.Contains(t, text, "\nHello Ada,\nthanks for your order.\n")
	assert.Contains(t, text, "\n12 Rue de la Paix\n75002 Paris")
}

func TestRenderFallbackTextKeepsAWebLinksURL(t *testing.T) {
	text := renderFallbackText(t, receiptHTML)

	assert.Contains(t, text,
		"\nTrack your order (https://shop.example/orders/NW-1?utm_source=email&utm_medium=receipt) · Contact support\n")
}

func TestRenderFallbackTextKeepsALinkWhoseTextIsItsURLOnce(t *testing.T) {
	text := renderFallbackText(t, `<p><a href='https://shop.example/a'>https://shop.example/a</a></p>`)

	assert.Equal(t, "https://shop.example/a", text)
}

func TestRenderFallbackTextKeepsOneBlankLineAtMost(t *testing.T) {
	text := renderFallbackText(t, `<p>One</p><br><br><br><br><p>Two</p>`)

	assert.Equal(t, "One\n\nTwo", text)
}

func TestRenderFallbackTextEscapesVariablesOnceAndDecodesThem(t *testing.T) {
	result, err := renderer.New().Render(&email.TemplateVersion{
		Subject:   "s",
		BodyHTML:  `<p>Hello {{ .name }}</p>`,
		Variables: []email.VariableSchema{{Name: "name", Type: email.VariableTypeString}},
	}, map[string]any{"name": "Tom & <Jerry>"})
	require.NoError(t, err)

	assert.Equal(t, "Hello Tom & <Jerry>", result.BodyText)
}

func firstLine(text string) string {
	for i, r := range text {
		if r == '\n' {
			return text[:i]
		}
	}
	return text
}
