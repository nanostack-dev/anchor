package service

import (
	"net/http"

	frameworkapierror "github.com/nanostack-dev/nanostack-framework/pkg/apierror"
	"github.com/rs/zerolog"
)

func logServiceError(logger zerolog.Logger, err error) *zerolog.Event {
	if apiErr, ok := frameworkapierror.As(err); ok && apiErr != nil {
		status := apiErr.HTTPStatus()
		if status < http.StatusInternalServerError {
			return logger.Debug().Err(err).Int("status", status)
		}

		return logger.Error().Err(err).Int("status", status)
	}

	return logger.Error().Err(err)
}
