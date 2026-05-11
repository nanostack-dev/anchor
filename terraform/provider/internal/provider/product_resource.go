package provider

import (
	"context"
	"fmt"
	"net/http"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*productResource)(nil)
	_ resource.ResourceWithConfigure   = (*productResource)(nil)
	_ resource.ResourceWithImportState = (*productResource)(nil)
)

func NewProductResource() resource.Resource {
	return &productResource{}
}

type productResource struct {
	client *nanoclient.ClientWithResponses
}

type productResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

func (r *productResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_product"
}

func (r *productResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Anchor product.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Product KSUID.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Product name.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Optional product description.",
			},
		},
	}
}

func (r *productResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data Type",
			fmt.Sprintf("Expected *providerData, got: %T", req.ProviderData),
		)
		return
	}

	r.client = data.client
}

func (r *productResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan productResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := r.client.CreateProductWithResponse(ctx, nanoclient.CreateProductJSONRequestBody{
		Name:        plan.Name.ValueString(),
		Description: stringPtrFromTFValue(plan.Description.ValueString()),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Product", err.Error())
		return
	}

	if createResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Unable to Create Product",
			formatAPIError("create product", createResp.StatusCode(), createResp.Body),
		)
		return
	}

	state := productResourceModel{
		ID:          types.StringValue(createResp.JSON201.Id),
		Name:        types.StringValue(createResp.JSON201.Name),
		Description: types.StringPointerValue(createResp.JSON201.Description),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *productResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state productResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.GetProductWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Product", err.Error())
		return
	}

	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if getResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unable to Read Product",
			formatAPIError("get product", getResp.StatusCode(), getResp.Body),
		)
		return
	}

	state.Name = types.StringValue(getResp.JSON200.Name)
	state.Description = types.StringPointerValue(getResp.JSON200.Description)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *productResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan productResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateResp, err := r.client.UpdateProductWithResponse(
		ctx,
		plan.ID.ValueString(),
		nanoclient.UpdateProductJSONRequestBody{
			Name:        plan.Name.ValueString(),
			Description: stringPtrFromTFValue(plan.Description.ValueString()),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Update Product", err.Error())
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unable to Update Product",
			formatAPIError("update product", updateResp.StatusCode(), updateResp.Body),
		)
		return
	}

	state := productResourceModel{
		ID:          types.StringValue(updateResp.JSON200.Id),
		Name:        types.StringValue(updateResp.JSON200.Name),
		Description: types.StringPointerValue(updateResp.JSON200.Description),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *productResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state productResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.DeleteProductWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Delete Product", err.Error())
		return
	}

	if deleteResp.StatusCode() != http.StatusNoContent && deleteResp.StatusCode() != http.StatusNotFound {
		resp.Diagnostics.AddError(
			"Unable to Delete Product",
			formatAPIError("delete product", deleteResp.StatusCode(), deleteResp.Body),
		)
		return
	}
}

func (r *productResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
