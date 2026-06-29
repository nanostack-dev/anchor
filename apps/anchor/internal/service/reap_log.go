package service

import (
	"github.com/nanostack-dev/pgkit/pgqueue"
	"github.com/rs/zerolog"
)

// reapLogLevel selects the severity for a stuck-job reap result.
//
// Requeued jobs are routine recovery: a worker exceeded its visibility timeout
// and the job was returned to pending for another attempt. That is a warning,
// not an error, and matches pgqueue's own warn-level reap logging.
//
// Jobs counted in Failed exhausted their attempts during the reap and were
// dead-lettered. That path does not fire the worker's OnJobFailed hook, so it
// must stay loud at error level to remain visible.
func reapLogLevel(result pgqueue.ReapResult) zerolog.Level {
	if result.Failed > 0 {
		return zerolog.ErrorLevel
	}
	return zerolog.WarnLevel
}
