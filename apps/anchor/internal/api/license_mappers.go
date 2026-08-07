package api

import "anchor/internal/domain/license"

func mapFieldDeclarationsFromAPI(declarations []LicenseFieldDeclaration) []license.FieldDeclaration {
	out := make([]license.FieldDeclaration, 0, len(declarations))
	for _, d := range declarations {
		fd := license.FieldDeclaration{
			Name: d.Name,
			Type: d.Type,
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
		Id:        f.ID,
		Name:      f.Name,
		Type:      f.Type,
		Rules:     f.Rules,
		CreatedAt: f.CreatedAt,
		UpdatedAt: f.UpdatedAt,
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
		Fields:    mapItems(s.Fields, mapLicenseFieldToResponse),
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
		Values:    values,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
	if t.Description != "" {
		resp.Description = new(t.Description)
	}
	return resp
}
