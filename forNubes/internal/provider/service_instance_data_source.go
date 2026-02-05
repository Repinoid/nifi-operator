package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ServiceInstanceDataSource{}

func NewServiceInstanceDataSource() datasource.DataSource {
	return &ServiceInstanceDataSource{}
}

type ServiceInstanceDataSource struct {
	client *NubesClient
}

type ServiceInstanceDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
	ServiceId   types.Int64  `tfsdk:"service_id"`
	ServiceName types.String `tfsdk:"service_name"`
}

func (d *ServiceInstanceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_instance"
}

func (d *ServiceInstanceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source для получения UUID сервиса (Instance) по его имени (DisplayName). Позволяет использовать human-readable имена (например 's3-111805') вместо UUID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "UUID найденного сервиса (instanceUid)",
				Computed:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Имя (DisplayName) для поиска. Например 's3-111805'",
				Required:            true,
			},
			"service_id": schema.Int64Attribute{
				MarkdownDescription: "ID типа сервиса для дополнительной фильтрации (например 12 для S3). Опционально.",
				Optional:            true,
			},
			"service_name": schema.StringAttribute{
				MarkdownDescription: "Название типа сервиса (например 'S3 Object Storage')",
				Computed:            true,
			},
		},
	}
}

func (d *ServiceInstanceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ServiceInstanceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ServiceInstanceDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	displayName := data.DisplayName.ValueString()
	
	instances, err := d.client.GetInstances(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch instances list: %s", err))
		return
	}

	var foundInstance *InstanceSummary
	
	// Поиск по DisplayName
	for _, inst := range instances {
		if strings.TrimSpace(inst.DisplayName) == strings.TrimSpace(displayName) {
			// Если задан service_id, проверяем и его
			if !data.ServiceId.IsNull() && !data.ServiceId.IsUnknown() {
				if int64(inst.ServiceId) != data.ServiceId.ValueInt64() {
					continue
				}
			}
			foundInstance = &inst
			break
		}
	}

	if foundInstance == nil {
		resp.Diagnostics.AddError(
			"Instance Not Found",
			fmt.Sprintf("Could not find instance with DisplayName '%s'", displayName),
		)
		return
	}

	data.ID = types.StringValue(foundInstance.InstanceUid)
	data.ServiceName = types.StringValue(foundInstance.Svc)
	data.ServiceId = types.Int64Value(int64(foundInstance.ServiceId))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
