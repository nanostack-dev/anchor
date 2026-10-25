package resource_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"anchor/internal/domain/resource"
)

func TestValidID(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		id    string
		valid bool
	}{
		"A UUID is a valid identifier":                        {id: "3b7e91d2-0c4a-4f6e-9d21-8a5c7e4b2f60", valid: true},
		"Every unreserved character is valid":                 {id: "Az09._~-", valid: true},
		"200 characters are valid":                            {id: strings.Repeat("a", 200), valid: true},
		"201 characters are refused":                          {id: strings.Repeat("a", 201)},
		"An empty identifier is refused":                      {id: ""},
		"A dot alone is refused":                              {id: "."},
		"Two dots alone are refused":                          {id: ".."},
		"Three dots are valid":                                {id: "...", valid: true},
		"A slash is refused":                                  {id: "a/b"},
		"A space is refused":                                  {id: "a b"},
		"A character that needs escaping in a URL is refused": {id: "a%b"},
		"A non-ASCII letter is refused":                       {id: "é"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.valid, resource.ValidID(tc.id))
		})
	}
}
