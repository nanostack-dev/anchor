package api

import (
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/rs/zerolog"
)

func logAPIError(logger zerolog.Logger, err error) *zerolog.Event {
	if apiErr, ok := fault.As(err); ok && apiErr != nil {
		status := apiErr.HTTPStatus()
		if status < http.StatusInternalServerError {
			return logger.Debug().Err(err).Int("status", status)
		}

		return logger.Error().Err(err).Int("status", status)
	}

	return logger.Error().Err(err)
}
