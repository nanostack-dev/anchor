package workflow

type WriteInput struct {
	Name             string  `validate:"required,notblank,min=2,max=100"`
	Description      *string `validate:"omitempty,max=500"`
	Enabled          bool
	TriggerEventType string `validate:"required,notblank"`
	Definition       Definition
}

type CreateInput struct {
	TenantID  string `validate:"required,notblank"`
	ProductID string `validate:"required,notblank"`
	WriteInput
}

type UpdateInput struct {
	TenantID   string `validate:"required,notblank"`
	ProductID  string `validate:"required,notblank"`
	WorkflowID string `validate:"required,notblank"`
	WriteInput
}

type GetInput struct {
	TenantID   string `validate:"required,notblank"`
	ProductID  string `validate:"required,notblank"`
	WorkflowID string `validate:"required,notblank"`
}

type DeleteInput = GetInput

type ListInput struct {
	TenantID  string `validate:"required,notblank"`
	ProductID string `validate:"required,notblank"`
}

type ListRunsInput struct {
	TenantID   string `validate:"required,notblank"`
	ProductID  string `validate:"required,notblank"`
	WorkflowID string `validate:"required,notblank"`
	Limit      int    `validate:"omitempty,min=1,max=100"`
}

// RunInput runs a saved workflow by hand against sample event data, as if
// an event of its trigger type had happened.
type RunInput struct {
	TenantID   string `validate:"required,notblank"`
	ProductID  string `validate:"required,notblank"`
	WorkflowID string `validate:"required,notblank"`
	EventData  map[string]string
}

// DryRunInput evaluates an unsaved workflow against sample event data.
// Read actions run for real so later conditions see real values; write
// actions are only resolved and reported.
type DryRunInput struct {
	TenantID  string `validate:"required,notblank"`
	ProductID string `validate:"required,notblank"`
	WriteInput
	EventData map[string]string
}

type ListProductRunsInput struct {
	TenantID  string `validate:"required,notblank"`
	ProductID string `validate:"required,notblank"`
	Limit     int    `validate:"omitempty,min=1,max=100"`
}
