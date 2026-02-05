package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &PgAdminResource{}
var _ resource.ResourceWithImportState = &PgAdminResource{}

type PgAdminResource struct {
	client *NubesClient
}

func NewPgAdminResource() resource.Resource {
	return &PgAdminResource{}
}

type PgAdminResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Domain        types.String `tfsdk:"domain"`
	ResourceRealm types.String `tfsdk:"resource_realm"`
	Cpu           types.Int64  `tfsdk:"cpu"`
	Memory        types.Int64  `tfsdk:"memory"`
	Disk          types.Int64  `tfsdk:"disk"`
	Email         types.String `tfsdk:"email"`
	Password      types.String `tfsdk:"password"`
	DeletionProtection types.Bool   `tfsdk:"deletion_protection"`
}



func (r *PgAdminResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pgadmin"
}

func (r *PgAdminResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a pgAdmin instance.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"domain": schema.StringAttribute{
				Required:    true,
				Description: "Domain name (param 164)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"resource_realm": schema.StringAttribute{
				Required:    true,
				Description: "Deployment platform (param 169). E.g. k8s-3.ext.nubes.ru",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cpu": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(200),
				Description: "CPU quota in milicores (param 165)",
			},
			"memory": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(256),
				Description: "Memory quota in MB (param 166)",
			},
			"disk": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(1),
				Description: "Disk size in GB (param 167)",
			},
			"email": schema.StringAttribute{
				Required:    true,
				Description: "Login email (param 170)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"password": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Login password (param 171)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"deletion_protection": schema.BoolAttribute{
				MarkdownDescription: "If true, the resource will only be removed from Terraform state upon destroy, but will remain in the cloud. If false, destroy will trigger 'suspend' in Nubes.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
		},
	}
}


func (r *PgAdminResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*NubesClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *NubesClient, got: %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *PgAdminResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PgAdminResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Creating pgAdmin resource...")

	// Params based on pg_admin.har analysis
	params := []InstanceParam{
		{SvcOperationCfsParamId: 164, ParamValue: plan.Domain.ValueString()},
		{SvcOperationCfsParamId: 169, ParamValue: plan.ResourceRealm.ValueString()},
		{SvcOperationCfsParamId: 165, ParamValue: fmt.Sprintf("%d", plan.Cpu.ValueInt64())},
		{SvcOperationCfsParamId: 166, ParamValue: fmt.Sprintf("%d", plan.Memory.ValueInt64())},
		{SvcOperationCfsParamId: 167, ParamValue: fmt.Sprintf("%d", plan.Disk.ValueInt64())},
		{SvcOperationCfsParamId: 170, ParamValue: plan.Email.ValueString()},
		{SvcOperationCfsParamId: 171, ParamValue: plan.Password.ValueString()},
	}

	serviceId := 96
	svcOperationId, err := r.client.GetOperationId(ctx, serviceId, "create")
	if err != nil {
		resp.Diagnostics.AddError("Failed to get create operation ID for pgAdmin", err.Error())
		return
	}

	instanceUid, opUid, err := r.client.CreateInstance(ctx, plan.Domain.ValueString(), serviceId, svcOperationId, params)
	if err != nil {
		resp.Diagnostics.AddError("Error creating pgAdmin", err.Error())
		return
	}

	err = r.client.WaitForOperation(ctx, opUid)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for pgAdmin ready", err.Error())
		return
	}

	plan.ID = types.StringValue(instanceUid)
	// plan.DeletionProtection = types.BoolValue(true)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}



func (r *PgAdminResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PgAdminResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	instanceUid := state.ID.ValueString()
	// Fetch current state
	inst, err := r.client.GetInstanceState(ctx, instanceUid)
	if err != nil {
		// If 404, remove from state
		if strings.Contains(err.Error(), "status 404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading pgAdmin state", err.Error())
		return
	}

	// Map params
	if inst.State != nil && inst.State.Params != nil {
		for k, v := range inst.State.Params {
			valStr := fmt.Sprintf("%v", v)
			// Log for debugging
			tflog.Info(ctx, fmt.Sprintf("PGAdmin Param: %s = %s", k, valStr))

			switch k {
			case "resourceCPU":
				val, _ := strconv.ParseInt(valStr, 10, 64)
				state.Cpu = types.Int64Value(val)
			case "resourceMemory":
				val, _ := strconv.ParseInt(valStr, 10, 64)
				state.Memory = types.Int64Value(val)
			case "resourceDisk":
				val, _ := strconv.ParseInt(valStr, 10, 64)
				state.Disk = types.Int64Value(val)
			case "domain":
				state.Domain = types.StringValue(valStr)
			case "resourceRealm":
				state.ResourceRealm = types.StringValue(valStr)
			case "login":
				state.Email = types.StringValue(valStr)
			}
		}
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *PgAdminResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state PgAdminResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	
	tflog.Info(ctx, "Updating pgAdmin resource...")

	// Find operation ID for 'modify'
	// Based on HAR/Screenshots, modify uses standard "modify" operation
	opId, err := r.client.GetOperationIdForInstance(ctx, state.ID.ValueString(), "modify")
	if err != nil {
		resp.Diagnostics.AddError("Failed to find update operation", err.Error())
		return
	}

	// Payload for OP creation
	opPayload := map[string]interface{}{
		"instanceUid":    state.ID.ValueString(),
		"svcOperationId": opId,
		"operation":      "modify",
	}

	// Create request
	var opUid string
	opUid, err = r.client.postIgnoreResponse(ctx, "/instanceOperations", opPayload, true)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create update operation", err.Error())
		return
	}

	// Fetch Op details to get correct CFS Param IDs for 'modify'
	opDetails, err := r.client.GetInstanceOperation(ctx, opUid)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch operation details", err.Error())
		return
	}

	paramMapping := make(map[string]int)
	for _, p := range opDetails.CfsParams {
		paramMapping[p.SvcOperationCfsParam] = p.SvcOperationCfsParamId
	}
	
	// Create params list from plan
	// We assume standard param names "resourceCPU", "resourceMemory", "resourceDisk" based on screenshot
	// But we use mapping to be safe if IDs differ from Create
	
	type Modification struct {
		Name  string
		Val   string
	}
	
	mods := []Modification{
		{"resourceCPU", fmt.Sprintf("%d", plan.Cpu.ValueInt64())},
		{"resourceMemory", fmt.Sprintf("%d", plan.Memory.ValueInt64())},
		{"resourceDisk", fmt.Sprintf("%d", plan.Disk.ValueInt64())},
	}
	
	params := []InstanceParam{}
	for _, m := range mods {
		if id, ok := paramMapping[m.Name]; ok {
			params = append(params, InstanceParam{SvcOperationCfsParamId: id, ParamValue: m.Val})
		} else {
			tflog.Warn(ctx, fmt.Sprintf("Param %s not found in modify operation", m.Name))
		}
	}
	
	// If no params (e.g. only email changed but modify doesn't support it?), skip
	if len(params) == 0 {
		tflog.Warn(ctx, "No matching parameters found for modify operation. Skipping.")
	} else {
		// Run op
		_, _, err = r.client.RunInstanceOperation(ctx, opUid, params)
		if err != nil {
			resp.Diagnostics.AddError("Error running modify operation", err.Error())
			return
		}
		
		// Wait
		err = r.client.WaitForOperation(ctx, opUid)
		if err != nil {
			resp.Diagnostics.AddError("Error waiting for modification", err.Error())
			return
		}
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}


func (r *PgAdminResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PgAdminResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// User requested NO DELETE for pgAdmin (suspend/protection).
	// We will simply remove it from Terraform state without calling API delete.
	
	if state.DeletionProtection.ValueBool() {
		tflog.Warn(ctx, "Deletion Protection is ENABLED for PgAdmin. Resource will remain active in Nubes Cloud. Manual cleanup required.")
		return
	}

	tflog.Info(ctx, "Deletion Protection is DISABLED for PgAdmin. Triggering 'suspend'...")

	instanceId := state.ID.ValueString()
	
	// Выполняем операцию suspend
	err := r.client.RunAction(ctx, instanceId, "suspend")
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to suspend PgAdmin: %s", err))
		return
	}

	tflog.Info(ctx, "PgAdmin suspended successfully. It will be permanently deleted from the cloud in 14 days.")
}


func (r *PgAdminResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
