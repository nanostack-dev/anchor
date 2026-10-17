package api

import "github.com/rs/zerolog"

func LogAPIErrorForTest(logger zerolog.Logger, err error) *zerolog.Event {
	return logAPIError(logger, err)
}
