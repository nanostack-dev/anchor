package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/pgkit/queue"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/license"
	licenserepo "anchor/internal/license/repository"
	"anchor/internal/license/service"
)

func TestTemplateSyncRejectsMalformedPayload(t *testing.T) {
	sync := service.NewLicenseTemplateSyncService(nil, nil, nil, nil, nil, nil, nil, nil, zerolog.Nop())
	for _, payload := range []string{
		`{`, `{}`, `{"tenant_id":" ","product_id":"prd_1","template_id":"ltpl_1"}`,
	} {
		err := sync.ProcessQueueJob(t.Context(), queue.Job{Payload: []byte(payload)})
		require.Error(t, err)
		require.True(t, queue.IsNonRetryable(err))
	}
}

func TestTemplateSyncValidationFailure(t *testing.T) {
	for _, test := range []struct {
		name    string
		err     error
		refused bool
	}{
		{"schema read failure", errors.New("schema lookup failed"), false},
		{"server failure", fault.Internal("SCHEMA_FAILURE", "Schema lookup failed"), false},
		{"invalid values", fault.BadRequest("LICENSE_VALUE_INVALID", "Outside the declared range"), true},
		{"missing schema", service.ErrLicenseSchemaNotDeclared, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			sync := service.NewLicenseTemplateSyncService(
				syncTemplateRepository{}, syncLicenseRepository{}, nil,
				syncSchemaFailure{err: test.err}, syncTransactor{}, nil, nil, nil, zerolog.Nop(),
			)
			err := sync.ProcessQueueJob(t.Context(), queue.Job{Payload: []byte(
				`{"tenant_id":"tenant_1","product_id":"prd_1","template_id":"ltpl_1"}`,
			)})
			if test.refused {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, test.err)
				require.False(t, queue.IsNonRetryable(err))
			}
		})
	}
}

type syncTemplateRepository struct{ licenserepo.TemplateRepository }

func (syncTemplateRepository) FindByID(
	context.Context,
	string,
	string,
	string,
) (functional.Option[license.Template], error) {
	return functional.Some(license.Template{ID: "ltpl_1", Values: license.TemplateValues{"flows": 50}}), nil
}

type syncLicenseRepository struct {
	licenserepo.OrganizationLicenseRepository
}

func (syncLicenseRepository) ListOrganizationIDsForTemplateAfter(
	context.Context, string, string, string, string, int,
) ([]string, error) {
	return []string{"org_1"}, nil
}

func (syncLicenseRepository) FindByOrganizationForUpdate(
	context.Context, string, string, string,
) (functional.Option[license.OrganizationLicense], error) {
	return functional.Some(license.OrganizationLicense{
		TemplateID: "ltpl_1", Values: license.TemplateValues{"flows": 100},
	}), nil
}

type syncSchemaFailure struct {
	service.LicenseSchemaService
	err error
}

func (s syncSchemaFailure) ValidateValues(context.Context, string, string, license.TemplateValues) error {
	return s.err
}

type syncTransactor struct{}

func (syncTransactor) InTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
