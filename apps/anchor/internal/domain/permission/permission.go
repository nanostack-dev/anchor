package permission

import (
	"time"
)

type ProductPermission struct {
	ProductID   string
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
