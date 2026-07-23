package webhook

// deliveryPageLimit is the hard ceiling on a delivery-log page.
const deliveryPageLimit = 200

// DefaultDeliveryPageSize is the page size used when a caller does not ask for
// one on the delivery log.
const DefaultDeliveryPageSize = 50

type ListEndpointsInput struct {
	ProductID string `json:"product_id" validate:"required,notblank"`
}

type GetEndpointInput struct {
	ProductID  string `json:"product_id"  validate:"required,notblank"`
	EndpointID string `json:"endpoint_id" validate:"required,notblank"`
}

type CreateEndpointInput struct {
	ProductID   string   `json:"product_id"  validate:"required,notblank"`
	URL         string   `json:"url"         validate:"required,notblank,max=2000"`
	Description string   `json:"description" validate:"omitempty,max=500"`
	EventTypes  []string `json:"event_types" validate:"required,min=1,max=50,dive,notblank"`
}

type UpdateEndpointInput struct {
	ProductID   string    `json:"product_id"            validate:"required,notblank"`
	EndpointID  string    `json:"endpoint_id"           validate:"required,notblank"`
	URL         *string   `json:"url,omitempty"         validate:"omitempty,notblank,max=2000"`
	Description *string   `json:"description,omitempty" validate:"omitempty,max=500"`
	EventTypes  *[]string `json:"event_types,omitempty" validate:"omitempty,min=1,max=50,dive,notblank"`
}

type DeleteEndpointInput struct {
	ProductID  string `json:"product_id"  validate:"required,notblank"`
	EndpointID string `json:"endpoint_id" validate:"required,notblank"`
}

// SetEndpointEnabledInput backs both the enable and disable sub-resources.
type SetEndpointEnabledInput struct {
	ProductID  string `json:"product_id"  validate:"required,notblank"`
	EndpointID string `json:"endpoint_id" validate:"required,notblank"`
	Enabled    bool   `json:"enabled"`
}

type RotateSecretInput struct {
	ProductID  string `json:"product_id"  validate:"required,notblank"`
	EndpointID string `json:"endpoint_id" validate:"required,notblank"`
}

type PingEndpointInput struct {
	ProductID  string `json:"product_id"  validate:"required,notblank"`
	EndpointID string `json:"endpoint_id" validate:"required,notblank"`
}

type ListDeliveriesInput struct {
	ProductID  string          `json:"product_id"           validate:"required,notblank"`
	EndpointID string          `json:"endpoint_id"          validate:"required,notblank"`
	Status     *DeliveryStatus `json:"status,omitempty"`
	EventType  *string         `json:"event_type,omitempty"`
	Limit      int32           `json:"limit"                validate:"omitempty,min=1,max=200"`
	Offset     int32           `json:"offset"               validate:"omitempty,min=0"`
}

type GetDeliveryInput struct {
	ProductID  string `json:"product_id"  validate:"required,notblank"`
	EndpointID string `json:"endpoint_id" validate:"required,notblank"`
	DeliveryID string `json:"delivery_id" validate:"required,notblank"`
}

type RetryDeliveryInput struct {
	ProductID  string `json:"product_id"  validate:"required,notblank"`
	EndpointID string `json:"endpoint_id" validate:"required,notblank"`
	DeliveryID string `json:"delivery_id" validate:"required,notblank"`
}

// EmitInput is the whole seam other features use to publish an event. It is
// deliberately narrow: adding an event type touches the registry and the emit
// call site, never fan-out, signing, retry, or delivery.
type EmitInput struct {
	ProductID      string  `json:"product_id"                validate:"required,notblank"`
	OrganizationID *string `json:"organization_id,omitempty"`
	EventType      string  `json:"event_type"                validate:"required,notblank"`
	// Data becomes the envelope's `data` object.
	Data any `json:"data"`
	// TargetEndpointID restricts fan-out to a single endpoint. Used by ping.
	TargetEndpointID *string `json:"target_endpoint_id,omitempty"`
}

// NormalizedLimit clamps a requested delivery page size into the allowed range.
func (i ListDeliveriesInput) NormalizedLimit() int32 {
	if i.Limit <= 0 {
		return DefaultDeliveryPageSize
	}
	if i.Limit > deliveryPageLimit {
		return deliveryPageLimit
	}

	return i.Limit
}
