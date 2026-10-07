package workflow

import (
	"regexp"
	"strings"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

const (
	MaxSteps      = 20
	MaxConditions = 20
	// MaxCausationDepth is the backstop behind the loop guards: an event
	// caused by this many chained runs starts no further run.
	MaxCausationDepth = 5
	// EventQueueName is the queue of events that start workflow runs.
	EventQueueName = "product-workflows"
	// CustomEventPrefix starts the type of every event a workflow emits for
	// other workflows. Custom events never leave Anchor.
	CustomEventPrefix   = "custom."
	maxCustomEventBytes = 100
)

var customEventPattern = regexp.MustCompile(`^custom\.[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)

func IsCustomEvent(eventType string) bool {
	return strings.HasPrefix(eventType, CustomEventPrefix)
}

func ValidCustomEvent(eventType string) bool {
	return len(eventType) <= maxCustomEventBytes && customEventPattern.MatchString(eventType)
}

// CustomEventType names the custom event a step emits, accepting the name
// with or without its prefix.
func CustomEventType(name string) string {
	name = strings.TrimSpace(name)
	if IsCustomEvent(name) {
		return name
	}
	return CustomEventPrefix + name
}

// Workflow is a Product's own automation: when an event of the trigger type
// happens and every condition holds, its steps run in order against the
// Product's resources.
type Workflow struct {
	ID               string
	ProductID        string
	PlatformTenantID string
	Name             string
	Description      *string
	Enabled          bool
	TriggerEventType string
	Definition       Definition
	// LastRun is set only on a read that included it, and stays nil when the
	// workflow never ran.
	LastRun   *Run
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (w *Workflow) GenerateID() {
	w.ID = ids.MustNew("wf")
}

type Definition struct {
	Conditions []Condition `json:"conditions"`
	Steps      []Step      `json:"steps"`
}

// Condition compares the value at Field against Value. Field is a path into
// the run context (`event.data.organization_id`, `steps.user.email_domain`).
// Value may hold `{{ }}` references to the same context.
type Condition struct {
	Field    string   `json:"field"`
	Operator Operator `json:"operator"`
	Value    string   `json:"value,omitempty"`
}

type Operator string

const (
	OperatorEquals      Operator = "equals"
	OperatorNotEquals   Operator = "not_equals"
	OperatorContains    Operator = "contains"
	OperatorNotContains Operator = "not_contains"
	OperatorStartsWith  Operator = "starts_with"
	OperatorEndsWith    Operator = "ends_with"
	OperatorIn          Operator = "in"
	OperatorExists      Operator = "exists"
	OperatorNotExists   Operator = "not_exists"
)

func AllOperators() []Operator {
	return []Operator{
		OperatorEquals, OperatorNotEquals, OperatorContains, OperatorNotContains,
		OperatorStartsWith, OperatorEndsWith, OperatorIn, OperatorExists, OperatorNotExists,
	}
}

func (o Operator) NeedsValue() bool {
	return o != OperatorExists && o != OperatorNotExists
}

// Step runs one action. Params are strings that may reference the run
// context with `{{ path }}`; the step's output becomes `steps.<id>.*` for
// every later step.
type Step struct {
	ID              string            `json:"id"`
	Name            string            `json:"name,omitempty"`
	Action          ActionType        `json:"action"`
	Params          map[string]string `json:"params"`
	When            []Condition       `json:"when,omitempty"`
	ContinueOnError bool              `json:"continue_on_error,omitempty"`
}

type ActionType string
