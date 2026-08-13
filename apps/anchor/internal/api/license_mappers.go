package api

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/slicex"

	"anchor/internal/domain/license"
)

func mapFieldDeclarationsFromAPI(declarations []LicenseFieldDeclaration) []license.FieldDeclaration {
	out := make([]license.FieldDeclaration, 0, len(declarations))
	for _, d := range declarations {
		fd := license.FieldDeclaration{
			Name:       d.Name,
			Type:       d.Type,
			UsageShape: d.UsageShape,
		}
		if d.Description != nil {
			fd.Description = *d.Description
		}
		if d.Rules != nil {
			fd.Rules = *d.Rules
		}
		out = append(out, fd)
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
		Fields:    slicex.Map(s.Fields, mapLicenseFieldToResponse),
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

func mapLicenseDiffToResponse(d license.OrganizationLicenseDiff) OrganizationLicenseDiffResponse {
	// DiffValues returns nil, deliberately, for "nothing differs" (see its own
	// doc) — but the contract's `differences` is a required, non-nullable
	// array, and slicex.Map preserves a nil input as nil. An identical copy
	// must read back as [], not null.
	differences := slicex.Map(d.Differences, mapLicenseFieldDifferenceToResponse)
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
