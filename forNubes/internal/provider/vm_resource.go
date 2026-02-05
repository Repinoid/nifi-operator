package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &VMResource{}
var _ resource.ResourceWithImportState = &VMResource{}

func NewVMResource() resource.Resource {
	return &VMResource{}
}

type VMResource struct {
	client *NubesClient
}

type VMResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	DisplayName           types.String `tfsdk:"display_name"`
	Description           types.String `tfsdk:"description"`
	Status                types.String `tfsdk:"status"`
	VappUid               types.String `tfsdk:"vapp_uid"`
	VmName                types.String `tfsdk:"vm_name"`
	VmCpu                 types.Int64  `tfsdk:"vm_cpu"`
	VmRam                 types.Int64  `tfsdk:"vm_ram"`
	AccessPortList        types.String `tfsdk:"access_port_list"`
	ImageVm               types.String `tfsdk:"image_vm"`
	UserLogin             types.String `tfsdk:"user_login"`
	UserPublicKey         types.String `tfsdk:"user_public_key"`
	NeedAddZabbixTemplate types.Bool   `tfsdk:"need_add_zabbix_template"`
	AccessIpList          types.String `tfsdk:"access_ip_list"`
	VmDisk                types.Int64  `tfsdk:"vm_disk"`
	IpSpaceName           types.String `tfsdk:"ip_space_name"`
	CloudInit             types.String `tfsdk:"cloud_init"`
	ResourceRealm         types.String `tfsdk:"resource_realm"`
	DeletionProtection    types.Bool   `tfsdk:"deletion_protection"`
}

func (r *VMResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_instance"
}

func (r *VMResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Nubes VM Instance resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Instance identifier (UUID)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Instance display name",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Instance description",
				Optional:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current status of the instance",
				Computed:            true,
			},
			"vapp_uid": schema.StringAttribute{
				MarkdownDescription: "UUID vApp, в которой будет создаваться виртуальная машина",
				Required:            true,
			},
			"vm_name": schema.StringAttribute{
				MarkdownDescription: "Уникальное имя виртуальной машины",
				Required:            true,
			},
			"vm_cpu": schema.Int64Attribute{
				MarkdownDescription: "Количество ядер процессора. Должно быть больше 0",
				Required:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"vm_ram": schema.Int64Attribute{
				MarkdownDescription: "Объем оперативной памяти в гигабайтах. Должно быть больше 0",
				Required:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"access_port_list": schema.StringAttribute{
				MarkdownDescription: "Белый список портов для доступа к виртуальной машине извне в формате JSON. Пример: jsonencode([\"22\", \"80\"])",
				Required:            true,
				Validators: []validator.String{
					ValidJSONArray(),
				},
			},
			"image_vm": schema.StringAttribute{
				MarkdownDescription: "Образ операционной системы для развёртывания (RockyLinux_9-16G-cloudinit, Ubuntu_22-20G, Debian_13-20G)",
				Required:            true,
			},
			"user_login": schema.StringAttribute{
				MarkdownDescription: "Логин пользователя для SSH-доступа",
				Required:            true,
			},
			"user_public_key": schema.StringAttribute{
				MarkdownDescription: "Публичный SSH-ключ пользователя в формате OpenSSH",
				Required:            true,
			},
			"need_add_zabbix_template": schema.BoolAttribute{
				MarkdownDescription: "Значение истина\\ложь, определяющее будет ли добавлен хост в zabbix",
				Required:            true,
			},
			"access_ip_list": schema.StringAttribute{
				MarkdownDescription: "Белый список IP-адресов для доступа к виртуальной машине в формате JSON. Пример: jsonencode([\"1.2.3.4\", \"10.0.0.0/8\"]). Пустая строка или не указан = доступ отовсюду",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					ValidJSONArrayOrEmpty(),
				},
			},
			"vm_disk": schema.Int64Attribute{
				MarkdownDescription: "Размер дополнительного диска в гигабайтах. Должно быть больше 0",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"ip_space_name": schema.StringAttribute{
				MarkdownDescription: "Внешний IP-адрес. Если не требуется — укажите значение 'no-needed'",
				Optional:            true,
			},
			"cloud_init": schema.StringAttribute{
				MarkdownDescription: "YAML-скрипт для кастомизации системы через cloud-init",
				Optional:            true,
			},
			"resource_realm": schema.StringAttribute{
				MarkdownDescription: "Платформа ресурса (например, vcd, openstack)",
				Optional:            true,
			},
			"deletion_protection": schema.BoolAttribute{
				MarkdownDescription: "Если true, при удалении ресурса из Terraform он просто удаляется из стейта. Если false, отправляется команда suspend (карантин 14 дней).",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
		},
	}
}

func (r *VMResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*NubesClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *NubesClient, got: %T", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *VMResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VMResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get service operation ID for 'create'
	serviceId := 28 // VM service

	var instanceUid string

	svcOperationId, err := r.client.GetOperationId(ctx, serviceId, "create")
	if err != nil {
		resp.Diagnostics.AddError("Failed to get create operation ID", err.Error())
		return
	}

	// Prepare parameters
	params := r.prepareVMParams(&data)

	// Use client.CreateInstance like Postgres does
	displayName := data.DisplayName.ValueString()
	var opUid string
	instanceUid, opUid, err = r.client.CreateInstance(ctx, displayName, serviceId, svcOperationId, params)
	if err != nil {
		resp.Diagnostics.AddError("Error creating VM", err.Error())
		return
	}

	// Wait for operation success (like Postgres)
	err = r.client.WaitForOperation(ctx, opUid)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for VM creation", err.Error())
		return
	}

	// Wait for instance to be running (like Postgres does)
	err = r.client.WaitForInstanceStatus(ctx, instanceUid, "running")
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for VM running status", err.Error())
		return
	}

	data.ID = types.StringValue(instanceUid)

	// Read final state
	readData, err := r.readInstance(ctx, instanceUid)
	if err != nil {
		resp.Diagnostics.AddWarning("Unable to read instance after creation", err.Error())
	} else {
		data.Status = types.StringValue(readData.Status)
	}

	// Fix "Provider returned invalid result object" for Computed fields
	// AccessIpList is Computed + Optional. If it was not provided, we sent "", so we must set it to ""
	if data.AccessIpList.IsNull() || data.AccessIpList.IsUnknown() {
		data.AccessIpList = types.StringValue("")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// prepareVMParams собирает параметры для VM (выделено в отдельную функцию для переиспользования)
func (r *VMResource) prepareVMParams(data *VMResourceModel) []InstanceParam {
	params := []InstanceParam{}

	// Hardcoded parameter mapping (от HAR анализа)
	paramMapping := map[string]int{
		"vappUid":               407, // vApp UUID
		"vmName":                408, // VM name
		"vmCpu":                 409, // CPU count
		"vmRam":                 410, // RAM in GB
		"vmDisk":                411, // Additional disk in GB
		"ipSpaceName":           412, // IP space name
		"accessIpList":          413, // JSON array of allowed IPs (or empty string for all)
		"imageVm":               414, // OS image name
		"cloudInit":             415, // Cloud-init script
		"userLogin":             416, // SSH username
		"userPublicKey":         417, // SSH public key
		"accessPortList":        448, // JSON array of ports
		"needAddZabbixTemplate": 449, // Boolean for Zabbix monitoring
	}

	// Собираем все параметры в том же порядке что и UI (по возрастанию ID)
	// vappUid (407)
	if !data.VappUid.IsNull() && !data.VappUid.IsUnknown() {
		params = append(params, InstanceParam{SvcOperationCfsParamId: paramMapping["vappUid"], ParamValue: data.VappUid.ValueString()})
	}
	// vmName (408)
	if !data.VmName.IsNull() && !data.VmName.IsUnknown() {
		params = append(params, InstanceParam{SvcOperationCfsParamId: paramMapping["vmName"], ParamValue: data.VmName.ValueString()})
	}
	// vmCpu (409)
	if !data.VmCpu.IsNull() && !data.VmCpu.IsUnknown() {
		params = append(params, InstanceParam{SvcOperationCfsParamId: paramMapping["vmCpu"], ParamValue: fmt.Sprintf("%d", data.VmCpu.ValueInt64())})
	}
	// vmRam (410)
	if !data.VmRam.IsNull() && !data.VmRam.IsUnknown() {
		params = append(params, InstanceParam{SvcOperationCfsParamId: paramMapping["vmRam"], ParamValue: fmt.Sprintf("%d", data.VmRam.ValueInt64())})
	}
	// vmDisk (411) - ВСЕГДА отправляем (пустая строка если не задан)
	if !data.VmDisk.IsNull() && !data.VmDisk.IsUnknown() && data.VmDisk.ValueInt64() > 0 {
		params = append(params, InstanceParam{SvcOperationCfsParamId: paramMapping["vmDisk"], ParamValue: fmt.Sprintf("%d", data.VmDisk.ValueInt64())})
	} else {
		params = append(params, InstanceParam{SvcOperationCfsParamId: paramMapping["vmDisk"], ParamValue: ""})
	}
	// ipSpaceName (412)
	if !data.IpSpaceName.IsNull() && !data.IpSpaceName.IsUnknown() {
		params = append(params, InstanceParam{SvcOperationCfsParamId: paramMapping["ipSpaceName"], ParamValue: data.IpSpaceName.ValueString()})
	}
	// accessIpList (413) - ВСЕГДА отправляем (пустая строка если не задан)
	if !data.AccessIpList.IsNull() && !data.AccessIpList.IsUnknown() {
		params = append(params, InstanceParam{SvcOperationCfsParamId: paramMapping["accessIpList"], ParamValue: data.AccessIpList.ValueString()})
	} else {
		params = append(params, InstanceParam{SvcOperationCfsParamId: paramMapping["accessIpList"], ParamValue: ""})
	}
	// imageVm (414)
	if !data.ImageVm.IsNull() && !data.ImageVm.IsUnknown() {
		params = append(params, InstanceParam{SvcOperationCfsParamId: paramMapping["imageVm"], ParamValue: data.ImageVm.ValueString()})
	}
	// cloudInit (415) - ВСЕГДА отправляем (пустая строка если не задан)
	if !data.CloudInit.IsNull() && !data.CloudInit.IsUnknown() {
		params = append(params, InstanceParam{SvcOperationCfsParamId: paramMapping["cloudInit"], ParamValue: data.CloudInit.ValueString()})
	} else {
		params = append(params, InstanceParam{SvcOperationCfsParamId: paramMapping["cloudInit"], ParamValue: ""})
	}
	// userLogin (416)
	if !data.UserLogin.IsNull() && !data.UserLogin.IsUnknown() {
		params = append(params, InstanceParam{SvcOperationCfsParamId: paramMapping["userLogin"], ParamValue: data.UserLogin.ValueString()})
	}
	// userPublicKey (417)
	if !data.UserPublicKey.IsNull() && !data.UserPublicKey.IsUnknown() {
		params = append(params, InstanceParam{SvcOperationCfsParamId: paramMapping["userPublicKey"], ParamValue: data.UserPublicKey.ValueString()})
	}
	// accessPortList (448)
	if !data.AccessPortList.IsNull() && !data.AccessPortList.IsUnknown() {
		params = append(params, InstanceParam{SvcOperationCfsParamId: paramMapping["accessPortList"], ParamValue: data.AccessPortList.ValueString()})
	}
	// needAddZabbixTemplate (449) - ВСЕГДА отправляем "true" (как в UI)
	params = append(params, InstanceParam{SvcOperationCfsParamId: paramMapping["needAddZabbixTemplate"], ParamValue: "true"})

	return params
}

func (r *VMResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VMResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	instanceResp, err := r.readInstance(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read instance: %s", err))
		return
	}

	data.Status = types.StringValue(instanceResp.Status)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VMResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan VMResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state VMResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Trigger modify operation
	operationReq := CreateOperationRequest{
		InstanceUid: plan.ID.ValueString(),
		Operation:   "modify",
	}

	jsonData, err := json.Marshal(operationReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to marshal operation request: %s", err))
		return
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", r.client.ApiEndpoint+"/instanceOperations", bytes.NewBuffer(jsonData))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create operation request: %s", err))
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if r.client.ApiToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
	}

	httpResp, err := r.client.HttpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update instance: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError(
			"API Error",
			fmt.Sprintf("Modify operation failed with status %d: %s", httpResp.StatusCode, string(body)),
		)
		return
	}

	// Extract operation ID and submit parameters
	location := httpResp.Header.Get("Location")
	if location != "" {
		operationId := location[2:]

		// Wait for backend to possibly populate params
		tflog.Info(ctx, "Waiting 5 seconds for operation parameters to initialize...")
		time.Sleep(5 * time.Second)

		// Discovery parameters for modify to debug 400 error
		tflog.Info(ctx, fmt.Sprintf("Discovering parameters for operation %s", operationId))
		op, err := r.client.GetInstanceOperation(ctx, operationId)
		if err == nil {
			for _, p := range op.CfsParams {
				tflog.Info(ctx, fmt.Sprintf("Allowed Param: %s (ID: %d)", p.SvcOperationCfsParam, p.SvcOperationCfsParamId))
			}
		}

		if err := r.submitVMOperationParams(ctx, operationId, plan, state); err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to submit operation parameters: %s", err))
			return
		}
	}

	// Read updated state
	time.Sleep(5 * time.Second)
	readData, err := r.readInstance(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read instance after update: %s", err))
		return
	}

	plan.Status = types.StringValue(readData.Status)

	// Fix "Provider returned invalid result object" for Computed fields
	if plan.AccessIpList.IsNull() || plan.AccessIpList.IsUnknown() {
		plan.AccessIpList = types.StringValue("")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *VMResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VMResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Deletion Logic:
	// If DeletionProtection is TRUE (default) -> Remove from state, do not touch cloud resource.
	// If DeletionProtection is FALSE -> Send 'suspend' operation (quarantine) + remove from state.

	if !data.DeletionProtection.IsUnknown() && data.DeletionProtection.ValueBool() {
		tflog.Info(ctx, "DeletionProtection is enabled. Resource will be removed from state but kept in cloud.")
		return
	}

	tflog.Info(ctx, "DeletionProtection is disabled. Initiating 'suspend' operation (quarantine).")

	// Trigger suspend operation (instead of delete)
	operationReq := CreateOperationRequest{
		InstanceUid: data.ID.ValueString(),
		Operation:   "suspend",
	}

	jsonData, err := json.Marshal(operationReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to marshal operation request: %s", err))
		return
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", r.client.ApiEndpoint+"/instanceOperations", bytes.NewBuffer(jsonData))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create operation request: %s", err))
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if r.client.ApiToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
	}

	httpResp, err := r.client.HttpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to suspend instance: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusCreated && httpResp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError(
			"API Error",
			fmt.Sprintf("Suspend operation failed with status %d: %s", httpResp.StatusCode, string(body)),
		)
		return
	}

	// Extract operation ID and run operation
	location := httpResp.Header.Get("Location")
	if location != "" {
		operationId := location[2:]

		// For suspend, we typically don't need parameters. Just Run.
		tflog.Info(ctx, fmt.Sprintf("Running suspend operation %s", operationId))

		runReq, err := http.NewRequestWithContext(ctx, "POST",
			r.client.ApiEndpoint+"/instanceOperations/"+operationId+"/run", bytes.NewBuffer([]byte("{}")))
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create run request: %s", err))
			return
		}

		runReq.Header.Set("Content-Type", "application/json")
		if r.client.ApiToken != "" {
			runReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
		}

		runResp, err := r.client.HttpClient.Do(runReq)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to run run request: %s", err))
			return
		}
		defer runResp.Body.Close()

		if runResp.StatusCode != http.StatusOK && runResp.StatusCode != http.StatusNoContent && runResp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(runResp.Body)
			resp.Diagnostics.AddWarning("Client Warning", fmt.Sprintf("Run suspend failed with status %d: %s", runResp.StatusCode, string(body)))
		}
	}
}

func (r *VMResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *VMResource) readInstance(ctx context.Context, id string) (*InstanceResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", r.client.ApiEndpoint+"/instances/"+id, nil)
	if err != nil {
		return nil, err
	}

	if r.client.ApiToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
	}

	httpResp, err := r.client.HttpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("read request failed with status %d: %s", httpResp.StatusCode, string(body))
	}

	var response struct {
		Instance InstanceResponse `json:"instance"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		tflog.Error(ctx, fmt.Sprintf("Failed to unmarshal instance details: %s, Body: %s", err, string(body)))
		return nil, err
	}

	tflog.Info(ctx, fmt.Sprintf("Read Instance %s: Status='%s', ExplainedStatus='%s'", id, response.Instance.Status, response.Instance.Status))
	return &response.Instance, nil
}

func (r *VMResource) submitVMOperationParams(ctx context.Context, operationUid string, plan VMResourceModel, state VMResourceModel) error {
	// Step 1: Get operation details with parameters to build dynamic mapping
	op, err := r.client.GetInstanceOperation(ctx, operationUid)
	if err != nil {
		return fmt.Errorf("unable to get operation parameters: %s", err)
	}

	// Step 2: Build parameter mapping dynamically
	type ParamInfo struct {
		Id           int
		Uid          string
		CurrentValue string
	}
	paramMapping := make(map[string]ParamInfo)
	for _, p := range op.CfsParams {
		// Map param name (e.g. "vmCpu") to info (ID + InstanceParamUID)
		curVal := ""
		if p.ParamValue != nil {
			curVal = *p.ParamValue
		}
		paramMapping[p.SvcOperationCfsParam] = ParamInfo{
			Id:           p.SvcOperationCfsParamId,
			Uid:          p.InstanceOperationCfsParamUid,
			CurrentValue: curVal,
		}
	}

	tflog.Info(ctx, fmt.Sprintf("Built dynamic parameter mapping for operation %s: %v", operationUid, paramMapping))

	// Step 3: Build named parameters list
	type NamedParam struct {
		Name  string
		Value string // Must be strictly string
	}

	namedParams := []NamedParam{}

	// Add ALL potential parameters. The mapping check will filter out those not applicable to the current operation.

	// vmName
	/*
		if !plan.VmName.IsNull() && !plan.VmName.IsUnknown() {
			// Only send if changed
			if !plan.VmName.Equal(state.VmName) {
				namedParams = append(namedParams, NamedParam{"vmName", plan.VmName.ValueString()})
			}
		}
	*/

	// vmCpu
	if !plan.VmCpu.IsNull() && !plan.VmCpu.IsUnknown() {
		if !plan.VmCpu.Equal(state.VmCpu) {
			namedParams = append(namedParams, NamedParam{"vmCpu", fmt.Sprintf("%d", plan.VmCpu.ValueInt64())})
		}
	}

	// vmRam
	/*
		if !plan.VmRam.IsNull() && !plan.VmRam.IsUnknown() {
			if !plan.VmRam.Equal(state.VmRam) {
				namedParams = append(namedParams, NamedParam{"vmRam", fmt.Sprintf("%d", plan.VmRam.ValueInt64())})
			}
		}

		// vmDisk
		if !plan.VmDisk.IsNull() && !plan.VmDisk.IsUnknown() {
			val := plan.VmDisk.ValueInt64()
			if val > 0 && !plan.VmDisk.Equal(state.VmDisk) {
				namedParams = append(namedParams, NamedParam{"vmDisk", fmt.Sprintf("%d", val)})
			}
		}

		// ipSpaceName
		if !plan.IpSpaceName.IsNull() && !plan.IpSpaceName.IsUnknown() {
			if !plan.IpSpaceName.Equal(state.IpSpaceName) {
				namedParams = append(namedParams, NamedParam{"ipSpaceName", plan.IpSpaceName.ValueString()})
			}
		}
	*/

	// accessIpList
	/*
		if !plan.AccessIpList.IsNull() && !plan.AccessIpList.IsUnknown() {
			val := plan.AccessIpList.ValueString()

			// If ipSpaceName is "no-needed", SKIP sending this parameter
			isNoNeeded := !plan.IpSpaceName.IsNull() && plan.IpSpaceName.ValueString() == "no-needed"
			if !isNoNeeded {
				if !plan.AccessIpList.Equal(state.AccessIpList) {
					namedParams = append(namedParams, NamedParam{"accessIpList", val})
				}
			}
		}

		// accessPortList
		if !plan.AccessPortList.IsNull() && !plan.AccessPortList.IsUnknown() {
			val := plan.AccessPortList.ValueString()

			// If ipSpaceName is "no-needed", SKIP sending this parameter
			isNoNeeded := !plan.IpSpaceName.IsNull() && plan.IpSpaceName.ValueString() == "no-needed"
			if !isNoNeeded {
				if !plan.AccessPortList.Equal(state.AccessPortList) {
					namedParams = append(namedParams, NamedParam{"accessPortList", val})
				}
			}
		}
	*/

	// needAddZabbixTemplate
	// Skip sending to avoid duplicates unless we really know it changed.
	// Since we don't track it in state (usually), we omit it.

	// Create-only params (will be filtered out for Modify automatically but good to skip anyway)
	// Generally they shouldn't change in Modify.
	// imageVm, cloudInit, userLogin, userPublicKey -> usually ForceNew?
	// If they are not ForceNew, check change.
	// Assuming ForceNew behavior for image/user details, but if they are Updateable:
	/*
		if !plan.ImageVm.IsNull() && !plan.ImageVm.IsUnknown() && !plan.ImageVm.Equal(state.ImageVm) {
			namedParams = append(namedParams, NamedParam{"imageVm", plan.ImageVm.ValueString()})
		}
		if !plan.CloudInit.IsNull() && !plan.CloudInit.IsUnknown() && !plan.CloudInit.Equal(state.CloudInit) {
			namedParams = append(namedParams, NamedParam{"cloudInit", plan.CloudInit.ValueString()})
		}
		if !plan.UserLogin.IsNull() && !plan.UserLogin.IsUnknown() && !plan.UserLogin.Equal(state.UserLogin) {
			namedParams = append(namedParams, NamedParam{"userLogin", plan.UserLogin.ValueString()})
		}
		if !plan.UserPublicKey.IsNull() && !plan.UserPublicKey.IsUnknown() && !plan.UserPublicKey.Equal(state.UserPublicKey) {
			namedParams = append(namedParams, NamedParam{"userPublicKey", plan.UserPublicKey.ValueString()})
		}
	*/

	// Step 4: Submit parameters
	if len(namedParams) == 0 {
		tflog.Info(ctx, "No parameters to submit. Skipping parameter submission.")
		return nil
	}

	// Submit parameters
	for i := len(namedParams) - 1; i >= 0; i-- {
		np := namedParams[i]
		info, ok := paramMapping[np.Name]
		if !ok {
			// Parameter not allowed in this operation
			continue
		}

		// Check if value is unchanged
		if np.Value == info.CurrentValue {
			tflog.Info(ctx, fmt.Sprintf("Skipping parameter %s (ID %d) as value '%s' is unchanged", np.Name, info.Id, np.Value))
			continue
		}

		tflog.Info(ctx, fmt.Sprintf("Submitting parameter: %s (ID %d) = %v [ParamUID: %s]", np.Name, info.Id, np.Value, info.Uid))

		var method string
		var payload map[string]interface{}

		if info.Uid != "" {
			// Update existing parameter -> PUT
			method = "PUT"
			payload = map[string]interface{}{
				"instanceOperationCfsParamUid": info.Uid,
				"svcOperationCfsParamId":       info.Id,
				"instanceOperationUid":         operationUid,
				"paramValue":                   np.Value,
			}
		} else {
			// Create new parameter -> POST
			method = "POST"
			payload = map[string]interface{}{
				"instanceOperationUid":   operationUid,
				"svcOperationCfsParamId": info.Id,
				"paramValue":             np.Value,
			}
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("unable to marshal param request: %s", err)
		}

		tflog.Info(ctx, fmt.Sprintf("Param Request Payload (%s): %s", method, string(jsonData)))

		url := r.client.ApiEndpoint + "/instanceOperationCfsParams"
		httpReq, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("unable to create param request: %s", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		if r.client.ApiToken != "" {
			httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
		}

		httpResp, err := r.client.HttpClient.Do(httpReq)
		if err != nil {
			return fmt.Errorf("unable to submit parameter: %s", err)
		}

		respBody, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()

		if httpResp.StatusCode != http.StatusCreated &&
			httpResp.StatusCode != http.StatusOK &&
			httpResp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("submit parameter %s (id %d) failed with status %d: %s",
				np.Name, info.Id, httpResp.StatusCode, string(respBody))
		}
	}

	// Step 5: Run the operation
	runReq, err := http.NewRequestWithContext(ctx, "POST",
		r.client.ApiEndpoint+"/instanceOperations/"+operationUid+"/run", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return fmt.Errorf("unable to create run request: %s", err)
	}

	runReq.Header.Set("Content-Type", "application/json")
	if r.client.ApiToken != "" {
		runReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
	}

	runResp, err := r.client.HttpClient.Do(runReq)
	if err != nil {
		return fmt.Errorf("unable to run operation: %s", err)
	}
	defer runResp.Body.Close()

	if runResp.StatusCode != http.StatusOK &&
		runResp.StatusCode != http.StatusNoContent &&
		runResp.StatusCode != http.StatusCreated {
		runBody, _ := io.ReadAll(runResp.Body)
		return fmt.Errorf("run operation failed with status %d: %s",
			runResp.StatusCode, string(runBody))
	}

	// Step 6: Wait for operation completion
	tflog.Info(ctx, fmt.Sprintf("Waiting for operation %s to complete...", operationUid))
	if err := r.client.WaitForOperation(ctx, operationUid); err != nil {
		return fmt.Errorf("wait for operation %s failed: %s", operationUid, err)
	}

	return nil
}

type VMOperationStage struct {
	Name         string `json:"stage"`
	IsSuccessful bool   `json:"isSuccessful"`
}

type VMOperation struct {
	IsInProgress bool               `json:"isInProgress"`
	IsPending    bool               `json:"isPending"`
	IsSuccessful *bool              `json:"isSuccessful"`
	DtFinish     *string            `json:"dtFinish"`
	ErrorLog     *string            `json:"errorLog"`
	Stages       []VMOperationStage `json:"stages"`
}

type GetVMOperationResponse struct {
	InstanceOperation VMOperation `json:"instanceOperation"`
}

func (r *VMResource) waitForVMOperationAndInstanceStatus(ctx context.Context, operationId string, instanceId string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	// Phase 1: Wait for operation completion
	opTicker := time.NewTicker(5 * time.Second)
	defer opTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled")
		case <-opTicker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for operation %s", operationId)
			}

			// Check operation status
			httpReq, err := http.NewRequestWithContext(ctx, "GET", r.client.ApiEndpoint+"/instanceOperations/"+operationId, nil)
			if err != nil {
				return fmt.Errorf("failed to create request: %s", err)
			}

			if r.client.ApiToken != "" {
				httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
			}

			httpResp, err := r.client.HttpClient.Do(httpReq)
			if err != nil {
				return fmt.Errorf("failed to check operation status: %s", err)
			}

			body, _ := io.ReadAll(httpResp.Body)
			httpResp.Body.Close()

			if httpResp.StatusCode != http.StatusOK {
				return fmt.Errorf("check operation failed: %d", httpResp.StatusCode)
			}

			var opResp GetVMOperationResponse
			if err := json.Unmarshal(body, &opResp); err != nil {
				return fmt.Errorf("failed to parse operation response: %s", err)
			}

			op := opResp.InstanceOperation

			// Detect Failure
			if op.IsSuccessful != nil && !*op.IsSuccessful {
				// Scan stages for detail
				failedStage := "unknown"
				for _, stage := range op.Stages {
					if !stage.IsSuccessful {
						failedStage = stage.Name
					}
				}
				return fmt.Errorf("operation failed at stage '%s' (isSuccessful=false)", failedStage)
			}

			// Detect Success (как в client_impl.go WaitForOperation)
			if !op.IsInProgress && !op.IsPending && op.DtFinish != nil && *op.DtFinish != "" {
				if op.IsSuccessful != nil && *op.IsSuccessful {
					// Operation finished successfully
					// Move to Phase 2
					goto Phase2
				}
				// Если не successful но finished - это ошибка (уже обработана выше)
			}

			// Still running, continue waiting...
		}
	}

Phase2:
	// Phase 2: Wait for Instance to be Running
	instTicker := time.NewTicker(10 * time.Second)
	defer instTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled")
		case <-instTicker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for instance running state")
			}

			instance, err := r.readInstance(ctx, instanceId)
			if err != nil {
				return fmt.Errorf("failed to check instance: %s", err)
			}

			if instance.Status == "running" {
				return nil
			}

			if instance.Status == "error" || instance.Status == "failed" {
				return fmt.Errorf("instance entered error state: %s", instance.Status)
			}
		}
	}
}
