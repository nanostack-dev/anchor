package api

import (
	"net/http"
	"regexp"
)

// workspaceRoute matches the deprecated built-in workspace routes: the
// collection, its search, and one workspace.
var workspaceRoute = regexp.MustCompile(
	`^(/v1/products/[^/]+/organizations/[^/]+)/workspaces(?:/([^/]+))?/?$`,
)

// workspaceDeprecation is the date the workspace routes were deprecated, as
// an RFC 9745 Deprecation header: 2026-10-25T00:00:00Z.
const workspaceDeprecation = "@1792886400"

// WorkspaceDeprecationMiddleware marks every response of a built-in workspace
// route deprecated (ADR-0022 A6): a Deprecation header, and a Link to its
// successor among the resource routes, the workspace resource for one
// workspace and the resource search for the collection. It runs before
// authentication, so refusals carry the headers too. Nothing else about the
// response changes.
func WorkspaceDeprecationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if match := workspaceRoute.FindStringSubmatch(r.URL.Path); match != nil {
			successor := match[1] + "/resources/search"
			if id := match[2]; id != "" && id != "search" {
				successor = match[1] + "/resources/workspace/" + id
			}
			w.Header().Set("Deprecation", workspaceDeprecation)
			w.Header().Add("Link", "<"+successor+`>; rel="successor-version"`)
		}
		next.ServeHTTP(w, r)
	})
}
