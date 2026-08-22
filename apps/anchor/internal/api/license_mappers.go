package api

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/domain/license"
)

func mapFieldDeclarationsFromAPI(declarations []LicenseFieldDeclaration) []license.FieldDeclaration {
	out := functional.Slice(declarations).Map(func(d LicenseFieldDeclaration) license.FieldDeclaration {
		return license.FieldDeclaration{
			Name:        d.Name,
			Type:        d.Type,
			UsageShape:  d.UsageShape,
			Description: functional.FromPtr(d.Description).OrElse(""),
			Rules:       functional.FromPtr(d.Rules).OrElse(license.FieldRules{}),
		}
	}).ToSlice()
	if out == nil {
		out = []license.FieldDeclaration{}
	}
	return out
}

func mapLicenseFieldToResponse(f license.Field) LicenseFieldResponse {
	resp := LicenseFieldResponse{
		Id:         f.ID,
		Name:       f.Name,
		Type:       f.Type,
		Rules:      f.Rules,
		UsageShape: f.UsageShape,
		CreatedAt:  f.CreatedAt,
		UpdatedAt:  f.UpdatedAt,
	}
	if f.Description != "" {
		resp.Description = new(f.Description)
	}
	return resp
}

func mapLicenseSchemaToResponse(s license.Schema) LicenseSchemaResponse {
	resp := LicenseSchemaResponse{
		Id:        s.ID,
		ProductId: s.ProductID,
		Fields:    functional.Slice(s.Fields).Map(mapLicenseFieldToResponse),
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
	if s.Description != "" {
		resp.Description = new(s.Description)
	}
	return resp
}

func mapLicenseTemplateToResponse(t license.Template) LicenseTemplateResponse {
	// A template that sets nothing reads back as an empty object, never as a
	// null, so a client can index into it without a nil check.
	values := t.Values
	if values == nil {
		values = license.TemplateValues{}
	}
	resp := LicenseTemplateResponse{
		Id:        t.ID,
		ProductId: t.ProductID,
		Name:      t.Name,
		Status:    t.Status,
		Values:    values,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
	if t.Description != "" {
		resp.Description = new(t.Description)
	}
	return resp
}

func mapOrganizationLicenseToResponse(l license.OrganizationLicense) OrganizationLicenseResponse {
	// A license that sets nothing reads back as an empty object, never as a
	// null, so a client can index into it without a nil check.
	values := l.Values
	if values == nil {
		values = license.TemplateValues{}
	}
	return OrganizationLicenseResponse{
		Id:             l.ID,
		ProductId:      l.ProductID,
		OrganizationId: l.OrganizationID,
		TemplateId:     l.TemplateID,
		InstantiatedAt: l.InstantiatedAt,
		Values:         values,
		CreatedAt:      l.CreatedAt,
		UpdatedAt:      l.UpdatedAt,
	}
}

// mapOrganizationLicenseSummaryToResponse carries no usage: a page of results
// would otherwise cost as many usage derivations as it has rows.
func mapOrganizationLicenseSummaryToResponse(
	s license.OrganizationLicenseSummary,
) OrganizationLicenseSummaryResponse {
	resp := OrganizationLicenseSummaryResponse{
		OrganizationId:   s.OrganizationID,
		OrganizationName: s.OrganizationName,
	}
	if s.License != nil {
		resp.License = new(mapOrganizationLicenseToResponse(*s.License))
	}
	return resp
}

func mapFieldUsageToResponse(u license.FieldUsage) LicenseFieldUsageResponse {
	return LicenseFieldUsageResponse{
		Limit:          u.Limit,
		Usage:          u.Usage,
		Status:         u.Status,
		LastReportedAt: u.LastReportedAt,
	}
}

// mapOrganizationLicenseReadToResponse is mapOrganizationLicenseToResponse
// plus the per-limit usage a license read carries. Only GetOrganizationLicense
// uses it: instantiating or adjusting a license returns
// mapOrganizationLicenseToResponse directly, without usage, since neither
// write computes it.
func mapOrganizationLicenseReadToResponse(l license.OrganizationLicenseRead) OrganizationLicenseResponse {
	resp := mapOrganizationLicenseToResponse(l.OrganizationLicense)
	usage := make(map[string]LicenseFieldUsageResponse, len(l.Usage))
	for name, u := range l.Usage {
		usage[name] = mapFieldUsageToResponse(u)
	}
	resp.Usage = &usage
	return resp
}

func mapOrganizationLicenseChangeToResponse(
	c license.OrganizationLicenseChange,
) OrganizationLicenseChangeResponse {
	return OrganizationLicenseChangeResponse{
		Id:                 c.ID,
		ProductId:          c.ProductID,
		OrganizationId:     c.OrganizationID,
		LicenseId:          c.LicenseID,
		Type:               c.Type,
		TemplateId:         c.TemplateID,
		PreviousTemplateId: c.PreviousTemplateID,
		Field:              c.Field,
		OldValue:           c.OldValue,
		NewValue:           c.NewValue,
		ChangedAt:          c.ChangedAt,
	}
}

func mapUsageObservationToResponse(o license.UsageObservation) UsageObservationResponse {
	return UsageObservationResponse{
		Id:             o.ID,
		ProductId:      o.ProductID,
		OrganizationId: o.OrganizationID,
		Key:            o.Key,
		Value:          o.Value,
		From:           o.From,
		To:             o.To,
		ObservedAt:     o.ObservedAt,
	}
}

func mapUsageSeriesPointToResponse(p license.UsageSeriesPoint) UsageSeriesPointResponse {
	return UsageSeriesPointResponse{
		Bucket: p.Bucket,
		Value:  p.Value,
		From:   p.WindowFrom,
		To:     p.WindowTo,
	}
}

func mapLicenseFieldDifferenceToResponse(d license.FieldDifference) LicenseFieldDifference {
	return LicenseFieldDifference{
		Field:         d.Field,
		Kind:          d.Kind,
		LicenseValue:  d.LicenseValue,
		TemplateValue: d.TemplateValue,
	}
}

// mapLicenseMigrationFailureToResponse reports why one organization was left
// behind. A fault carries the machine-readable code a caller retries or
// escalates on; anything else reached the handler unwrapped, so it reads as the
// generic unexpected error rather than leaking an internal message into a
// per-organization result.
func mapLicenseMigrationFailureToResponse(err error) *ApiError {
	if err == nil {
		return nil
	}
	if faulted, ok := fault.As(err); ok && len(faulted.Details) > 0 {
		return &faulted.Details[0]
	}
	return &ApiError{Code: fault.CodeUnexpected, Message: "Unexpected error"}
}

func mapLicenseMigrationResultToResponse(
	r license.OrganizationMigrationResult,
) OrganizationLicenseMigrationResult {
	// DiffValues returns nil for "nothing differs"; Seq.Map preserves that,
	// but changes is a required, non-nullable array in the contract.
	changes := functional.Slice(r.Changes).Map(mapLicenseFieldDifferenceToResponse)
	if changes == nil {
		changes = []LicenseFieldDifference{}
	}
	return OrganizationLicenseMigrationResult{
		OrganizationId:     r.OrganizationID,
		Outcome:            r.Outcome,
		PreviousTemplateId: r.PreviousTemplateID,
		Changes:            changes,
		Count:              len(changes),
		Error:              mapLicenseMigrationFailureToResponse(r.Error),
	}
}

func mapLicenseMigrationToResponse(m license.Migration) OrganizationLicenseMigrationResponse {
	tally := m.Tally()
	return OrganizationLicenseMigrationResponse{
		TemplateId: m.TemplateID,
		MigratedAt: m.MigratedAt,
		Results:    functional.Slice(m.Results).Map(mapLicenseMigrationResultToResponse),
		Count:      len(m.Results),
		Changed:    tally.Changed,
		Unchanged:  tally.Unchanged,
		Failed:     tally.Failed,
	}
}

func mapLicenseDiffToResponse(d license.OrganizationLicenseDiff) OrganizationLicenseDiffResponse {
	// DiffValues returns nil for "nothing differs"; Seq.Map preserves that,
	// but differences is a required, non-nullable array in the contract.
	differences := functional.Slice(d.Differences).Map(mapLicenseFieldDifferenceToResponse)
	if differences == nil {
		differences = []LicenseFieldDifference{}
	}
	return OrganizationLicenseDiffResponse{
		OrganizationId: d.OrganizationID,
		TemplateId:     d.TemplateID,
		Differences:    differences,
		Count:          len(differences),
	}
}
