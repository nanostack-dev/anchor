package engine

import (
	"context"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/pgkit/queue"

	"anchor/internal/domain/workflow"
	"anchor/internal/events"
)

const (
	customEventIDPrefix   = "wfevt"
	customEventMaxRetries = 6
	keyEventType          = "event_type"
	keyEventID            = "event_id"
	keyData               = "data"
	keyDataTypes          = "data_types"
)

// CustomEventSender hands a custom event to the workflows of a Product. The
// event carries the causation of the run that sent it.
type CustomEventSender interface {
	Send(ctx context.Context, productID, eventType string, data events.Data) (string, error)
}

type queueCustomEvents struct {
	queue      *queue.Client
	transactor transactor.Transactor
}

func NewCustomEventSender(queueClient *queue.Client, tx transactor.Transactor) CustomEventSender {
	return &queueCustomEvents{queue: queueClient, transactor: tx}
}

func (q *queueCustomEvents) Send(
	ctx context.Context, productID, eventType string, data events.Data,
) (string, error) {
	eventID := ids.MustNew(customEventIDPrefix)
	payload, err := events.EncodeEvent(ctx, eventID, productID, events.Type(eventType), data, time.Now())
	if err != nil {
		return "", err
	}
	err = q.transactor.InTx(ctx, func(txCtx context.Context) error {
		_, enqueueErr := q.queue.EnqueueTx(txCtx, transactor.CurrentTx(txCtx), queue.EnqueueParams{
			QueueName:   workflow.EventQueueName,
			Payload:     payload,
			MaxAttempts: customEventMaxRetries,
		})
		return enqueueErr
	})
	return eventID, err
}

func workflowEmitAction(sender CustomEventSender) action {
	return action{
		spec: ActionSpec{
			Type: ActionWorkflowEmit, Group: GroupCustom, Name: "Start other workflows", Writes: true,
			Description: "Emits a custom event. Every enabled workflow triggered by it runs next, " +
				"with this event's data. Custom events stay inside Anchor.",
			Params: []ParamSpec{
				{
					Name:        keyEvent,
					Label:       "Custom event",
					Type:        ParamCustomEvent,
					Required:    true,
					Literal:     true,
					Description: "Name of the event, such as onboarding.completed. It becomes custom.onboarding.completed.",
				},
				{
					Name: keyData, Label: "Data", Type: ParamJSON,
					Description: "JSON object the event carries; later workflows read it as event.data.<key>.",
				},
				{
					Name: keyDataTypes, Label: "Field types", Type: ParamFieldTypes, Literal: true, Types: keyData,
					Description: `Type of each data key, such as {"plan": "text"}. A key left out is text. ` +
						"Every step that sends the event gives a field the same type.",
				},
			},
			Outputs: []OutputSpec{
				{Name: keyEventType, Type: FieldText, Description: "Type of the emitted event."},
				{Name: keyEventID, Type: FieldText, Description: "Identifier of the emitted event."},
			},
		},
		emits: func(params map[string]string) []string {
			return []string{workflow.CustomEventType(params[keyEvent])}
		},
		run: func(ctx context.Context, env Env, p Params) (map[string]any, error) {
			object, _, err := p.Object(keyData)
			if err != nil {
				return nil, err
			}
			data := events.Data{}
			for key, value := range object {
				data[key] = Stringify(value)
			}
			eventType := workflow.CustomEventType(p.String(keyEvent))
			eventID, err := sender.Send(ctx, env.ProductID, eventType, data)
			if err != nil {
				return nil, err
			}
			return map[string]any{keyEventType: eventType, keyEventID: eventID}, nil
		},
	}
}
