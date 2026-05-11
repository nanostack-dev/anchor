package middleware

import (
	"encoding/json"
	"errors"
	"net/http"

	sharedsentry "github.com/nanostack-dev/shared/fxmodules/sentry"
	"github.com/nanostack-dev/shared/toolkit"

	"anchor/internal/api"

	"github.com/rs/zerolog"
)

// ErrorMiddleware handles centralized error processing for API responses.
type ErrorMiddleware struct {
	logger zerolog.Logger
}

// NewErrorMiddleware creates a new ErrorMiddleware instance.
func NewErrorMiddleware(logger zerolog.Logger) *ErrorMiddleware {
	return &ErrorMiddleware{
		logger: logger,
	}
}

// HandleRequestError is designed to be used as RequestErrorHandlerFunc in the OpenAPI framework.
// It handles errors that occur while decoding/binding the incoming request (e.g. malformed JSON
// body) and returns a 400 Bad Request with a structured JSON error body.
func (em *ErrorMiddleware) HandleRequestError(w http.ResponseWriter, _ *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	apiResponse := api.ApiErrorResponse{
		Errors: []api.ApiError{
			{Code: "BAD_REQUEST", Message: err.Error()},
		},
	}
	if encodeErr := json.NewEncoder(w).Encode(apiResponse); encodeErr != nil {
		em.logger.Error().Err(encodeErr).Msg("Failed to encode request error response")
	}
}

// HandleResponseError is designed to be used as ResponseErrorHandlerFunc in OpenAPI framework
// This function processes errors from API handlers and returns proper error responses.
func (em *ErrorMiddleware) HandleResponseError(w http.ResponseWriter, _ *http.Request, err error) {
	apiResponse, status := em.prepareErrorResponse(err)

	// Set content type and status
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// Encode and write response
	if encodeErr := json.NewEncoder(w).Encode(apiResponse); encodeErr != nil {
		em.logger.Error().Err(encodeErr).Msg("Failed to encode error response")
		// Fallback to plain text error if JSON encoding fails
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}
}

// prepareErrorResponse processes different types of errors and returns appropriate API response.
func (em *ErrorMiddleware) prepareErrorResponse(err error) (api.ApiErrorResponse, int) {
	var anchorErr *toolkit.NanostackError
	if errors.As(err, &anchorErr) {
		status := anchorErr.GetStatus()
		if status == 0 {
			status = http.StatusBadRequest
		}
		apiResponse := mapAnchorErrorsToAPI(anchorErr)
		em.logger.Debug().
			Int("status", status).
			Int("error_count", len(anchorErr.Errors)).
			Msg("Mapped Anchor errors to API response")
		return apiResponse, status
	}

	em.logger.Error().Err(err).Msg("Unhandled internal server error")
	sharedsentry.CaptureException(err)
	apiResponse := mapAnchorErrorsToAPI(toolkit.ErrUnexpected)
	return apiResponse, http.StatusInternalServerError
}

// mapAnchorErrorsToAPI converts shared toolkit errors to API response format.
func mapAnchorErrorsToAPI(anchorErr *toolkit.NanostackError) api.ApiErrorResponse {
	apiErrors := make([]api.ApiError, 0, len(anchorErr.Errors))
	for _, detail := range anchorErr.Errors {
		apiErr := api.ApiError{
			Code:    detail.Code,
			Message: detail.Message,
		}

		if len(detail.Metadata) > 0 {
			meta := detail.Metadata
			apiErr.Details = &meta
		}
		apiErrors = append(apiErrors, apiErr)
	}
	return api.ApiErrorResponse{Errors: apiErrors}
}
