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

var _ datasource.DataSource = &VDCDataSource{}

func NewVDCDataSource() datasource.DataSource {
	return &VDCDataSource{}
}

type VDCDataSource struct {
	client *NubesClient
}

type VDCDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	DisplayName  types.String `tfsdk:"display_name"`
	Status       types.String `tfsdk:"status"`
	Description  types.String `tfsdk:"description"`
}

func (d *VDCDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vdc"
}

func (d *VDCDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source для получения информации о существующем VDC по имени",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "UUID VDC",
				Computed:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Имя VDC для поиска",
				Required:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Статус VDC (running, suspended, etc.)",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Описание VDC",
				Computed:            true,
			},
		},
	}
}

func (d *VDCDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *VDCDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data VDCDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Поиск VDC по имени
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

	// Поиск VDC с указанным именем
	searchName := data.DisplayName.ValueString()
	for _, instance := range instancesResp.Results {
		if instance.Service == "Виртуальный датацентр (vDC)" && instance.DisplayName == searchName {
			data.ID = types.StringValue(instance.ID)
			data.Status = types.StringValue(instance.Status)
			data.Description = types.StringValue(instance.Description)

			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	resp.Diagnostics.AddError(
		"VDC Not Found",
		fmt.Sprintf("VDC с именем '%s' не найден. Проверьте имя или создайте новый VDC.", searchName),
	)
}
