package api

import "anchor/internal/domain/license"

func mapFieldDeclarationsFromAPI(declarations []LicenseFieldDeclaration) []license.FieldDeclaration {
	out := make([]license.FieldDeclaration, 0, len(declarations))
	for _, d := range declarations {
		fd := license.FieldDeclaration{
			Name: d.Name,
			Type: d.Type,
		}
		if d.Required != nil {
			fd.Required = *d.Required
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
		Required:  f.Required,
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
