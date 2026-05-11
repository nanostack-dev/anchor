package provider

import (
	"context"
	"fmt"
	"os"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = (*anchorProvider)(nil)

func New() provider.Provider {
	return &anchorProvider{}
}

type anchorProvider struct{}

type anchorProviderModel struct {
	BaseURL types.String `tfsdk:"base_url"`
	Token   types.String `tfsdk:"token"`
	APIKey  types.String `tfsdk:"api_key"`
	Product types.String `tfsdk:"product_id"`
}

type providerData struct {
	client    *nanoclient.ClientWithResponses
	productID string
}

const requestEditorCapacity = 2

func (p *anchorProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "anchor"
}

func (p *anchorProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Anchor provider for managing products, product roles, and product resource permissions.",
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "Anchor API base URL. Can also be set with ANCHOR_BASE_URL.",
			},
			"token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Platform bearer token. Can also be set with ANCHOR_TOKEN.",
			},
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Product API key sent as X-Product-API-Key. Can also be set with ANCHOR_API_KEY.",
			},
			"product_id": schema.StringAttribute{
				Optional:    true,
				Description: "Default product ID for product-scoped resources. Can also be set with ANCHOR_PRODUCT_ID.",
			},
		},
	}
}

func (p *anchorProvider) Configure(
	ctx context.Context,
	req provider.ConfigureRequest,
	resp *provider.ConfigureResponse,
) {
	var data anchorProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseURL := os.Getenv("ANCHOR_BASE_URL")
	if baseURL == "" {
		baseURL = "https://anchorapi.nanostack.dev"
	}

	token := os.Getenv("ANCHOR_TOKEN")
	apiKey := os.Getenv("ANCHOR_API_KEY")
	productID := os.Getenv("ANCHOR_PRODUCT_ID")

	if !data.BaseURL.IsNull() && !data.BaseURL.IsUnknown() {
		baseURL = data.BaseURL.ValueString()
	}
	if !data.Token.IsNull() && !data.Token.IsUnknown() {
		token = data.Token.ValueString()
	}
	if !data.APIKey.IsNull() && !data.APIKey.IsUnknown() {
		apiKey = data.APIKey.ValueString()
	}
	if !data.Product.IsNull() && !data.Product.IsUnknown() {
		productID = data.Product.ValueString()
	}

	if token == "" && apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing Anchor Credentials",
			"Set either token/api_key in the provider block or ANCHOR_TOKEN/ANCHOR_API_KEY in the environment.",
		)
	}

	if apiKey != "" && productID == "" {
		resp.Diagnostics.AddError(
			"Missing Product ID Configuration",
			"When using api_key authentication, set product_id in the provider block or ANCHOR_PRODUCT_ID in the environment.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	requestEditors := make([]nanoclient.RequestEditorFn, 0, requestEditorCapacity)
	if token != "" {
		requestEditors = append(requestEditors, bearerTokenEditor(token))
	}
	if apiKey != "" {
		requestEditors = append(requestEditors, productAPIKeyEditor(apiKey))
	}

	client, err := nanoclient.NewClientWithConfig(nanoclient.Config{
		BaseURL:        baseURL,
		RequestEditors: requestEditors,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Configure Anchor API Client",
			fmt.Sprintf("Failed to create API client: %v", err),
		)
		return
	}

	providerData := &providerData{client: client, productID: productID}
	resp.ResourceData = providerData
	resp.DataSourceData = providerData
}

func (p *anchorProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProductResource,
		NewProductRoleResource,
		NewProductPermissionResource,
	}
}

func (p *anchorProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
