package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*productRoleResource)(nil)
	_ resource.ResourceWithConfigure   = (*productRoleResource)(nil)
	_ resource.ResourceWithImportState = (*productRoleResource)(nil)
)

func NewProductRoleResource() resource.Resource {
	return &productRoleResource{}
}

type productRoleResource struct {
	client           *nanoclient.ClientWithResponses
	defaultProductID string
}

type productRoleResourceModel struct {
	ID          types.String `tfsdk:"id"`
	ProductID   types.String `tfsdk:"product_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Permissions types.Set    `tfsdk:"permissions"`
}

func (r *productRoleResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_product_role"
}

func (r *productRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Anchor product role.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Product role KSUID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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
				Description: "Role name.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Optional role description.",
			},
			"permissions": schema.SetAttribute{
				Optional:    true,
				Description: "Resource permission names assigned to this role.",
				ElementType: types.StringType,
			},
		},
	}
}

func (r *productRoleResource) Configure(
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

func (r *productRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan productRoleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	permissions, diags := setToStringSlice(ctx, plan.Permissions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	productID, diags := resolveProductID(plan.ProductID, r.defaultProductID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := r.client.CreateProductRoleWithResponse(
		ctx,
		productID,
		nanoclient.CreateProductRoleJSONRequestBody{
			Name:        plan.Name.ValueString(),
			Description: stringPtrFromTFValue(plan.Description.ValueString()),
			Permissions: permissions,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Product Role", err.Error())
		return
	}

	if createResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Unable to Create Product Role",
			formatAPIError("create product role", createResp.StatusCode(), createResp.Body),
		)
		return
	}

	state, diags := productRoleStateFromAPI(ctx, createResp.JSON201)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *productRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state productRoleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.GetProductRoleWithResponse(ctx, state.ProductID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Product Role", err.Error())
		return
	}

	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if getResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unable to Read Product Role",
			formatAPIError("get product role", getResp.StatusCode(), getResp.Body),
		)
		return
	}

	updatedState, diags := productRoleStateFromAPI(ctx, getResp.JSON200)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &updatedState)...)
}

func (r *productRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state productRoleResourceModel
	var plan productRoleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planPermissions, diags := setToStringSlice(ctx, plan.Permissions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	statePermissions, diags := setToStringSlice(ctx, state.Permissions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	productID, diags := resolveProductID(plan.ProductID, r.defaultProductID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()

	updateResp, err := r.client.UpdateProductRoleWithResponse(
		ctx,
		productID,
		plan.ID.ValueString(),
		nanoclient.UpdateProductRoleJSONRequestBody{
			Name:        name,
			Description: stringPtrFromTFValue(plan.Description.ValueString()),
			Permissions: nil,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Update Product Role", err.Error())
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unable to Update Product Role",
			formatAPIError("update product role", updateResp.StatusCode(), updateResp.Body),
		)
		return
	}

	for _, permissionName := range difference(planPermissions, statePermissions) {
		assignResp, assignErr := r.client.AssignPermissionToProductRoleWithResponse(
			ctx,
			productID,
			plan.ID.ValueString(),
			nanoclient.AssignPermissionToProductRoleJSONRequestBody{PermissionName: permissionName},
		)
		if assignErr != nil {
			resp.Diagnostics.AddError("Unable to Assign Product Role Permission", assignErr.Error())
			return
		}
		if assignResp.StatusCode() != http.StatusNoContent {
			resp.Diagnostics.AddError(
				"Unable to Assign Product Role Permission",
				formatAPIError("assign role permission", assignResp.StatusCode(), assignResp.Body),
			)
			return
		}
	}

	for _, permissionName := range difference(statePermissions, planPermissions) {
		unassignResp, unassignErr := r.client.UnassignPermissionFromProductRoleWithResponse(
			ctx,
			productID,
			plan.ID.ValueString(),
			permissionName,
		)
		if unassignErr != nil {
			resp.Diagnostics.AddError("Unable to Unassign Product Role Permission", unassignErr.Error())
			return
		}
		if unassignResp.StatusCode() != http.StatusNoContent && unassignResp.StatusCode() != http.StatusNotFound {
			resp.Diagnostics.AddError(
				"Unable to Unassign Product Role Permission",
				formatAPIError("unassign role permission", unassignResp.StatusCode(), unassignResp.Body),
			)
			return
		}
	}

	getResp, err := r.client.GetProductRoleWithResponse(ctx, productID, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Product Role", err.Error())
		return
	}
	if getResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unable to Read Product Role",
			formatAPIError("get product role", getResp.StatusCode(), getResp.Body),
		)
		return
	}

	state, diags = productRoleStateFromAPI(ctx, getResp.JSON200)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func difference(left, right []string) []string {
	rightSet := make(map[string]struct{}, len(right))
	for _, item := range right {
		rightSet[item] = struct{}{}
	}

	result := make([]string, 0, len(left))
	for _, item := range left {
		if _, exists := rightSet[item]; !exists {
			result = append(result, item)
		}
	}

	return result
}

func (r *productRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state productRoleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.DeleteProductRoleWithResponse(
		ctx,
		state.ProductID.ValueString(),
		state.ID.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Delete Product Role", err.Error())
		return
	}

	if deleteResp.StatusCode() != http.StatusNoContent && deleteResp.StatusCode() != http.StatusNotFound {
		resp.Diagnostics.AddError(
			"Unable to Delete Product Role",
			formatAPIError("delete product role", deleteResp.StatusCode(), deleteResp.Body),
		)
		return
	}
}

func (r *productRoleResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format product_id:role_id, got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("product_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func productRoleStateFromAPI(
	ctx context.Context,
	role *nanoclient.ProductRoleResponse,
) (productRoleResourceModel, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	permissions := make([]string, 0, len(role.Permissions))
	for _, permission := range role.Permissions {
		permissions = append(permissions, permission.PermissionName)
	}

	permissionsSet, setDiags := types.SetValueFrom(ctx, types.StringType, permissions)
	diags.Append(setDiags...)

	return productRoleResourceModel{
		ID:          types.StringValue(role.Id),
		ProductID:   types.StringValue(role.ProductId),
		Name:        types.StringValue(role.Name),
		Description: types.StringPointerValue(role.Description),
		Permissions: permissionsSet,
	}, diags
}
