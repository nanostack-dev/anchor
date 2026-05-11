package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

const docsHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
    <title>Anchor API Documentation</title>
    <!-- Embed elements Elements via Web Component -->
    <script src="https://unpkg.com/@stoplight/elements/web-components.min.js"></script>
    <link rel="stylesheet" href="https://unpkg.com/@stoplight/elements/styles.min.css">
  </head>
  <body>
    <elements-api
      apiDescriptionUrl="/openapi.yaml"
      router="hash"
      layout="sidebar"
    />
  </body>
</html>`

func (n *AnchorAPI) RegisterDocsRoutes(router chi.Router, openAPISpec []byte) {
	router.Get("/docs", n.serveDocs)
	router.Get(
		"/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
			n.serveOpenAPISpec(w, r, openAPISpec)
		},
	)
}

func (n *AnchorAPI) serveDocs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(docsHTML))
}

func (n *AnchorAPI) serveOpenAPISpec(
	w http.ResponseWriter, _ *http.Request, openAPISpec []byte,
) {
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(openAPISpec)
}
