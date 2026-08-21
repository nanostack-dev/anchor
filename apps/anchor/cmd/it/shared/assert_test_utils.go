package itshared

import (
	"net/http"
	"reflect"
	"testing"

	anchorClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
)

// AssertProductAPIKeyInsufficientPermissions validates standard API key permission failures.
func AssertProductAPIKeyInsufficientPermissions(
	t *testing.T,
	response any,
	apiKeyID string,
	requiredScopes []string,
	currentScopes []string,
) {
	statusCode := getStatusCode(response)
	json403 := getJSON403(response)

	assert.Equal(
		t, http.StatusForbidden, statusCode,
		"should return 403 Forbidden for insufficient permissions",
	)

	if assert.NotNil(t, json403, "403 response should not be nil") {
		expectedResponse := &anchorClient.ApiErrorResponse{
			Errors: []anchorClient.ApiError{{
				Code:    "PRODUCT_API_KEY_INSUFFICIENT_PERMISSIONS",
				Message: "The product API key does not have sufficient permissions.",
				Metadata: &map[string]any{
					"api_key_id":      apiKeyID,
					"required_scopes": convertToInterfaceSlice(requiredScopes),
					"current_scopes":  convertToInterfaceSlice(currentScopes),
				},
				Field: nil,
			}},
		}

		assert.Equal(t, expectedResponse, json403)
	}
}

func AssertAnchorBadRequestError(
	t *testing.T,
	response any,
	code string,
	message string,
	details map[string]any,
) {
	statusCode := getStatusCode(response)
	json400 := getJSONBody(response, "JSON400")
	assert.Equal(
		t, http.StatusBadRequest, statusCode, "should return 400 Bad Request for validation errors",
	)
	if assert.NotNil(t, json400, "400 response should not be nil") {
		expectedResponse := &anchorClient.ApiErrorResponse{
			Errors: []anchorClient.ApiError{{
				Code:     code,
				Message:  message,
				Metadata: &details,
				Field:    nil,
			}},
		}
		assert.Equal(t, expectedResponse, json400)
	}
}

func AssertAnchorConflictError(
	t *testing.T,
	response any,
	code string,
	message string,
	details map[string]any,
) {
	t.Helper()
	statusCode := getStatusCode(response)
	json409 := getJSONBody(response, "JSON409")
	assert.Equal(t, http.StatusConflict, statusCode, "a uniqueness collision is a 409")
	if assert.NotNil(t, json409, "409 response should not be nil") {
		expectedResponse := &anchorClient.ApiErrorResponse{
			Errors: []anchorClient.ApiError{{
				Code:     code,
				Message:  message,
				Metadata: &details,
				Field:    nil,
			}},
		}
		assert.Equal(t, expectedResponse, json409)
	}
}

func getStatusCode(response any) int {
	if r, ok := response.(interface{ StatusCode() int }); ok {
		return r.StatusCode()
	}
	return 0
}

func getJSON403(response any) *anchorClient.ApiErrorResponse {
	value := reflect.ValueOf(response)

	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	if value.Kind() == reflect.Struct {
		json403Field := value.FieldByName("JSON403")
		if json403Field.IsValid() && !json403Field.IsNil() {
			if apiErrorResp, ok := reflect.TypeAssert[*anchorClient.ApiErrorResponse](json403Field); ok {
				return apiErrorResp
			}
		}
	}

	return nil
}

func getJSONBody(response any, field string) *anchorClient.ApiErrorResponse {
	value := reflect.ValueOf(response)

	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	if value.Kind() == reflect.Struct {
		json400Field := value.FieldByName(field)
		if json400Field.IsValid() && !json400Field.IsNil() {
			if apiErrorResp, ok := reflect.TypeAssert[*anchorClient.ApiErrorResponse](json400Field); ok {
				return apiErrorResp
			}
		}
	}

	return nil
}

func convertToInterfaceSlice(strings []string) []any {
	result := make([]any, len(strings))
	for i, s := range strings {
		result[i] = s
	}
	return result
}
