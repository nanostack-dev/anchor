package api

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/log"
	"github.com/rs/zerolog"
)

// logAPIError starts an event for err at the level log.LevelFor picks, with
// err and its HTTP status attached: a caller fault (a 4xx fault, an invalid
// search cursor or sort) logs at debug, a cancelled request at warn, and
// anything else at error.
func logAPIError(logger zerolog.Logger, err error) *zerolog.Event {
	return log.Event(&logger, err)
}
