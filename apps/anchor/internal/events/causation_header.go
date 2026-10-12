package events

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
)

// CausationHeader carries a workflow run's causation to a Product backend
// called by a workflow step. A backend that sends it back on its own calls to
// Anchor keeps those writes inside the chain, so the loop guards still hold
// across the round trip. A forged value can only stop the caller's own
// workflows from running.
const CausationHeader = "Anchor-Workflow-Causation"

const maxCausationHeaderBytes = 4096

func EncodeCausationHeader(causation Causation) string {
	encoded, err := json.Marshal(causation)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func DecodeCausationHeader(value string) (Causation, bool) {
	if value == "" || len(value) > maxCausationHeaderBytes {
		return Causation{}, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return Causation{}, false
	}
	var causation Causation
	if json.Unmarshal(decoded, &causation) != nil || causation.Depth < 0 {
		return Causation{}, false
	}
	return causation, true
}

// CausationMiddleware puts the causation a request carries into its context.
func CausationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		causation, ok := DecodeCausationHeader(r.Header.Get(CausationHeader))
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithCausation(r.Context(), causation)))
	})
}
