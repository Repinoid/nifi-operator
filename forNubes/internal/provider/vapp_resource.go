package provider

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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &VAppResource{}
var _ resource.ResourceWithImportState = &VAppResource{}

func NewVAppResource() resource.Resource {
	return &VAppResource{}
}

type VAppResource struct {
	client *NubesClient
}

type VAppResourceModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
	Description types.String `tfsdk:"description"`
	EdgeUID     types.String `tfsdk:"edge_uid"`
	VAppName    types.String `tfsdk:"vapp_name"`
	VdcUID      types.String `tfsdk:"vdc_uid"`
	Status      types.String `tfsdk:"status"`
}

func (r *VAppResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vapp"
}

func (r *VAppResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Nubes vApp (Virtual Application Catalog) resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "vApp identifier (UUID)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "vApp display name",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "vApp description",
				Optional:            true,
			},
			"edge_uid": schema.StringAttribute{
				MarkdownDescription: "Edge Gateway UUID (parameter ID 190)",
				Required:            true,
			},
			"vapp_name": schema.StringAttribute{
				MarkdownDescription: "vApp name in Cloud Director (parameter ID 191)",
				Required:            true,
			},
			"vdc_uid": schema.StringAttribute{
				MarkdownDescription: "VDC UUID (parameter ID 623)",
				Required:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current status of the vApp",
				Computed:            true,
			},
		},
	}
}

func (r *VAppResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *VAppResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VAppResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Step 1: Create instance
	createReq := CreateInstanceRequest{
		ServiceId:   26, // VApp service ID
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

	// Step 4: Wait for operation completion and instance running status
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

func (r *VAppResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VAppResourceModel

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

func (r *VAppResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data VAppResourceModel

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

func (r *VAppResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VAppResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	operationReq := CreateOperationRequest{
		InstanceUid: data.ID.ValueString(),
		Operation:   "delete",
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
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete instance: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusCreated && httpResp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError(
			"API Error",
			fmt.Sprintf("Delete operation failed with status %d: %s", httpResp.StatusCode, string(body)),
		)
		return
	}

	location := httpResp.Header.Get("Location")
	if location != "" {
		operationId := location[2:]
		if err := r.submitOperationParams(ctx, operationId, data); err != nil {
			resp.Diagnostics.AddWarning("Client Warning", fmt.Sprintf("Unable to submit operation parameters: %s", err))
		}
	}
}

func (r *VAppResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *VAppResource) readInstance(ctx context.Context, id string) (*InstanceResponse, error) {
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

func (r *VAppResource) submitOperationParams(ctx context.Context, operationUid string, data VAppResourceModel) error {
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
		
		// Map vApp parameters by ID
		switch param.SvcOperationCfsParamId {
		case 190: // edgeUid
			if !data.EdgeUID.IsNull() && !data.EdgeUID.IsUnknown() {
				valToSend = data.EdgeUID.ValueString()
			}
		case 191: // vappName
			if !data.VAppName.IsNull() && !data.VAppName.IsUnknown() {
				valToSend = data.VAppName.ValueString()
			}
		case 623: // vdcUid
			if !data.VdcUID.IsNull() && !data.VdcUID.IsUnknown() {
				valToSend = data.VdcUID.ValueString()
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

func (r *VAppResource) waitForOperationAndInstanceStatus(ctx context.Context, operationId string, instanceId string, timeout time.Duration) error {
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

if instance.Status == "running" {
return nil
}

if instance.Status == "error" || instance.Status == "failed" {
return fmt.Errorf("instance entered error state: %s", instance.Status)
}
}
}
}
