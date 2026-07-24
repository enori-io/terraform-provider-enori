package provider

// Enori provider definition. STATUS: first-draft, uncompiled — see main.go.

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure EnoriProvider satisfies the provider.Provider interface.
var _ provider.Provider = &EnoriProvider{}

type EnoriProvider struct {
	version string
}

// EnoriProviderModel maps provider schema data to a Go type.
type EnoriProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &EnoriProvider{version: version}
	}
}

func (p *EnoriProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "enori"
	resp.Version = p.version
}

func (p *EnoriProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Enori provider manages uptime monitors and alert channels via the Enori public REST API.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Enori API base URL. Defaults to `https://api.enori.io`. May also be set via the `ENORI_ENDPOINT` environment variable.",
				Optional:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "Enori API key (create one in the dashboard under Settings → API Keys). Requires the `monitors:read`, `monitors:write`, `alerts:read`, and `alerts:write` scopes for full provider functionality. May also be set via the `ENORI_API_KEY` environment variable (preferred — keeps the key out of state/plan files).",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *EnoriProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config EnoriProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Precedence: explicit config value > environment variable > default.
	endpoint := os.Getenv("ENORI_ENDPOINT")
	if !config.Endpoint.IsNull() {
		endpoint = config.Endpoint.ValueString()
	}
	if endpoint == "" {
		endpoint = defaultBaseURL
	}

	apiKey := os.Getenv("ENORI_API_KEY")
	if !config.APIKey.IsNull() {
		apiKey = config.APIKey.ValueString()
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing Enori API key",
			"Set the api_key provider attribute or the ENORI_API_KEY environment variable. "+
				"Create a key in the dashboard under Settings → API Keys with the monitors:read/write and alerts:read/write scopes.",
		)
		return
	}

	client := NewClient(endpoint, apiKey)
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *EnoriProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewMonitorResource,
		// P2: NewAlertChannelResource,
	}
}

func (p *EnoriProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil // none for the MVP
}
