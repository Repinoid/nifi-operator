package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &EdgeDataSource{}

func NewEdgeDataSource() datasource.DataSource {
	return &EdgeDataSource{}
}

type EdgeDataSource struct {
	client *NubesClient
}

type EdgeDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
	Status      types.String `tfsdk:"status"`
	Description types.String `tfsdk:"description"`
}

func (d *EdgeDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge"
}

func (d *EdgeDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source для получения информации о существующем Edge Gateway по имени",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "UUID Edge Gateway",
				Computed:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Имя Edge Gateway для поиска",
				Required:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Статус Edge Gateway",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Описание Edge Gateway",
				Computed:            true,
			},
		},
	}
}

func (d *EdgeDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*NubesClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *NubesClient, got: %T", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *EdgeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EdgeDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", d.client.ApiEndpoint+"/instances", nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(d.client.ApiToken))
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := d.client.HttpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read instances: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read response: %s", err))
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("GET /instances returned %d: %s", httpResp.StatusCode, string(body)))
		return
	}

	var instancesResp struct {
		Results []struct {
			ID          string `json:"id"`
			DisplayName string `json:"name"`
			Status      string `json:"status"`
			Description string `json:"desc"`
			Service     string `json:"svc"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &instancesResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	searchName := data.DisplayName.ValueString()
	for _, instance := range instancesResp.Results {
		if instance.Service == "Сетевой шлюз периметра (Edge)" && instance.DisplayName == searchName {
			data.ID = types.StringValue(instance.ID)
			data.Status = types.StringValue(instance.Status)
			data.Description = types.StringValue(instance.Description)

			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	resp.Diagnostics.AddError(
		"Edge Gateway Not Found",
		fmt.Sprintf("Edge Gateway с именем '%s' не найден. Проверьте имя или создайте новый Edge.", searchName),
	)
}
