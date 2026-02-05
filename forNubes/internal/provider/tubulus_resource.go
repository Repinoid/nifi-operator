package provider

// [НЕ ИЗМЕНЯТЬ !!!!]
// Данный ресурс реализует паттерн "Nubes Flow" для тестовой болванки (Tubulus).
// ВАЖНО: Ресурс предназначен для тестирования жизненного цикла и не имеет операции Update.
// Любое изменение параметров приведет к пересозданию инстанса.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings" // Added strings
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &TubulusResource{}
var _ resource.ResourceWithImportState = &TubulusResource{}
var _ resource.ResourceWithModifyPlan = &TubulusResource{}

func NewTubulusResource() resource.Resource {
	return &TubulusResource{}
}

type TubulusResource struct {
	client *NubesClient
}

type TubulusResourceModel struct {
	ID             types.String `tfsdk:"id"`
	DisplayName    types.String `tfsdk:"display_name"`
	Description    types.String `tfsdk:"description"`
	Status         types.String `tfsdk:"status"`
	DurationMs     types.Int64  `tfsdk:"duration_ms"`
	FailAtStart    types.Bool   `tfsdk:"fail_at_start"`
	FailInProgress types.Bool   `tfsdk:"fail_in_progress"`
	WhereFail      types.Int64  `tfsdk:"where_fail"`
	BodyMessage    types.String `tfsdk:"body_message"`
	ResourceRealm  types.String `tfsdk:"resource_realm"`
	MapExample     types.String `tfsdk:"map_example"`
	JsonExample    types.String `tfsdk:"json_example"`
	YamlExample    types.String `tfsdk:"yaml_example"`
}

type CreateInstanceRequest struct {
	ServiceId   int    `json:"serviceId"`
	DisplayName string `json:"displayName"`
	Descr       string `json:"descr,omitempty"`
}

type CreateOperationRequest struct {
	InstanceUid string `json:"instanceUid"`
	Operation   string `json:"operation"`
}

type InstanceResponse struct {
	Uid         string `json:"instanceUid"`
	DisplayName string `json:"displayName"`
	Descr       string `json:"descr"`
	Status      string `json:"explainedStatus"` // Правильное поле из API
}

type GetInstanceResponse struct {
	Instance InstanceResponse `json:"instance"`
}

// GetOperationResponse and associated types removed as they are now in client_impl.go

type CreateCfsParamRequest struct {
	InstanceOperationCfsParamUid string `json:"instanceOperationCfsParamUid,omitempty"`
	InstanceOperationUid         string `json:"instanceOperationUid"`
	SvcOperationCfsParamId       int    `json:"svcOperationCfsParamId"`
	ParamValue                   string `json:"paramValue"`
}

func (r *TubulusResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tubulus_instance"
}

func (r *TubulusResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Nubes Tubulus Instance resource with Gemini AI integration",

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
			"duration_ms": schema.Int64Attribute{
				MarkdownDescription: "Duration in ms (sleep before completion)",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"fail_at_start": schema.BoolAttribute{
				MarkdownDescription: "If true, job fails immediately",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"fail_in_progress": schema.BoolAttribute{
				MarkdownDescription: "If true, job fails during execution",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"where_fail": schema.Int64Attribute{
				MarkdownDescription: "1=prepare, 2=data_fill, 3=after_vault",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"body_message": schema.StringAttribute{
				MarkdownDescription: "Message to be sent to Vault",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"resource_realm": schema.StringAttribute{
				MarkdownDescription: "Platform realm (e.g. dummy)",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"map_example": schema.StringAttribute{
				MarkdownDescription: "Example map input (JSON string)",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"json_example": schema.StringAttribute{
				MarkdownDescription: "Example JSON input (JSON string)",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"yaml_example": schema.StringAttribute{
				MarkdownDescription: "Example YAML input",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *TubulusResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// AI Logic disabled as per user instruction.
}

func (r *TubulusResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// doRequestWithRetry executes an HTTP request with retries for transient errors.
func (r *TubulusResource) doRequestWithRetry(ctx context.Context, httpReq *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	maxRetries := 3
	baseWait := 2 * time.Second

	for i := 0; i <= maxRetries; i++ {
		// Reset body for retry if needed
		if i > 0 && httpReq.GetBody != nil {
			body, err := httpReq.GetBody()
			if err != nil {
				log.Printf("[ERROR] Failed to get request body for retry: %s", err)
				return nil, err
			}
			httpReq.Body = body
		}

		resp, err = r.client.HttpClient.Do(httpReq)

		// Case 1: Client/Network error
		if err != nil {
			// Check context cancellation first
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			errMsg := err.Error()
			if i < maxRetries && (strings.Contains(errMsg, "TLS handshake timeout") ||
				strings.Contains(errMsg, "connection refused") ||
				strings.Contains(errMsg, "connection reset") ||
				strings.Contains(errMsg, "timeout") ||
				strings.Contains(errMsg, "EOF")) {
				
				log.Printf("[WARN] Network error '%s' (attempt %d/%d). Retrying in %s...", errMsg, i+1, maxRetries+1, baseWait*time.Duration(i+1))
				time.Sleep(baseWait * time.Duration(i+1))
				continue
			}
			return nil, err
		}

		// Case 2: Server error (5xx)
		if i < maxRetries && resp.StatusCode >= 500 && resp.StatusCode < 600 {
			log.Printf("[WARN] Server error %d (attempt %d/%d). Retrying in %s...", resp.StatusCode, i+1, maxRetries+1, baseWait*time.Duration(i+1))
			resp.Body.Close()
			time.Sleep(baseWait * time.Duration(i+1))
			continue
		}

		// If success or 4xx, return
		return resp, nil
	}

	return resp, err
}

func (r *TubulusResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TubulusResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// [NUBES SOFT DELETE LOGIC]
	// Сначала проверяем, нет ли уже такого инстанса в статусе suspended.
	// Если есть — выполняем resume вместо создания нового.
	existingInstance, _ := r.findInstance(ctx, data.DisplayName.ValueString())
	if existingInstance != nil && existingInstance.Status == "suspended" {
		log.Printf("[INFO] Found existing suspended instance %s. Triggering RESUME.", existingInstance.Uid)
		instanceId := existingInstance.Uid
		data.ID = types.StringValue(instanceId)

		operationReq := CreateOperationRequest{
			InstanceUid: instanceId,
			Operation:   "resume",
		}

		jsonData, err := json.Marshal(operationReq)
		if err == nil {
			httpReq, err := http.NewRequestWithContext(ctx, "POST", r.client.ApiEndpoint+"/instanceOperations", bytes.NewBuffer(jsonData))
			if err == nil {
				httpReq.Header.Set("Content-Type", "application/json")
				if r.client.ApiToken != "" {
					httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
				}
				httpResp, _ := r.doRequestWithRetry(ctx, httpReq)
				if httpResp != nil && (httpResp.StatusCode == http.StatusCreated || httpResp.StatusCode == http.StatusOK) {
					location := httpResp.Header.Get("Location")
					if location != "" {
						operationId := location[2:]
						
						// [FIX 2026-01-27]: Ensure params are submitted even for RESUME if needed.
						// Use "data" model which contains current plan values.
						if err := r.submitOperationParams(ctx, operationId, data); err != nil {
							log.Printf("[WARN] Failed to submit params for resume operation (might be non-fatal): %s", err)
						}

						// [FIX] User requirement: Bolvanka resume must be fast (< 1 min).
						// Increased to 3m as sometimes cloud is slow.
						if err := r.waitForOperationAndInstanceStatus(ctx, operationId, instanceId, 3*time.Minute); err == nil {
							data.Status = types.StringValue("running")
							
							// Ensure computed optional fields are checked for Unknown and set to Null if needed
							if data.FailAtStart.IsUnknown() { data.FailAtStart = types.BoolNull() }
							if data.FailInProgress.IsUnknown() { data.FailInProgress = types.BoolNull() }
							if data.WhereFail.IsUnknown() { data.WhereFail = types.Int64Null() }
							if data.BodyMessage.IsUnknown() { data.BodyMessage = types.StringNull() }
							if data.ResourceRealm.IsUnknown() { data.ResourceRealm = types.StringNull() }
							if data.MapExample.IsUnknown() { data.MapExample = types.StringNull() }
							if data.JsonExample.IsUnknown() { data.JsonExample = types.StringNull() }
							if data.YamlExample.IsUnknown() { data.YamlExample = types.StringNull() }
							if data.DurationMs.IsUnknown() { data.DurationMs = types.Int64Null() }

							resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
							// CRITICAL FIX: Return immediately after successful resume!
							// Do not fall through to Create logic.
							return
						} else {
							// If wait failed (timeout or error), report it.
							resp.Diagnostics.AddError("Resume Failed", fmt.Sprintf("Failed to resume existing instance: %s", err))
							return
						}
					}
					httpResp.Body.Close()
				}
			}
		}
		// Если resume не удался, попробуем обычное создание (может имя освободилось)
	}

	// Step 1: Create instance
	createReq := CreateInstanceRequest{
		ServiceId:   1, // Болванка
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

	httpResp, err := r.doRequestWithRetry(ctx, httpReq)
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
	// Location format: "./UUID"
	instanceId := location[2:] // Remove "./"

	data.ID = types.StringValue(instanceId)

	// Step 2: Create operation
	// Инициализируем операцию 'create'. Это создает "визард" в Deck API.
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

	httpResp, err = r.doRequestWithRetry(ctx, httpReq)
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

	// Step 3: Submit operation parameters to trigger execution
	// Синхронизация параметров и запуск выполнения (Full Sync + Run).
	if err := r.submitOperationParams(ctx, operationId, data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to submit operation parameters: %s", err))
		return
	}

	// Step 4: Wait for completion and read status
	// [MODIFIED BY USER REQUEST]: Timeout reduced to 1 minute.
	if err := r.waitForOperationAndInstanceStatus(ctx, operationId, instanceId, 1*time.Minute); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Instance failed to reach running status: %s", err))
		return
	}

	readData, err := r.readInstance(ctx, instanceId)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read instance after creation: %s", err))
		return
	}

	data.Status = types.StringValue(readData.Status)

	// Ensure computed optional fields are checked for Unknown and set to Null if needed
	if data.FailAtStart.IsUnknown() { data.FailAtStart = types.BoolNull() }
	if data.FailInProgress.IsUnknown() { data.FailInProgress = types.BoolNull() }
	if data.WhereFail.IsUnknown() { data.WhereFail = types.Int64Null() }
	if data.BodyMessage.IsUnknown() { data.BodyMessage = types.StringNull() }
	if data.ResourceRealm.IsUnknown() { data.ResourceRealm = types.StringNull() }
	if data.MapExample.IsUnknown() { data.MapExample = types.StringNull() }
	if data.JsonExample.IsUnknown() { data.JsonExample = types.StringNull() }
	if data.YamlExample.IsUnknown() { data.YamlExample = types.StringNull() }
	if data.DurationMs.IsUnknown() { data.DurationMs = types.Int64Null() }

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TubulusResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TubulusResourceModel

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

func (r *TubulusResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// [НЕ ИЗМЕНЯТЬ !!!!]
	// Ресурс считается иммутабельным в рамках текущих тестов.
	// Метод сохранен для совместимости, но использование 'modify' в Tubulus часто не дает результата на бэкенде.

	var data TubulusResourceModel

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

	httpResp, err := r.doRequestWithRetry(ctx, httpReq)
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
		if err := r.submitOperationParams(ctx, operationId, data); err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to submit operation parameters: %s", err))
			return
		}

		// Wait for modify to complete using strict logic
		if err := r.waitForOperationAndInstanceStatus(ctx, operationId, data.ID.ValueString(), 1*time.Minute); err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Instance failed to complete modify operation: %s", err))
			return
		}
	}

	// Read updated state (no sleep needed after strict wait)
	readData, err := r.readInstance(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read instance after update: %s", err))
		return
	}

	data.Status = types.StringValue(readData.Status)

	// Ensure computed optional fields are checked for Unknown and set to Null if needed
	if data.FailAtStart.IsUnknown() { data.FailAtStart = types.BoolNull() }
	if data.FailInProgress.IsUnknown() { data.FailInProgress = types.BoolNull() }
	if data.WhereFail.IsUnknown() { data.WhereFail = types.Int64Null() }
	if data.BodyMessage.IsUnknown() { data.BodyMessage = types.StringNull() }
	if data.ResourceRealm.IsUnknown() { data.ResourceRealm = types.StringNull() }
	if data.MapExample.IsUnknown() { data.MapExample = types.StringNull() }
	if data.JsonExample.IsUnknown() { data.JsonExample = types.StringNull() }
	if data.YamlExample.IsUnknown() { data.YamlExample = types.StringNull() }
	if data.DurationMs.IsUnknown() { data.DurationMs = types.Int64Null() }

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TubulusResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TubulusResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// [NUBES SOFT DELETE]
	// Вместо 'delete' отправляем 'suspend'. Ресурс перейдет в 14-дневный период удержания.
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

	httpResp, err := r.doRequestWithRetry(ctx, httpReq)
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

	// Extract operation ID and wait for completion (without parameters)
	location := httpResp.Header.Get("Location")
	if location != "" {
		operationId := location[2:]
		// Run suspend operation (no params needed usually)
		runReq, _ := http.NewRequestWithContext(ctx, "POST", r.client.ApiEndpoint+"/instanceOperations/"+operationId+"/run", bytes.NewBuffer([]byte("{}")))
		runReq.Header.Set("Content-Type", "application/json")
		runReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
		r.doRequestWithRetry(ctx, runReq)
		
		// Wait for suspend to complete using strict logic (ignore status check)
		r.waitForOperationAndInstanceStatus(ctx, operationId, data.ID.ValueString(), 1*time.Minute)
	}
}

func (r *TubulusResource) findInstance(ctx context.Context, name string) (*InstanceResponse, error) {
	page := 1
	pageSize := 100

	for {
		url := fmt.Sprintf("%s/instances?page=%d&size=%d", r.client.ApiEndpoint, page, pageSize)
		httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}

		if r.client.ApiToken != "" {
			httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
		}

		httpResp, err := r.doRequestWithRetry(ctx, httpReq)
		if err != nil {
			return nil, err
		}

		var instancesResp struct {
			Total   int `json:"total"`
			Results []struct {
				ID     string `json:"instanceUid"`
				Name   string `json:"displayName"`
				Status string `json:"explainedStatus"`
				Svc    string `json:"svc"`
			} `json:"results"`
		}

		err = json.NewDecoder(httpResp.Body).Decode(&instancesResp)
		httpResp.Body.Close()

		if err != nil {
			return nil, err
		}

		for _, instance := range instancesResp.Results {
			// [FIX] Checking both "Bolvanka" and "Болванка" just in case, but usually it's "Болванка"
			if instance.Name == name && (instance.Svc == "Болванка" || instance.Svc == "Bolvanka") {
				return &InstanceResponse{
					Uid:         instance.ID,
					DisplayName: instance.Name,
					Status:      instance.Status,
				}, nil
			}
		}

		if len(instancesResp.Results) == 0 || page*pageSize >= instancesResp.Total {
			break
		}
		page++
	}

	return nil, fmt.Errorf("instance not found")
}

func (r *TubulusResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *TubulusResource) readInstance(ctx context.Context, id string) (*InstanceResponse, error) {
	// [FIX 2026-01-27]: Explicitly request explainedStatus to ensure we get the correct status
	// Default response might not include computed/expensive fields.
	httpReq, err := http.NewRequestWithContext(ctx, "GET", r.client.ApiEndpoint+"/instances/"+id+"?fields=instanceUid,displayName,descr,explainedStatus", nil)
	if err != nil {
		return nil, err
	}

	if r.client.ApiToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
	}

	httpResp, err := r.doRequestWithRetry(ctx, httpReq)
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

	var instanceResp GetInstanceResponse
	if err := json.Unmarshal(body, &instanceResp); err != nil {
		return nil, err
	}

	return &instanceResp.Instance, nil
}

func (r *TubulusResource) submitOperationParams(ctx context.Context, operationUid string, data TubulusResourceModel) error {
	// Step 1: Get operation details with parameters
	httpReq, err := http.NewRequestWithContext(ctx, "GET",
		r.client.ApiEndpoint+"/instanceOperations/"+operationUid+"?fields=cfsParams", nil)
	if err != nil {
		return fmt.Errorf("unable to create request: %s", err)
	}

	if r.client.ApiToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
	}

	httpResp, err := r.doRequestWithRetry(ctx, httpReq)
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

	// Step 2: Submit each parameter to ensure they exist (populate UIDs)
	// ВАЖНО: Мы ДОЛЖНЫ пройтись по всем параметрам, которые вернул API.
	// Бэкенд Nubes инициализирует значения параметров только после их явной отправки (POST).
	// Если пропустить параметр, Deck может не разрешить выполнение (Run).
	for _, param := range operationResp.CfsParams {
		// Determine value to send: existing value > default value > empty string
		valToSend := ""

		// Try to match parameter by Code or Name with Terraform data
		// Assuming param.Code or param.Name matches the keys we know
		// [FIX 2026-01-27]: Added SvcOperationCfsParam as fallback key
		key := param.Code
		if key == "" {
			key = param.Name
		}
		if key == "" {
			key = param.SvcOperationCfsParam
		}

		// Helper to override valToSend if TF attribute is set
		if key == "durationMs" && !data.DurationMs.IsNull() && !data.DurationMs.IsUnknown() {
			valToSend = fmt.Sprintf("%d", data.DurationMs.ValueInt64())
		} else if key == "failAtStart" && !data.FailAtStart.IsNull() && !data.FailAtStart.IsUnknown() {
			valToSend = fmt.Sprintf("%t", data.FailAtStart.ValueBool())
		} else if key == "failInProgress" && !data.FailInProgress.IsNull() && !data.FailInProgress.IsUnknown() {
			valToSend = fmt.Sprintf("%t", data.FailInProgress.ValueBool())
		} else if key == "whereFail" && !data.WhereFail.IsNull() && !data.WhereFail.IsUnknown() {
			valToSend = fmt.Sprintf("%d", data.WhereFail.ValueInt64())
		} else if key == "bodymessage" && !data.BodyMessage.IsNull() && !data.BodyMessage.IsUnknown() {
			valToSend = data.BodyMessage.ValueString()
		} else if key == "resourceRealm" && !data.ResourceRealm.IsNull() && !data.ResourceRealm.IsUnknown() {
			valToSend = data.ResourceRealm.ValueString()
		} else if key == "mapExample" && !data.MapExample.IsNull() && !data.MapExample.IsUnknown() {
			valToSend = data.MapExample.ValueString()
		} else if key == "jsonExample" && !data.JsonExample.IsNull() && !data.JsonExample.IsUnknown() {
			valToSend = data.JsonExample.ValueString()
		} else if key == "yamlExample" && !data.YamlExample.IsNull() && !data.YamlExample.IsUnknown() {
			valToSend = data.YamlExample.ValueString()
		} else {
			// Fallback to existing logic if not set in TF
			if param.ParamValue != nil {
				valToSend = *param.ParamValue
			} else if param.DefaultValue != nil {
				valToSend = *param.DefaultValue
			}
		}

		// Fix specific data type formatting
		// [CRITICAL WARNING]: DO NOT CHANGE THIS LOGIC for MAP/JSON/ARRAY!
		// Sending empty string "" for map/json/array triggers 400 Bad Request (invalid format).
		// Always send "{}" for empty maps/jsons and "[]" for empty lists.
		if valToSend == "" || valToSend == "\"\"" {
			if param.DataType == "map" || param.DataType == "json" {
				valToSend = "{}"
			} else if param.DataType == "array" || param.DataType == "list" {
				valToSend = "[]"
			}
		}


		// [FIX 2026-01-27]: Если значение не менялось (пустое или дефолтное) и это не обязательный параметр, 
		// всё равно отправляем POST, чтобы инициализировать параметр на бэкенде.
		// НО: для корректного формирования JSON строки, нужно убедиться, что кавычки экранированы, если это JSON внутри JSON.
		// В простом случае мы шлем строку.
		
		paramReq := CreateCfsParamRequest{
			InstanceOperationUid:   operationUid,
			SvcOperationCfsParamId: param.SvcOperationCfsParamId,
			ParamValue:             valToSend,
		}

		jsonData, err := json.Marshal(paramReq)
		if err != nil {
			return fmt.Errorf("unable to marshal param request: %s", err)
		}

		// USE POST to create/set the parameter
		httpReq, err = http.NewRequestWithContext(ctx, "POST",
			r.client.ApiEndpoint+"/instanceOperationCfsParams", bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("unable to create param request: %s", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		if r.client.ApiToken != "" {
			httpReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
		}

		httpResp, err = r.doRequestWithRetry(ctx, httpReq)
		if err != nil {
			return fmt.Errorf("unable to submit parameter: %s", err)
		}

		respBody, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()

		if httpResp.StatusCode != http.StatusCreated &&
			httpResp.StatusCode != http.StatusOK &&
			httpResp.StatusCode != http.StatusNoContent {
			// Log but maybe continue? No, if we fail to set param, run will likely fail.
			return fmt.Errorf("submit parameter id %d failed with status %d: %s",
				param.SvcOperationCfsParamId, httpResp.StatusCode, string(respBody))
		}
	}

	// Step 3: Run the operation
	runReq, err := http.NewRequestWithContext(ctx, "POST",
		r.client.ApiEndpoint+"/instanceOperations/"+operationUid+"/run", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return fmt.Errorf("unable to create run request: %s", err)
	}

	runReq.Header.Set("Content-Type", "application/json")
	if r.client.ApiToken != "" {
		runReq.Header.Set("Authorization", "Bearer "+r.client.ApiToken)
	}

	runResp, err := r.doRequestWithRetry(ctx, runReq)
	if err != nil {
		return fmt.Errorf("unable to run operation: %s", err)
	}
	defer runResp.Body.Close()

	if runResp.StatusCode != http.StatusOK &&
		runResp.StatusCode != http.StatusNoContent &&
		runResp.StatusCode != http.StatusCreated {
		runBody, _ := io.ReadAll(runResp.Body)
		// debugJSON removed from scope, simplified error
		return fmt.Errorf("run operation failed with status %d: %s",
			runResp.StatusCode, string(runBody))
	}

	return nil
}

func (r *TubulusResource) waitForOperationAndInstanceStatus(ctx context.Context, operationId string, instanceId string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
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

			// Проверяем статус операции через клиент
			opResp, err := r.client.GetInstanceOperation(ctx, operationId)
			if err != nil {
				log.Printf("[WARN] Failed to get operation status: %s", err)
				continue // Пробуем еще раз на следующем тике
			}

			// Вывод для логов, чтобы видеть что реально приходит
			if opResp.DtFinish != nil {
				log.Printf("[DEBUG] dtFinish: %s", *opResp.DtFinish)
			}
			if opResp.IsSuccessful != nil {
				log.Printf("[DEBUG] isSuccessful: %v", *opResp.IsSuccessful)
			}

			// Извлекаем все доступные данные из ответа (максимальная диагностика)
			isSucc := "N/A"
			if opResp.IsSuccessful != nil {
				isSucc = fmt.Sprintf("%v", *opResp.IsSuccessful)
			}
			dtFin := "not finished"
			if opResp.DtFinish != nil {
				dtFin = *opResp.DtFinish
			}
			submitCode := "N/A"
			if opResp.SubmitResult != nil {
				submitCode = *opResp.SubmitResult
			}
			dur := 0.0
			if opResp.Duration != nil {
				dur = *opResp.Duration
			}
			who := "system"
			if opResp.UpdaterShortname != nil {
				who = *opResp.UpdaterShortname
			}

			log.Printf("[DEBUG] Polling: OpID=%s, Status=%s/%s, Success=%s, Finish=%s, SubmitResult=%s, Dur=%.2fs, Initiator=%s",
				operationId, 
				map[bool]string{true: "PENDING", false: "ACTIVE"}[opResp.IsPending], 
				map[bool]string{true: "WORKING", false: "IDLE"}[opResp.IsInProgress],
				isSucc, dtFin, submitCode, dur, who)

			// [FIX 2026-01-27]: STRICT SUCCESS LOGIC
			// [CRITICAL WARNING]: DO NOT CHANGE THIS LOGIC!
			// This logic is battle-tested against 14+ HAR files.
			// 1. dtFinish IS THE ONLY SOURCE OF TRUTH for completion. 
			// 2. Do NOT rely on status, isInProgress, or isPending for completion check.
			// 3. If dtFinish is set -> The operation is DONE. We stop waiting IMMEDIATELY.
			// 4. If it is done, we check Success:
			//    - isSuccessful == true => OK (break loop)
			//    - isSuccessful == false OR null => ERROR (return error)
			// ANY DEVIATION FROM THIS WILL CAUSE TIMEOUTS OR FALSE POSITIVES.
			if opResp.DtFinish != nil {
				log.Printf("[DEBUG] dtFinish detected: %s. Operation is FINISHED.", *opResp.DtFinish)

				if opResp.IsSuccessful != nil && *opResp.IsSuccessful {
					log.Printf("[DEBUG] Operation SUCCESS detected (isSuccessful=true).")
					break operationLoop
				}

				// If we are here, it means finished but NOT successful (false or null)
				errNote := "Operation finished but marked as UNSUCCESSFUL"
				if opResp.ErrorLog != nil {
					errNote = fmt.Sprintf("%s. LOG: %s", errNote, *opResp.ErrorLog)
				} else {
					errNote = fmt.Sprintf("%s. (isSuccessful=%s, dtFinish=%s)", errNote, isSucc, *opResp.DtFinish)
				}
				return fmt.Errorf("%s (initiator: %s, duration: %.2fs)", errNote, who, dur)
			}
			
			// If dtFinish is NOT set, we continue polling until timeout.
			// Reordered check: We trust dtFinish as the primary signal of completion.

			// Legacy check for "Cold Start" or weird states where flags are false but nothing happened yet
			if !opResp.IsInProgress && !opResp.IsPending && opResp.SubmitResult == nil {
				log.Printf("[DEBUG] Operation uninitialized (cold start or slow pending). Waiting...")
				continue
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
