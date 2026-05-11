package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*productPermissionResource)(nil)
	_ resource.ResourceWithConfigure   = (*productPermissionResource)(nil)
	_ resource.ResourceWithImportState = (*productPermissionResource)(nil)
)

func NewProductPermissionResource() resource.Resource {
	return &productPermissionResource{}
}

type productPermissionResource struct {
	client           *nanoclient.ClientWithResponses
	defaultProductID string
}

type productPermissionResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ProductID     types.String `tfsdk:"product_id"`
	Name          types.String `tfsdk:"name"`
	Description   types.String `tfsdk:"description"`
	ScopeModifier types.String `tfsdk:"scope_modifier"`
}

func (r *productPermissionResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_product_permission"
}

func (r *productPermissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Anchor product resource permission.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite identifier: <product_id>:<name>.",
			},
			"product_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Product KSUID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Permission name, for example flows:create.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Optional permission description.",
			},
			"scope_modifier": schema.StringAttribute{
				Optional:    true,
				Description: "Optional scope modifier (for example own or team).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *productPermissionResource) Configure(
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
	r.defaultProductID = data.productID
}

func (r *productPermissionResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan productPermissionResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	productID, diags := resolveProductID(plan.ProductID, r.defaultProductID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := r.client.CreateProductResourcePermissionWithResponse(
		ctx,
		productID,
		nanoclient.CreateProductResourcePermissionJSONRequestBody{
			Name:          plan.Name.ValueString(),
			Description:   stringPtrFromTFValue(plan.Description.ValueString()),
			ScopeModifier: stringPtrFromTFValue(plan.ScopeModifier.ValueString()),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Product Permission", err.Error())
		return
	}

	if createResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Unable to Create Product Permission",
			formatAPIError("create product permission", createResp.StatusCode(), createResp.Body),
		)
		return
	}

	state := productPermissionResourceModel{
		ID: types.StringValue(
			buildProductPermissionID(createResp.JSON201.ProductId, createResp.JSON201.Name),
		),
		ProductID:     types.StringValue(createResp.JSON201.ProductId),
		Name:          types.StringValue(createResp.JSON201.Name),
		Description:   types.StringPointerValue(createResp.JSON201.Description),
		ScopeModifier: types.StringPointerValue(createResp.JSON201.ScopeModifier),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *productPermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state productPermissionResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.GetProductResourcePermissionWithResponse(
		ctx,
		state.ProductID.ValueString(),
		state.Name.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Product Permission", err.Error())
		return
	}

	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if getResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unable to Read Product Permission",
			formatAPIError("get product permission", getResp.StatusCode(), getResp.Body),
		)
		return
	}

	state.ID = types.StringValue(buildProductPermissionID(getResp.JSON200.ProductId, getResp.JSON200.Name))
	state.ProductID = types.StringValue(getResp.JSON200.ProductId)
	state.Name = types.StringValue(getResp.JSON200.Name)
	state.Description = types.StringPointerValue(getResp.JSON200.Description)
	state.ScopeModifier = types.StringPointerValue(getResp.JSON200.ScopeModifier)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *productPermissionResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan productPermissionResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	productID, diags := resolveProductID(plan.ProductID, r.defaultProductID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateResp, err := r.client.UpdateProductResourcePermissionWithResponse(
		ctx,
		productID,
		plan.Name.ValueString(),
		nanoclient.UpdateProductResourcePermissionJSONRequestBody{
			Description: stringPtrFromTFValue(plan.Description.ValueString()),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Update Product Permission", err.Error())
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unable to Update Product Permission",
			formatAPIError("update product permission", updateResp.StatusCode(), updateResp.Body),
		)
		return
	}

	state := productPermissionResourceModel{
		ID: types.StringValue(
			buildProductPermissionID(updateResp.JSON200.ProductId, updateResp.JSON200.Name),
		),
		ProductID:     types.StringValue(updateResp.JSON200.ProductId),
		Name:          types.StringValue(updateResp.JSON200.Name),
		Description:   types.StringPointerValue(updateResp.JSON200.Description),
		ScopeModifier: types.StringPointerValue(updateResp.JSON200.ScopeModifier),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *productPermissionResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state productPermissionResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.DeleteProductResourcePermissionWithResponse(
		ctx,
		state.ProductID.ValueString(),
		state.Name.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Delete Product Permission", err.Error())
		return
	}

	if deleteResp.StatusCode() != http.StatusNoContent && deleteResp.StatusCode() != http.StatusNotFound {
		resp.Diagnostics.AddError(
			"Unable to Delete Product Permission",
			formatAPIError("delete product permission", deleteResp.StatusCode(), deleteResp.Body),
		)
		return
	}
}

func (r *productPermissionResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	parts := strings.Split(req.ID, ":")
	if len(parts) < 3 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format product_id:permission_name, got: %q", req.ID),
		)
		return
	}

	productID := parts[0]
	permissionName := strings.Join(parts[1:], ":")

	if strings.TrimSpace(permissionName) == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format product_id:permission_name, got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("product_id"), productID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), permissionName)...)
	resp.Diagnostics.Append(
		resp.State.SetAttribute(
			ctx,
			path.Root("id"),
			buildProductPermissionID(productID, permissionName),
		)...,
	)
}

func buildProductPermissionID(productID, permissionName string) string {
	return productID + ":" + permissionName
}
