package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &OrganizationResource{}

func NewOrganizationResource() resource.Resource {
	return &OrganizationResource{}
}

type OrganizationResource struct {
	client *NubesClient
}

type OrganizationResourceModel struct {
	ID               types.String `tfsdk:"id"`
	DisplayName      types.String `tfsdk:"display_name"`
	Description      types.String `tfsdk:"description"`
	Platform         types.String `tfsdk:"platform"`
	OrganizationType types.String `tfsdk:"organization_type"`
	ResourceRealm    types.String `tfsdk:"resource_realm"`
	Status           types.String `tfsdk:"status"`
}

func (r *OrganizationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (r *OrganizationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Cloud Director Organization resource - корневая сущность для управления облачной инфраструктурой",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "UUID организации",
			},
			"display_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Отображаемое имя организации",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Описание организации",
			},
			"platform": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Платформа для развертывания (например: ngcloud.ru)",
			},
			"organization_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Тип организации: iaas (с доступом к Cloud Director) или saas (управляется Nubes)",
			},
			"resource_realm": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Realm ресурса (по умолчанию: vcd)",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Текущий статус организации",
			},
		},
	}
}

func (r *OrganizationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *OrganizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrganizationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set default resource_realm if not provided
	if data.ResourceRealm.IsNull() || data.ResourceRealm.IsUnknown() {
		data.ResourceRealm = types.StringValue("vcd")
	}

	// Step 1: Create instance
	createReq := CreateInstanceRequest{
		ServiceId:   19, // Cloud Director Organization
		DisplayName: data.DisplayName.ValueString(),
		Descr:       data.Description.ValueString(),
	}

	jsonData, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to marshal create request: %s", err))
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

	// Extract instance ID from Location header
	location := httpResp.Header.Get("Location")
	if location == "" {
		resp.Diagnostics.AddError("API Error", "No Location header in response")
		return
	}
	instanceId := location[2:] // Remove "./"

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

	// Extract operation ID from Location header
	location = httpResp.Header.Get("Location")
	if location == "" {
		resp.Diagnostics.AddError("API Error", "No Location header in operation response")
		return
	}
	operationId := location[2:] // Remove "./"

	// Step 3: Submit operation parameters
	if err := r.submitOrganizationParams(ctx, operationId, data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to submit operation parameters: %s", err))
		return
	}

	// Step 4: Wait for completion and read status
	time.Sleep(5 * time.Second)

	readData, err := r.readInstance(ctx, instanceId)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read instance after creation: %s", err))
		return
	}

	data.Status = types.StringValue(readData.Status)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationResourceModel

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

func (r *OrganizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OrganizationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Trigger modify operation
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

	// Extract operation ID and submit parameters
	location := httpResp.Header.Get("Location")
	if location != "" {
		operationId := location[2:]
		if err := r.submitOrganizationParams(ctx, operationId, data); err != nil {
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

func (r *OrganizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OrganizationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// First suspend the organization
	suspendReq := CreateOperationRequest{
		InstanceUid: data.ID.ValueString(),
		Operation:   "suspend",
	}

	jsonData, err := json.Marshal(suspendReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to marshal suspend request: %s", err))
		return
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", r.client.ApiEndpoint+"/instanceOperations", bytes.NewBuffer(jsonData))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create suspend request: %s", err))
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if r.client.ApiToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
	}

	httpResp, err := r.client.HttpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to suspend organization: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError(
			"API Error",
			fmt.Sprintf("Suspend operation failed with status %d: %s", httpResp.StatusCode, string(body)),
		)
		return
	}

	// Note: Actual deletion requires 14 days wait after suspend
	// For now we just suspend and remove from state
	resp.Diagnostics.AddWarning(
		"Organization Suspended",
		"Organization has been suspended. Actual deletion requires 14 days wait period and must be performed manually.",
	)
}

func (r *OrganizationResource) readInstance(ctx context.Context, instanceId string) (*InstanceResponse, error) {
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

func (r *OrganizationResource) submitOrganizationParams(ctx context.Context, operationUid string, data OrganizationResourceModel) error {
	// Step 1: Get operation details with parameters
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

	// Step 2: Submit each parameter
	for _, param := range operationResp.CfsParams {
		valToSend := ""

		// Map parameters by ID
		switch param.SvcOperationCfsParamId {
		case 418: // platform
			if !data.Platform.IsNull() && !data.Platform.IsUnknown() {
				valToSend = data.Platform.ValueString()
			}
		case 556: // organizationType
			if !data.OrganizationType.IsNull() && !data.OrganizationType.IsUnknown() {
				valToSend = data.OrganizationType.ValueString()
			}
		default:
			// Use existing or default value
			if param.ParamValue != nil {
				valToSend = *param.ParamValue
			} else if param.DefaultValue != nil {
				valToSend = *param.DefaultValue
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
			return fmt.Errorf("unable to submit param: %s", err)
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(httpResp.Body)
			return fmt.Errorf("submit parameter id %d failed with status %d: %s",
				param.SvcOperationCfsParamId, httpResp.StatusCode, string(body))
		}
	}

	// Step 3: Run the operation
	httpReq, err = http.NewRequestWithContext(ctx, "POST",
		r.client.ApiEndpoint+"/instanceOperations/"+operationUid+"/run", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return fmt.Errorf("unable to create run request: %s", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if r.client.ApiToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
	}

	httpResp, err = r.client.HttpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("unable to run operation: %s", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("run operation failed with status %d: %s", httpResp.StatusCode, string(body))
	}

	return nil
}
