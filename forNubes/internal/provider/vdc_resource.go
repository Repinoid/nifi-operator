package provider

// [НЕ ИЗМЕНЯТЬ !!!!]
// Данный ресурс реализует паттерн "Nubes Flow" для Virtual Data Center (VDC).
// ВАЖНО: VDC является фундаментальным ресурсом инфраструктуры.
//
// ЛОГИКА УДАЛЕНИЯ (Delete):
// Автоматическое удаление через API ОТКЛЮЧЕНО для защиты от случайной потери данных.
// При вызове 'terraform destroy' ресурс просто УДАЛЯЕТСЯ ИЗ СТЕЙТА Terraform, но остается в облаке.
// Для реального удаления юзер должен вручную перевести инстанс в 'suspend' через UI и дождаться удаления (14 дней).
//
// ЛОГИКА ИЗМЕНЕНИЯ (Modify):
// Если ресурс находится в стейте, выполнение 'terraform apply' вызовет метод Update,
// который запустит операцию 'modify' для обновления параметров (квот и т.д.).
// ВНИМАНИЕ: Если вы уже выполнили 'destroy' (удалили из стейта), но ресурс остался в облаке,
// то для его изменения через Terraform вам придется сначала выполнить 'terraform import'.
//
// TODO: Протестировать VDC 'modify' позже.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
)

var _ resource.Resource = &VDCResource{}
var _ resource.ResourceWithImportState = &VDCResource{}

func NewVDCResource() resource.Resource {
	return &VDCResource{}
}

type VDCResource struct {
	client *NubesClient
}

type VDCResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	DisplayName        types.String `tfsdk:"display_name"`
	Description        types.String `tfsdk:"description"`
	OrganizationUID    types.String `tfsdk:"organization_uid"`
	ProviderVDC        types.String `tfsdk:"provider_vdc"`
	StorageProfiles    types.String `tfsdk:"storage_profiles"`
	NetworkPool        types.String `tfsdk:"network_pool"`
	CpuAllocationPct   types.Int64  `tfsdk:"cpu_allocation_pct"`
	RamAllocationPct   types.Int64  `tfsdk:"ram_allocation_pct"`
	CpuQuota           types.Int64  `tfsdk:"cpu_quota"`
	RamQuota           types.Int64  `tfsdk:"ram_quota"`
	DeletionProtection types.Bool   `tfsdk:"deletion_protection"`
	Status             types.String `tfsdk:"status"`
	Timeouts           timeouts.Value `tfsdk:"timeouts"`
}

func (r *VDCResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vdc"
}

func (r *VDCResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Nubes VDC (Virtual Data Center) resource",
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "VDC identifier (UUID)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "VDC display name",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "VDC description",
				Optional:            true,
			},
			"organization_uid": schema.StringAttribute{
				MarkdownDescription: "Organization UUID (parameter ID 30)",
				Required:            true,
			},
			"provider_vdc": schema.StringAttribute{
				MarkdownDescription: "Provider VDC name in Cloud Director (parameter ID 335)",
				Required:            true,
			},
			"storage_profiles": schema.StringAttribute{
				MarkdownDescription: "Storage profiles JSON array, e.g. [{\"name\":\"vsan\",\"size\":\"100\"}] (parameter ID 361)",
				Required:            true,
			},
			"network_pool": schema.StringAttribute{
				MarkdownDescription: "Network Pool name in Cloud Director (parameter ID 366)",
				Required:            true,
			},
			"cpu_allocation_pct": schema.Int64Attribute{
				MarkdownDescription: "CPU allocation percentage (parameter ID 397)",
				Required:            true,
			},
			"ram_allocation_pct": schema.Int64Attribute{
				MarkdownDescription: "RAM allocation percentage (parameter ID 398)",
				Required:            true,
			},
			"cpu_quota": schema.Int64Attribute{
				MarkdownDescription: "CPU quota (parameter ID 557)",
				Optional:            true,
			},
			"ram_quota": schema.Int64Attribute{
				MarkdownDescription: "RAM quota (parameter ID 558)",
				Optional:            true,
			},
			"deletion_protection": schema.BoolAttribute{
				MarkdownDescription: "If true, the resource will only be removed from Terraform state upon destroy, but will remain in the cloud. If false, destroy will trigger 'suspend' in Nubes.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current status of the VDC",
				Computed:            true,
			},
		},
	}
}

func (r *VDCResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *VDCResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// [ЛОГИКА VDC FLOW]
	// Ресурс VDC создается по стандартному 7-шаговому алгоритму Nubes.
	// Особое внимание уделяется параметрам квот (CPU/RAM) и сетевым пулам.
	
	var data VDCResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Apply timeout
	createTimeout, diags := data.Timeouts.Create(ctx, 3*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// Step 1: Create instance
	createReq := CreateInstanceRequest{
		ServiceId:   21, // VDC service ID
		DisplayName: data.DisplayName.ValueString(),
		Descr:       data.Description.ValueString(),
	}

	jsonData, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to marshal request: %s", err))
		return
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", r.client.ApiEndpoint+"/instances", bytes.NewBuffer(jsonData))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if r.client.ApiToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
	}

	httpResp, err := r.client.HttpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create instance: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError(
			"API Error",
			fmt.Sprintf("Create instance failed with status %d: %s", httpResp.StatusCode, string(body)),
		)
		return
	}

	location := httpResp.Header.Get("Location")
	if location == "" {
		resp.Diagnostics.AddError("API Error", "No Location header in response")
		return
	}
	instanceId := location[2:]

	data.ID = types.StringValue(instanceId)

	// Step 2: Create operation
	// Deck API требует явного создания операции 'create' для инстанса.
	// Это переводит инстанс в состояние "wizard", где можно настраивать параметры.
	operationReq := CreateOperationRequest{
		InstanceUid: instanceId,
		Operation:   "create",
	}

	jsonData, err = json.Marshal(operationReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to marshal operation request: %s", err))
		return
	}

	httpReq, err = http.NewRequestWithContext(ctx, "POST", r.client.ApiEndpoint+"/instanceOperations", bytes.NewBuffer(jsonData))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create operation request: %s", err))
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if r.client.ApiToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
	}

	httpResp, err = r.client.HttpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create operation: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError(
			"API Error",
			fmt.Sprintf("Create operation failed with status %d: %s", httpResp.StatusCode, string(body)),
		)
		return
	}

	location = httpResp.Header.Get("Location")
	if location == "" {
		resp.Diagnostics.AddError("API Error", "No Location header in operation response")
		return
	}
	operationId := location[2:]

	// Step 3: Submit operation parameters and run
	if err := r.submitOperationParams(ctx, operationId, data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to submit operation parameters: %s", err))
		return
	}

	// Step 4: Wait for operation to complete, then check instance status
	if err := r.waitForOperationAndInstanceStatus(ctx, operationId, instanceId, 10*time.Minute); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Instance failed to reach running status: %s", err))
		return
	}

	readData, err := r.readInstance(ctx, instanceId)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read instance after creation: %s", err))
		return
	}

	data.Status = types.StringValue(readData.Status)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VDCResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VDCResourceModel

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

func (r *VDCResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// [НЕ ИЗМЕНЯТЬ !!!!]
	// VDC поддерживает изменение параметров через операцию 'modify'.
	// Это позволяет обновлять квоты CPU, RAM и другие параметры без пересоздания ресурса.

	var data VDCResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	operationReq := CreateOperationRequest{
		InstanceUid: data.ID.ValueString(),
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

	location := httpResp.Header.Get("Location")
	if location != "" {
		operationId := location[2:]
		if err := r.submitOperationParams(ctx, operationId, data); err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to submit operation parameters: %s", err))
			return
		}
	}

	time.Sleep(5 * time.Second)
	readData, err := r.readInstance(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read instance after update: %s", err))
		return
	}

	data.Status = types.StringValue(readData.Status)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VDCResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// ЛОГИКА УДАЛЕНИЯ VDC:
	// В зависимости от флага deletion_protection:
	// 1. Если true (по умолчанию): Только удаляем из стейта. VDC остается работать.
	// 2. Если false: Вызываем операцию 'suspend' через API. 
	//    Доступа к VDC больше не будет, данные сохраняются 14 дней, затем авто-удаление.

	var data VDCResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.DeletionProtection.ValueBool() {
		tflog.Warn(ctx, "Deletion Protection is ENABLED. VDC will remain active in Nubes Cloud. Manual cleanup required.")
		return
	}

	tflog.Info(ctx, "Deletion Protection is DISABLED. Triggering 'suspend' for VDC...")

	instanceId := data.ID.ValueString()
	
	// Выполняем операцию suspend
	err := r.triggerOperation(ctx, instanceId, "suspend")
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to suspend VDC: %s", err))
		return
	}

	tflog.Info(ctx, "VDC suspended successfully. It will be permanently deleted from the cloud in 14 days.")
}

func (r *VDCResource) triggerOperation(ctx context.Context, instanceId string, operationName string) error {
	opReq := InstanceOperationRequest{
		Action: operationName,
		Params: struct{}{},
	}

	jsonData, err := json.Marshal(opReq)
	if err != nil {
		return fmt.Errorf("unable to marshal request: %s", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", r.client.ApiEndpoint+"/instances/"+instanceId+"/run", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("unable to create request: %s", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if r.client.ApiToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
	}

	httpResp, err := r.client.HttpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("unable to trigger operation: %s", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("operation failed with status %d: %s", httpResp.StatusCode, string(body))
	}

	// Ждем пока статус изменится на целевой (например, suspended)
	targetStatus := "running"
	if operationName == "suspend" {
		targetStatus = "suspended"
	}

	return r.client.WaitForInstanceStatus(ctx, instanceId, targetStatus)
}

func (r *VDCResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *VDCResource) readInstance(ctx context.Context, id string) (*InstanceResponse, error) {
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

	var instanceResp InstanceResponse
	if err := json.Unmarshal(body, &instanceResp); err != nil {
		return nil, err
	}

	return &instanceResp, nil
}

func (r *VDCResource) submitOperationParams(ctx context.Context, operationUid string, data VDCResourceModel) error {
	// [ЛОГИКА СИНХРОНИЗАЦИИ ПАРАМЕТРОВ]
	// 1. Получаем список параметров операции (GET /instanceOperations/{uid}?fields=cfsParams).
	// 2. Для каждого параметра из списка находим значение в Terraform или используем дефолт.
	// 3. Отправляем значение обратно в API (POST /instanceOperationCfsParams).
	// 4. После синхронизации всех параметров вызываем RUN.

	// Get operation details
	httpReq, err := http.NewRequestWithContext(ctx, "GET",
		r.client.ApiEndpoint+"/instanceOperations/"+operationUid+"?fields=cfsParams", nil)
	if err != nil {
		return fmt.Errorf("unable to create request: %s", err)
	}

	if r.client.ApiToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
	}

	httpResp, err := r.client.HttpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("unable to get operation: %s", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("unable to read response: %s", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("get operation failed with status %d: %s", httpResp.StatusCode, string(body))
	}

	var getResp GetOperationResponse
	if err := json.Unmarshal(body, &getResp); err != nil {
		return fmt.Errorf("unable to unmarshal response: %s", err)
	}
	operationResp := getResp.InstanceOperation

	// Submit each parameter
	for _, param := range operationResp.CfsParams {
		valToSend := ""
		
		// Map VDC parameters by ID
		switch param.SvcOperationCfsParamId {
		case 30: // organizationUid
			if !data.OrganizationUID.IsNull() && !data.OrganizationUID.IsUnknown() {
				valToSend = data.OrganizationUID.ValueString()
			}
		case 335: // providerVdc
			if !data.ProviderVDC.IsNull() && !data.ProviderVDC.IsUnknown() {
				valToSend = data.ProviderVDC.ValueString()
			}
		case 361: // storageProfiles (JSON)
			if !data.StorageProfiles.IsNull() && !data.StorageProfiles.IsUnknown() {
				valToSend = data.StorageProfiles.ValueString()
			}
		case 366: // networkPool
			if !data.NetworkPool.IsNull() && !data.NetworkPool.IsUnknown() {
				valToSend = data.NetworkPool.ValueString()
			}
		case 397: // cpuAllocationPct
			if !data.CpuAllocationPct.IsNull() && !data.CpuAllocationPct.IsUnknown() {
				valToSend = fmt.Sprintf("%d", data.CpuAllocationPct.ValueInt64())
			}
		case 398: // ramAllocationPct
			if !data.RamAllocationPct.IsNull() && !data.RamAllocationPct.IsUnknown() {
				valToSend = fmt.Sprintf("%d", data.RamAllocationPct.ValueInt64())
			}
		case 557: // cpuQuota
			if !data.CpuQuota.IsNull() && !data.CpuQuota.IsUnknown() {
				valToSend = fmt.Sprintf("%d", data.CpuQuota.ValueInt64())
			}
		case 558: // ramQuota
			if !data.RamQuota.IsNull() && !data.RamQuota.IsUnknown() {
				valToSend = fmt.Sprintf("%d", data.RamQuota.ValueInt64())
			}
		default:
			// Use existing or default value for unknown parameters
			if param.ParamValue != nil {
				valToSend = *param.ParamValue
			} else if param.DefaultValue != nil {
				valToSend = *param.DefaultValue
			}
		}

		// Fix specific data type formatting
		if valToSend == "" || valToSend == "\"\"" {
			if param.DataType == "map" || param.DataType == "json" {
				valToSend = "{}"
			} else if param.DataType == "array" || param.DataType == "list" {
				valToSend = "[]"
			}
		}

		paramReq := CreateCfsParamRequest{
			InstanceOperationUid:   operationUid,
			SvcOperationCfsParamId: param.SvcOperationCfsParamId,
			ParamValue:             valToSend,
		}

		jsonData, err := json.Marshal(paramReq)
		if err != nil {
			return fmt.Errorf("unable to marshal param request: %s", err)
		}

		httpReq, err = http.NewRequestWithContext(ctx, "POST",
			r.client.ApiEndpoint+"/instanceOperationCfsParams", bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("unable to create param request: %s", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		if r.client.ApiToken != "" {
			httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
		}

		httpResp, err = r.client.HttpClient.Do(httpReq)
		if err != nil {
			return fmt.Errorf("unable to submit parameter: %s", err)
		}

		respBody, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()

		if httpResp.StatusCode != http.StatusCreated &&
			httpResp.StatusCode != http.StatusOK &&
			httpResp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("submit parameter id %d failed with status %d: %s",
				param.SvcOperationCfsParamId, httpResp.StatusCode, string(respBody))
		}
	}

	// Run the operation
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

	return nil
}

func (r *VDCResource) waitForOperationAndInstanceStatus(ctx context.Context, operationId string, instanceId string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	log.Printf("[DEBUG] Starting two-stage polling: operationId=%s, instanceId=%s, timeout=%v", operationId, instanceId, timeout)

	// Шаг 1: Ждём завершения операции
operationLoop:
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled")
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for operation to complete")
			}

			log.Printf("[DEBUG] Polling operation status: operationId=%s", operationId)

			// Проверяем статус операции
			httpReq, err := http.NewRequestWithContext(ctx, "GET",
				r.client.ApiEndpoint+"/instanceOperations/"+operationId, nil)
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
				return fmt.Errorf("get operation failed with status %d", httpResp.StatusCode)
			}

			var opResp struct {
				InstanceOperation struct {
					IsInProgress bool `json:"isInProgress"`
					IsPending    bool `json:"isPending"`
				} `json:"instanceOperation"`
			}

			if err := json.Unmarshal(body, &opResp); err != nil {
				return fmt.Errorf("failed to parse operation response: %s", err)
			}

			log.Printf("[DEBUG] Operation status: isInProgress=%v, isPending=%v", opResp.InstanceOperation.IsInProgress, opResp.InstanceOperation.IsPending)

			// Операция завершена когда isInProgress=false И isPending=false
			if !opResp.InstanceOperation.IsInProgress && !opResp.InstanceOperation.IsPending {
				log.Printf("[DEBUG] Operation completed, moving to instance status check")
				break operationLoop
			}
		}
	}

	// Шаг 2: Проверяем статус instance
	ticker2 := time.NewTicker(5 * time.Second)
	defer ticker2.Stop()

	log.Printf("[DEBUG] Starting instance status polling")

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled")
		case <-ticker2.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for instance to become running")
			}

			log.Printf("[DEBUG] Polling instance status: instanceId=%s", instanceId)

			instance, err := r.readInstance(ctx, instanceId)
			if err != nil {
				return fmt.Errorf("failed to check instance status: %s", err)
			}

			log.Printf("[DEBUG] Instance status: %s", instance.Status)

			if instance.Status == "running" {
				log.Printf("[DEBUG] Instance is running, success!")
				return nil
			}

			if instance.Status == "error" || instance.Status == "failed" {
				return fmt.Errorf("instance entered error state: %s", instance.Status)
			}
		}
	}
}

