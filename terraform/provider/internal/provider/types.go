package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func setToStringSlice(ctx context.Context, value types.Set) ([]string, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}

	items := make([]string, 0, len(value.Elements()))
	if len(value.Elements()) == 0 {
		return items, nil
	}

	diags := value.ElementsAs(ctx, &items, false)
	return items, diags
}

func resolveProductID(productID types.String, defaultProductID string) (string, diag.Diagnostics) {
	if !productID.IsNull() && !productID.IsUnknown() && productID.ValueString() != "" {
		return productID.ValueString(), nil
	}

	if defaultProductID != "" {
		return defaultProductID, nil
	}

	var diags diag.Diagnostics
	diags.AddAttributeError(
		path.Root("product_id"),
		"Missing Product ID",
		"Set product_id on this resource or configure product_id on the anchor provider.",
	)

	return "", diags
}
