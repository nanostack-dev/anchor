package events

import (
	"errors"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
)

var errEmitRequiresTx = errors.New("events: Emit requires transactor.InTx")

func unknownTypeError(eventType Type) error {
	return fault.BadRequest(
		"UNKNOWN_EVENT_TYPE",
		"This event type is not in the product event catalog.",
	).Metadata(map[string]any{
		"event_type": string(eventType),
	})
}

func invalidEndpointURLError() error {
	return fault.BadRequest(
		"INVALID_EVENT_ENDPOINT_URL",
		"The event endpoint URL must be an absolute HTTP or HTTPS URL.",
	)
}

func insecureEndpointURLError() error {
	return fault.BadRequest(
		"INSECURE_EVENT_ENDPOINT_URL",
		"Production event endpoints must use HTTPS.",
	)
}
