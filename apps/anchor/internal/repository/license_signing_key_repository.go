package repository

import (
	"context"
	"database/sql"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/license"
	"anchor/internal/mapper"
)

var _ LicenseSigningKeyRepository = (*licenseSigningKeyRepositoryImpl)(nil)

// LicenseSigningKeyRepository manages the deployment-global Ed25519 license
// signing keys. The table has no tenant or product dimension by design: keys
// sign tokens for every product, public keys are public by nature, and private
// keys are stored VersionedCipher-encrypted. Handlers must only ever expose
// the public fields (kid, public key, status).
type LicenseSigningKeyRepository interface {
	// FindActive returns the newest ACTIVE signing key, if any.
	FindActive(ctx context.Context) (*license.SigningKey, error)
	// ListAll returns every signing key, newest first.
	ListAll(ctx context.Context) ([]license.SigningKey, error)
	Create(ctx context.Context, key license.SigningKey) (license.SigningKey, error)
}

type licenseSigningKeyRepositoryImpl struct {
	db            *sql.DB
	licenseMapper *mapper.LicenseMapper
	logger        zerolog.Logger
}

func NewLicenseSigningKeyRepository(
	db *sql.DB, licenseMapper *mapper.LicenseMapper, logger zerolog.Logger,
) LicenseSigningKeyRepository {
	return &licenseSigningKeyRepositoryImpl{
		db:            db,
		licenseMapper: licenseMapper,
		logger: logger.With().Str(
			"component", "license_signing_key_repository",
		).Logger(),
	}
}

func (r *licenseSigningKeyRepositoryImpl) FindActive(
	ctx context.Context,
) (*license.SigningKey, error) {
	stmt := table.LicenseSigningKeys.SELECT(
		table.LicenseSigningKeys.AllColumns,
	).WHERE(
		table.LicenseSigningKeys.Status.EQ(
			postgres.String(string(license.SigningKeyStatusActive)),
		),
	).ORDER_BY(table.LicenseSigningKeys.CreatedAt.DESC()).LIMIT(1)

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt, r.licenseMapper.SigningKeyToDomain,
	)
}

func (r *licenseSigningKeyRepositoryImpl) ListAll(
	ctx context.Context,
) ([]license.SigningKey, error) {
	stmt := table.LicenseSigningKeys.SELECT(
		table.LicenseSigningKeys.AllColumns,
	).ORDER_BY(table.LicenseSigningKeys.CreatedAt.DESC())

	return transactor.QueryMapSlice(
		ctx, r.db, stmt, r.licenseMapper.SigningKeyToDomain,
	)
}

func (r *licenseSigningKeyRepositoryImpl) Create(
	ctx context.Context, key license.SigningKey,
) (license.SigningKey, error) {
	entity := r.licenseMapper.SigningKeyToEntity(key)

	stmt := table.LicenseSigningKeys.INSERT(
		table.LicenseSigningKeys.AllColumns.Except(
			table.LicenseSigningKeys.CreatedAt, table.LicenseSigningKeys.UpdatedAt,
		),
	).MODEL(entity).RETURNING(table.LicenseSigningKeys.AllColumns)

	created, err := transactor.Query[model.LicenseSigningKeys](ctx, r.db, stmt)
	if err != nil {
		return license.SigningKey{}, err
	}

	return r.licenseMapper.SigningKeyToDomain(created), nil
}
