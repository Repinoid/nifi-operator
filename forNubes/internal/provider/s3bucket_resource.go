package provider

// [НЕ ИЗМЕНЯТЬ !!!!]
// Данный ресурс реализует паттерн "Nubes Flow" для S3 бакета.
// ВАЖНО: Операция Update (изменение) для S3 бакетов в API Nubes отсутствует.
// Любое изменение атрибутов в Terraform приведет к пересозданию ресурса (RequiresReplace).

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &S3BucketResource{}

type S3BucketResource struct {
	client *NubesClient
}

func NewS3BucketResource() resource.Resource {
	return &S3BucketResource{}
}

type S3BucketResourceModel struct {
	ID         types.String `tfsdk:"id"`
	S3UserUid  types.String `tfsdk:"s3_user_uid"`
	BucketName types.String `tfsdk:"bucket_name"`
	MaxSize    types.Int64  `tfsdk:"max_size"`
	ReadAll    types.Bool   `tfsdk:"read_all"`
	ListAll    types.Bool   `tfsdk:"list_all"`
	CorsAll    types.Bool   `tfsdk:"cors_all"`
	Placement  types.String `tfsdk:"placement"`
}

func (r *S3BucketResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket"
}

func (r *S3BucketResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an S3 Bucket resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"s3_user_uid": schema.StringAttribute{
				Required:    true,
				Description: "UID Корневой услуги S3 (Param 124). Это UUID экземпляра услуги S3 Object Storage (svcId 12). Например: 6d6061cb-b0c1-44b9-8969-a70f08fe673c",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"bucket_name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the bucket (param 125)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"max_size": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(-1),
				Description: "Max size of the bucket, -1 for unlimited (param 126)",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"read_all": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Enable read all (param 127)",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"list_all": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Enable list all (param 128)",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"cors_all": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Enable CORS all (param 129)",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"placement": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("HOT"),
				Description: "Placement strategy (HOT/COLD) (param 130)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *S3BucketResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*NubesClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *NubesClient, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}
	r.client = client
}

func (r *S3BucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3BucketResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Creating S3 Bucket resource...")

	// 1. Подготовка параметров для создания.
	// Каждый параметр соответствует svcOperationCfsParamId из API.
	// Эти ID фиксированы для услуги S3 (ServiceId 13).
	params := []InstanceParam{
		{SvcOperationCfsParamId: 124, ParamValue: plan.S3UserUid.ValueString()},
		{SvcOperationCfsParamId: 125, ParamValue: plan.BucketName.ValueString()},
		{SvcOperationCfsParamId: 126, ParamValue: fmt.Sprintf("%d", plan.MaxSize.ValueInt64())},
		{SvcOperationCfsParamId: 127, ParamValue: strconv.FormatBool(plan.ReadAll.ValueBool())},
		{SvcOperationCfsParamId: 128, ParamValue: strconv.FormatBool(plan.ListAll.ValueBool())},
		{SvcOperationCfsParamId: 129, ParamValue: strconv.FormatBool(plan.CorsAll.ValueBool())},
		{SvcOperationCfsParamId: 130, ParamValue: plan.Placement.ValueString()},
	}

	// Service ID 13 для S3 (бакеты)
	serviceId := 13

	// Динамический поиск ID операции "create".
	// В Nubes для каждого сервиса свой набор ID операций.
	svcOperationId, err := r.client.GetOperationId(ctx, serviceId, "create")
	if err != nil {
		resp.Diagnostics.AddError("Failed to get create operation ID", err.Error())
		return
	}

	// CreateInstance выполняет полный цикл "Nubes Flow":
	// 1. POST /instances (создание объекта)
	// 2. POST /instanceOperations (инициализация визарда)
	// 3. GET /instanceOperations?fields=cfsParams (получение списка ожидаемых параметров)
	// 4. POST /instanceOperationCfsParams (синхронизация значений)
	// 5. POST /run {BODY: {}} (запуск выполнения)
	instanceUid, opUid, err := r.client.CreateInstance(ctx, plan.BucketName.ValueString(), serviceId, svcOperationId, params)
	if err != nil {
		resp.Diagnostics.AddError("Error creating S3 bucket", err.Error())
		return
	}

	// Ожидание завершения операции.
	err = r.client.WaitForOperation(ctx, opUid)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for S3 bucket ready", err.Error())
		return
	}

	plan.ID = types.StringValue(instanceUid)
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *S3BucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3BucketResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Implement Read if necessary. For now, just maintain state.
	// Ideally, fetch instance details and update state.
	// We can use r.client.GetInstance(ctx, state.ID.ValueString()) to check if it exists or is deleted.

	/*
		instance, err := r.client.GetInstance(ctx, state.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error reading S3 bucket", err.Error())
			return
		}
		if instance == nil || instance.IsDeleted {
			resp.State.RemoveResource(ctx)
			return
		}
		// Update fields if needed
	*/

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *S3BucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// [НЕ ИЗМЕНЯТЬ !!!!]
	// API Nubes не поддерживает операцию Modify для S3 бакетов.
	// Этот метод оставлен пустым, так как RequiresReplace в схеме должен предотвращать его вызов для критичных полей.
}

func (r *S3BucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3BucketResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Assume delete operation exists
	// Get "delete" operation ID
	serviceId := 13
	svcOperationId, err := r.client.GetOperationId(ctx, serviceId, "delete")
	if err != nil {
		resp.Diagnostics.AddError("Failed to get delete operation ID", err.Error())
		return
	}

	err = r.client.DeleteInstance(ctx, state.ID.ValueString(), svcOperationId)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting S3 bucket", err.Error())
		return
	}
}
