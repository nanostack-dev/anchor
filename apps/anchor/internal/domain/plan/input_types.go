package plan

type CreatePlanInput struct {
	ProductID    string       `json:"product_id"   validate:"required,notblank"`
	Key          string       `json:"key"          validate:"required,notblank,max=100"`
	Name         string       `json:"name"         validate:"required,notblank,max=100"`
	Description  string       `json:"description"  validate:"omitempty,max=1000"`
	Entitlements Entitlements `json:"entitlements"`
	IsDefault    bool         `json:"is_default"`
}

type UpdatePlanInput struct {
	ProductID    string        `json:"product_id"             validate:"required,notblank"`
	PlanID       string        `json:"plan_id"                validate:"required,notblank"`
	Name         *string       `json:"name,omitempty"         validate:"omitempty,notblank,max=100"`
	Description  *string       `json:"description,omitempty"  validate:"omitempty,max=1000"`
	Entitlements *Entitlements `json:"entitlements,omitempty"`
	IsDefault    *bool         `json:"is_default,omitempty"`
}

type GetPlanInput struct {
	ProductID string `json:"product_id" validate:"required,notblank"`
	PlanID    string `json:"plan_id"    validate:"required,notblank"`
}

type DeletePlanInput struct {
	ProductID string `json:"product_id" validate:"required,notblank"`
	PlanID    string `json:"plan_id"    validate:"required,notblank"`
}

type ListPlansInput struct {
	ProductID string `json:"product_id" validate:"required,notblank"`
}
