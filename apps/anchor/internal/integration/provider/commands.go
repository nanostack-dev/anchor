package provider

import "fmt"

// CommandType represents the type of canonical command produced by a provider.
type CommandType string

// Command is a provider-agnostic command that the integration service executes.
// Each provider defines its own CommandType constants and data structs.
type Command struct {
	Type CommandType
	Data any
}

// ErrInvalidCommandData is returned when a command's Data field cannot be
// type-asserted to the expected concrete type.
func ErrInvalidCommandData(cmdType CommandType, expected string) error {
	return fmt.Errorf("invalid data type for %s command: expected %s", cmdType, expected)
}
