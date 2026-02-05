package provider

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-mycloud/internal/core"
	"terraform-provider-mycloud/internal/generated"
)

var _ provider.Provider = &NubesProvider{}

type NubesProvider struct {
	version string
}

type NubesProviderModel struct {
	ApiEndpoint types.String `tfsdk:"api_endpoint"`
	ApiToken    types.String `tfsdk:"api_token"`
}

type NubesClient struct {
	HttpClient  *http.Client
	ApiEndpoint string
	ApiToken    string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &NubesProvider{
			version: version,
		}
	}
}

func (p *NubesProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "nubes"
	resp.Version = p.version
}

func (p *NubesProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_endpoint": schema.StringAttribute{
				MarkdownDescription: "API Gateway endpoint for Nubes Cloud",
				Optional:            true,
			},
			"api_token": schema.StringAttribute{
				MarkdownDescription: "API authentication token",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *NubesProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config NubesProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Default values
	apiEndpoint := "https://deck-api.ngcloud.ru/api/v1/index.cfm"
	apiToken := ""

	if !config.ApiEndpoint.IsNull() {
		apiEndpoint = config.ApiEndpoint.ValueString()
	}

	if !config.ApiToken.IsNull() {
		apiToken = strings.TrimSpace(config.ApiToken.ValueString())
	} else {
		// Try to get from environment
		if token := os.Getenv("NUBES_API_TOKEN"); token != "" {
			apiToken = strings.TrimSpace(token)
		} else {
			// Try to read from ~/.nubes_token file
			homeDir, err := os.UserHomeDir()
			if err == nil {
				tokenFile := homeDir + "/.nubes_token"
				if data, err := os.ReadFile(tokenFile); err == nil {
					apiToken = strings.TrimSpace(string(data))
				}
			}
		}
	}

	// Custom transport based on DefaultTransport
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSHandshakeTimeout = 60 * time.Second

	// Force HTTP/1.1 by setting NextProtos
	// Also ignore cert verification
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"http/1.1"},
		MinVersion:         tls.VersionTLS12,
	}
	transport.ForceAttemptHTTP2 = false

	// Use Core Universal Client
	client := &core.UniversalClient{
		HttpClient: &http.Client{
			Transport: transport,
			Timeout:   300 * time.Second,
		},
		ApiEndpoint: apiEndpoint,
		ApiToken:    apiToken,
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *NubesProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewTubulusResource,
		NewOrganizationResource,
		NewVMResource,
		NewVDCResource,
		NewEdgeResource,
		NewVAppResource,
		NewQuickStartResource,
		NewPostgresResource,
		NewS3BucketResource,
		NewPgAdminResource,
		// Generated Resources
		generated.NewBolvankaResource,
		generated.NewBolvankaUniversalResource,
		generated.NewBolvankaUniversalLifecycleResource,
	}
}

func (p *NubesProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewVDCDataSource,
		NewEdgeDataSource,
		NewVAppDataSource,
		NewServiceInstanceDataSource,
	}
}
