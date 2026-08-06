package service

import frameworkvalidate "github.com/nanostack-dev/nanostack-framework/pkg/validate"

func validateStruct(input any) error {
	if err := frameworkvalidate.ValidateStruct(input); err != nil {
		return err
	}

	return nil
}
