package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/view"
	"anchor/internal/domain/license"
)

var _ UsageSeriesRepository = (*usageSeriesRepositoryImpl)(nil)

type usageSeriesRepositoryImpl struct {
	db     *sql.DB
	logger zerolog.Logger
}

func NewUsageSeriesRepository(db *sql.DB, logger zerolog.Logger) UsageSeriesRepository {
	return &usageSeriesRepositoryImpl{
		db:     db,
		logger: logger.With().Str("component", "usage_series_repository").Logger(),
	}
}

// Read selects the one continuous-aggregate view the requested granularity
// names. Building the aggregate itself already happened once, in SQL, when
// the migration created it — see
// docs/adr/0005-timescaledb-for-usage-history.md. This is a plain filtered,
// paginated SELECT onto that result.
//
// The three levels are three distinct generated Go types with no shared
// interface: go-jet's query-result mapping scans into the exact type its own
// generator produced for a relation, and silently returns zero rows for a
// hand-rolled struct — however precisely its field names otherwise match the
// columns. That is what forces a branch per level rather than one query
// built from columns picked at runtime.
func (r *usageSeriesRepositoryImpl) Read(
	ctx context.Context, in license.GetUsageSeriesInput,
) (search.Result[license.UsageSeriesPoint], error) {
	switch in.Granularity {
	case license.UsageGranularityHour:
		v := view.UsageObservationsHour
		return readUsageSeriesLevel(
			ctx, r.db, v, v.AllColumns,
			usageSeriesWhere(v.PlatformTenantID, v.ProductID, v.OrganizationID, v.Key, v.Bucket, in),
			v.Bucket, in.Pagination, mapUsageObservationsHourToPoint,
		)
	case license.UsageGranularityDay:
		v := view.UsageObservationsDay
		return readUsageSeriesLevel(
			ctx, r.db, v, v.AllColumns,
			usageSeriesWhere(v.PlatformTenantID, v.ProductID, v.OrganizationID, v.Key, v.Bucket, in),
			v.Bucket, in.Pagination, mapUsageObservationsDayToPoint,
		)
	case license.UsageGranularityMinute:
		fallthrough
	default:
		// The service layer's oneof validation refuses anything else before this
		// is ever reached; minute is the finest level and the safest default.
		v := view.UsageObservationsMinute
		return readUsageSeriesLevel(
			ctx, r.db, v, v.AllColumns,
			usageSeriesWhere(v.PlatformTenantID, v.ProductID, v.OrganizationID, v.Key, v.Bucket, in),
			v.Bucket, in.Pagination, mapUsageObservationsMinuteToPoint,
		)
	}
}

// usageSeriesWhere is the one predicate every level filters by: the
// (tenant, product, organization, key) coordinate a bucket belongs to, and
// the requested [From, To) range. The columns differ by identity per level
// even though their types match, so this takes them rather than a table.
func usageSeriesWhere(
	tenant, product, organization, key postgres.ColumnString,
	bucket postgres.ColumnTimestampz,
	in license.GetUsageSeriesInput,
) postgres.BoolExpression {
	return tenant.EQ(postgres.String(in.TenantID)).
		AND(product.EQ(postgres.String(in.ProductID))).
		AND(organization.EQ(postgres.String(in.OrganizationID))).
		AND(key.EQ(postgres.String(in.Key))).
		AND(bucket.GT_EQ(postgres.TimestampzT(*in.From))).
		AND(bucket.LT(postgres.TimestampzT(*in.To)))
}

// readUsageSeriesLevel runs the count-then-page query for one level of the
// cascade and maps it into the granularity-independent domain shape.
func readUsageSeriesLevel[M any](
	ctx context.Context, db *sql.DB,
	from postgres.ReadableTable, columns postgres.ColumnList,
	where postgres.BoolExpression, bucket postgres.ColumnTimestampz,
	pagination search.Pagination, mapFunc func(M) license.UsageSeriesPoint,
) (search.Result[license.UsageSeriesPoint], error) {
	return transactor.Page(db, mapFunc, columns).
		From(from).
		Where(where).
		OrderBy(bucket.ASC()).
		Run(ctx, pagination).
		Value()
}

func mapUsageObservationsMinuteToPoint(m model.UsageObservationsMinute) license.UsageSeriesPoint {
	return usageSeriesPointFromColumns(m.Bucket, m.Value, m.WindowFrom, m.WindowTo)
}

func mapUsageObservationsHourToPoint(m model.UsageObservationsHour) license.UsageSeriesPoint {
	return usageSeriesPointFromColumns(m.Bucket, m.Value, m.WindowFrom, m.WindowTo)
}

func mapUsageObservationsDayToPoint(m model.UsageObservationsDay) license.UsageSeriesPoint {
	return usageSeriesPointFromColumns(m.Bucket, m.Value, m.WindowFrom, m.WindowTo)
}

// usageSeriesPointFromColumns fills a point from a continuous-aggregate row.
// Every generated view column is a pointer, because Postgres reports every
// view column as nullable regardless of the underlying data — never nil for
// bucket or value in practice, since a bucket exists only where last()
// produced one, but the type carries the possibility.
func usageSeriesPointFromColumns(
	bucket *time.Time, value *float64, windowFrom, windowTo *time.Time,
) license.UsageSeriesPoint {
	point := license.UsageSeriesPoint{WindowFrom: windowFrom, WindowTo: windowTo}
	if bucket != nil {
		point.Bucket = *bucket
	}
	if value != nil {
		point.Value = *value
	}
	return point
}
