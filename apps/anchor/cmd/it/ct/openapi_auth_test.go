package ct_test

import (
	"io"
	"net/http"
	"testing"

	itshared "anchor/cmd/it/shared"
)

func TestOpenAPIProtected403(t *testing.T) {
	noAuth := map[string]struct{}{
		"post /v1/auth/login":    {},
		"post /v1/auth/logout":   {},
		"post /v1/auth/register": {},
		"post /v1/auth/refresh":  {},
	}

	spec, components, paramDefs := parseOpenAPISpec(t)
	ops := extractOperations(spec, components, paramDefs, noAuth, itshared.ServerURL)
	// expiredToken, err =
	for _, op := range ops {
		t.Run(
			op.OpKey+"_false_jwt", func(t *testing.T) {
				headers := map[string]string{
					"Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.KMUFsIDTnFmyG3nMiGM6H9FNFUROf3wh7SmqJp-QV30",
				}
				resp, err := sendRequest(op, components, headers)
				if err != nil {
					t.Fatalf("failed to %s %s: %v", op.Verb, op.URL, err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusUnauthorized {
					bodyBytes, _ := io.ReadAll(resp.Body)
					t.Errorf(
						"expected 401 Forbidden for %s, got %d. Response body: %s", op.URL,
						resp.StatusCode, string(bodyBytes),
					)
				}
			},
		)
		t.Run(
			op.OpKey+"_expired_jwt", func(t *testing.T) {
				headers := map[string]string{
					//TODO: replace this with a real expired token
					"Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoidXNlcl8yeFRvWXhiYUlQSW5lRDZJRjJ1M0lFSGpIVnkiLCJpc3MiOiJuYW5vc3RhY2siLCJzdWIiOiJ1c2VyXzJ4VG9ZeGJhSVBJbmVENklGMnUzSUVIakhWeSIsImF1ZCI6WyJuYW5vc3RhY2tfYWNjZXNzIl0sImV4cCI6MTc0Nzk3MTQ3NSwiaWF0IjoxNzQ3OTcxNDc1LCJ0ZW5hbnRfaWQiOiJ0ZW5hbnRfMnhUb1l2ZFpjWFA0b1JHNDRRTDFLTVl0UGJEIn0.05CqzPQbJQZhIVOpZvwMlDnR_c1b6zidq96Lc3kfcKE",
				}
				resp, err := sendRequest(op, components, headers)
				if err != nil {
					t.Fatalf("failed to %s %s: %v", op.Verb, op.URL, err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusUnauthorized {
					bodyBytes, _ := io.ReadAll(resp.Body)
					t.Errorf(
						"expected 401 Forbidden for %s, got %d. Response body: %s", op.URL,
						resp.StatusCode, string(bodyBytes),
					)
				}
			},
		)
		//TODO: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoidXNlcl8yeFRwQW5SQnprVmowbjlndEFCNjdzWUpCRzYiLCJpc3MiOiJuYW5vc3RhY2siLCJzdWIiOiJ1c2VyXzJ4VHBBblJCemtWajBuOWd0QUI2N3NZSkJHNiIsImF1ZCI6WyJuYW5vc3RhY2tfYWNjZXNzIl0sImV4cCI6MTA5NzEzNDM4MTMsImlhdCI6MTc0Nzk3MTc3NiwidGVuYW50X2lkIjoidGVuYW50XzJ4VHBBbENPY3BzR3A0ZmhoVVo5R1hWUzFQQyJ9.1cJpT7qqLWOOUnYQ59ekMn8pGkvV8t_3JBO2FJosTP0
		// Test that use valid jwt but target resources that do not belong to the productuserservice
	}
}
