package ct_test

import (
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/stretchr/testify/assert"
)

func AssertAPIError(t *testing.T, rawErrors []ct.ApiError, expected ct.ApiError) {
	assert.Len(t, rawErrors, 1, "Expected exactly one error in the response")
	assert.Equal(t, expected.Code, rawErrors[0].Code, "Error code does not match")
	assert.Equal(t, expected.Message, rawErrors[0].Message, "Error message does not match")
}
