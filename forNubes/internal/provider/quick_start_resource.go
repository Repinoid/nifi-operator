package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &QuickStartResource{}
var _ resource.ResourceWithImportState = &QuickStartResource{}

func NewQuickStartResource() resource.Resource {
	return &QuickStartResource{}
}

type QuickStartResource struct {
	client *NubesClient
}

type QuickStartResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	DisplayName           types.String `tfsdk:"display_name"`
	Description           types.String `tfsdk:"description"`
	Status                types.String `tfsdk:"status"`
	
	// Параметры из HAR файла
	OrgDomain             types.String `tfsdk:"org_domain"`              // 332: ngcloud.ru
	VAppName              types.String `tfsdk:"vapp_name"`               // 334: vappa
	ProviderVDC           types.String `tfsdk:"provider_vdc"`            // 358: sandbox-v1cl1-pvdc
	EdgeCount             types.Int64  `tfsdk:"edge_count"`              // 359: 1
	ExternalNetwork       types.String `tfsdk:"external_network"`        // 360: internet-ipv4-v1
	StorageProfiles       types.String `tfsdk:"storage_profiles"`        // 399: [{"name": "", "size": 1024}]
	NetworkPool           types.String `tfsdk:"network_pool"`            // 400: nsxt-sandbox-geneve-np
	CPUAllocationPercent  types.Int64  `tfsdk:"cpu_allocation_percent"`  // 401: 20
	RAMAllocationPercent  types.Int64  `tfsdk:"ram_allocation_percent"`  // 402: 20
	ServiceEngineGroup    types.String `tfsdk:"service_engine_group"`    // 403: SEGROUP-SANDBOX-CL1-SHARED-01
	ThinProvisioning      types.Bool   `tfsdk:"thin_provisioning"`       // 404: true
	FastProvisioning      types.Bool   `tfsdk:"fast_provisioning"`       // 405: false
	VDCNetworkQuota       types.Int64  `tfsdk:"vdc_network_quota"`       // 566: 1
	CPUQuotaMhz           types.Int64  `tfsdk:"cpu_quota_mhz"`           // 567: 10
	RAMQuotaGb            types.Int64  `tfsdk:"ram_quota_gb"`            // 568: 1
}

func (r *QuickStartResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quick_start"
}

func (r *QuickStartResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Nubes Quick Start - создаёт полное окружение (Org + VDC + Edge + vApp)",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Quick Start identifier (UUID)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Display name",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description",
				Optional:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current status",
				Computed:            true,
			},
			"org_domain": schema.StringAttribute{
				MarkdownDescription: "Organization domain (например: ngcloud.ru)",
				Required:            true,
			},
			"vapp_name": schema.StringAttribute{
				MarkdownDescription: "vApp name",
				Required:            true,
			},
			"provider_vdc": schema.StringAttribute{
				MarkdownDescription: "Provider VDC name",
				Required:            true,
			},
			"edge_count": schema.Int64Attribute{
				MarkdownDescription: "Number of Edge gateways",
				Required:            true,
			},
			"external_network": schema.StringAttribute{
				MarkdownDescription: "External network name",
				Required:            true,
			},
			"storage_profiles": schema.StringAttribute{
				MarkdownDescription: "Storage profiles JSON",
				Required:            true,
			},
			"network_pool": schema.StringAttribute{
				MarkdownDescription: "Network pool name",
				Required:            true,
			},
			"cpu_allocation_percent": schema.Int64Attribute{
				MarkdownDescription: "CPU allocation percent",
				Required:            true,
			},
			"ram_allocation_percent": schema.Int64Attribute{
				MarkdownDescription: "RAM allocation percent",
				Required:            true,
			},
			"service_engine_group": schema.StringAttribute{
				MarkdownDescription: "Service Engine Group",
				Required:            true,
			},
			"thin_provisioning": schema.BoolAttribute{
				MarkdownDescription: "Enable thin provisioning",
				Required:            true,
			},
			"fast_provisioning": schema.BoolAttribute{
				MarkdownDescription: "Enable fast provisioning",
				Required:            true,
			},
			"vdc_network_quota": schema.Int64Attribute{
				MarkdownDescription: "VDC network quota",
				Required:            true,
			},
			"cpu_quota_mhz": schema.Int64Attribute{
				MarkdownDescription: "CPU quota in MHz",
				Required:            true,
			},
			"ram_quota_gb": schema.Int64Attribute{
				MarkdownDescription: "RAM quota in GB",
				Required:            true,
			},
		},
	}
}

func (r *QuickStartResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *QuickStartResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data QuickStartResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Step 1: Create instance
	instanceReq := CreateInstanceRequest{
		ServiceId:   113, // Quick Start service ID
		DisplayName: data.DisplayName.ValueString(),
		Descr:       data.Description.ValueString(),
	}

	jsonData, err := json.Marshal(instanceReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to marshal instance request: %s", err))
		return
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", r.client.ApiEndpoint+"/instances", bytes.NewBuffer(jsonData))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create instance request: %s", err))
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
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Create instance failed with status %d: %s", httpResp.StatusCode, string(body)))
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
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Create operation failed with status %d: %s", httpResp.StatusCode, string(body)))
		return
	}

	location = httpResp.Header.Get("Location")
	if location == "" {
		resp.Diagnostics.AddError("API Error", "No Location header in operation response")
		return
	}
	operationId := location[2:]

	// Step 3: Submit all parameters
	params := map[int]string{
		332: data.OrgDomain.ValueString(),
		334: data.VAppName.ValueString(),
		358: data.ProviderVDC.ValueString(),
		359: fmt.Sprintf("%d", data.EdgeCount.ValueInt64()),
		360: data.ExternalNetwork.ValueString(),
		399: data.StorageProfiles.ValueString(),
		400: data.NetworkPool.ValueString(),
		401: fmt.Sprintf("%d", data.CPUAllocationPercent.ValueInt64()),
		402: fmt.Sprintf("%d", data.RAMAllocationPercent.ValueInt64()),
		403: data.ServiceEngineGroup.ValueString(),
		404: fmt.Sprintf("%t", data.ThinProvisioning.ValueBool()),
		405: fmt.Sprintf("%t", data.FastProvisioning.ValueBool()),
		566: fmt.Sprintf("%d", data.VDCNetworkQuota.ValueInt64()),
		567: fmt.Sprintf("%d", data.CPUQuotaMhz.ValueInt64()),
		568: fmt.Sprintf("%d", data.RAMQuotaGb.ValueInt64()),
	}

	for paramId, paramValue := range params {
		paramReq := CreateCfsParamRequest{
			InstanceOperationUid:   operationId,
			SvcOperationCfsParamId: paramId,
			ParamValue:             paramValue,
		}

		jsonData, err = json.Marshal(paramReq)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to marshal param request: %s", err))
			return
		}

		httpReq, err = http.NewRequestWithContext(ctx, "POST", r.client.ApiEndpoint+"/instanceOperationCfsParams", bytes.NewBuffer(jsonData))
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create param request: %s", err))
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		if r.client.ApiToken != "" {
			httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
		}

		httpResp, err = r.client.HttpClient.Do(httpReq)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to submit parameter: %s", err))
			return
		}

		respBody, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()

		if httpResp.StatusCode != http.StatusCreated && httpResp.StatusCode != http.StatusOK {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Submit parameter %d failed with status %d: %s", paramId, httpResp.StatusCode, string(respBody)))
			return
		}
	}

	// Step 4: Run operation
	httpReq, err = http.NewRequestWithContext(ctx, "POST", r.client.ApiEndpoint+"/instanceOperations/"+operationId+"/run", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create run request: %s", err))
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if r.client.ApiToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
	}

	httpResp, err = r.client.HttpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to run operation: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusCreated && httpResp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Run operation failed with status %d: %s", httpResp.StatusCode, string(body)))
		return
	}

	// Wait and read status
	time.Sleep(5 * time.Second)

	readData, err := r.readInstance(ctx, instanceId)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read instance after creation: %s", err))
		return
	}

	data.Status = types.StringValue(readData.Status)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *QuickStartResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data QuickStartResourceModel

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

func (r *QuickStartResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Not Supported", "Quick Start resource cannot be updated")
}

func (r *QuickStartResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data QuickStartResourceModel

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
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create delete operation: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Create delete operation failed with status %d: %s", httpResp.StatusCode, string(body)))
		return
	}

	location := httpResp.Header.Get("Location")
	if location == "" {
		resp.Diagnostics.AddError("API Error", "No Location header in operation response")
		return
	}
	operationId := location[2:]

	// Run delete operation
	httpReq, err = http.NewRequestWithContext(ctx, "POST", r.client.ApiEndpoint+"/instanceOperations/"+operationId+"/run", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create run request: %s", err))
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if r.client.ApiToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
	}

	httpResp, err = r.client.HttpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to run delete operation: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Run delete operation failed with status %d: %s", httpResp.StatusCode, string(body)))
		return
	}
}

func (r *QuickStartResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *QuickStartResource) readInstance(ctx context.Context, instanceId string) (*InstanceResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", r.client.ApiEndpoint+"/instances/"+instanceId, nil)
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
