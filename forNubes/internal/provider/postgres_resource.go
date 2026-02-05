package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &PostgresResource{}
var _ resource.ResourceWithImportState = &PostgresResource{}

type PostgresResource struct {
	client *NubesClient
}

func NewPostgresResource() resource.Resource {
	return &PostgresResource{}
}

type PostgresResourceModel struct {
	ID                       types.String `tfsdk:"id"`
	Name                     types.String `tfsdk:"name"`
	ResourceRealm            types.String `tfsdk:"resource_realm"`
	S3Uid                    types.String `tfsdk:"s3_uid"`
	Cpu                      types.Int64  `tfsdk:"cpu"`
	Ram                      types.Int64  `tfsdk:"ram"`
	DiskSize                 types.Int64  `tfsdk:"disk_size"`
	NodesCount               types.Int64  `tfsdk:"nodes_count"`
	Version                  types.String `tfsdk:"version"`
	BackupSchedule           types.String `tfsdk:"backup_schedule"`
	BackupRetention          types.Int64  `tfsdk:"backup_retention"`
	Parameters               types.String `tfsdk:"parameters"`
	EnablePgPoolerMaster     types.Bool   `tfsdk:"enable_pgpooler_master"`
	EnablePgPoolerSlave      types.Bool   `tfsdk:"enable_pgpooler_slave"`
	AllowNoSSL               types.Bool   `tfsdk:"allow_no_ssl"`
	AutoScale                types.Bool   `tfsdk:"auto_scale"`
	AutoScalePercentage      types.Int64  `tfsdk:"auto_scale_percentage"`
	AutoScaleTechWindow      types.Int64  `tfsdk:"auto_scale_tech_window"`
	AutoScaleQuotaGb         types.Int64  `tfsdk:"auto_scale_quota_gb"`
	EnableExternalMaster     types.Bool   `tfsdk:"enable_external_master"`
	EnableExternalSlave      types.Bool   `tfsdk:"enable_external_slave"`
	IpSpaceMaster            types.String `tfsdk:"ip_space_master"`
	IpSpaceSlave             types.String `tfsdk:"ip_space_slave"`
	DeletionProtection       types.Bool   `tfsdk:"deletion_protection"`

	// Computed
	AdminUser                 types.String `tfsdk:"admin_user"`
	AdminPassword             types.String `tfsdk:"admin_password"`
	StandbyUser               types.String `tfsdk:"standby_user"`
	StandbyPassword           types.String `tfsdk:"standby_password"`
	InternalHost              types.String `tfsdk:"internal_host"`
	VaultUserPath             types.String `tfsdk:"vault_user_path"`
	VaultUrl                  types.String `tfsdk:"vault_url"`
	MonitoringUrl             types.String `tfsdk:"monitoring_url"`
	InternalConnectMaster     types.String `tfsdk:"internal_connect_master"`
	InternalConnectSlave      types.String `tfsdk:"internal_connect_slave"`
	ExternalConnectMasterIp   types.String `tfsdk:"external_connect_master_ip"`
	ExternalConnectMasterFqdn types.String `tfsdk:"external_connect_master_fqdn"`
	ExternalConnectSlaveIp    types.String `tfsdk:"external_connect_slave_ip"`
	ExternalConnectSlaveFqdn  types.String `tfsdk:"external_connect_slave_fqdn"`
}

func (r *PostgresResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_postgres"
}

func (r *PostgresResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Postgres cluster.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the Postgres instance",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"resource_realm": schema.StringAttribute{
				Required:    true,
				Description: "Deployment platform (param 102). Example: k8s-3.ext.nubes.ru",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"s3_uid": schema.StringAttribute{
				Required:    true,
				Description: "UID Корневой услуги S3 (Param 23). Это UUID экземпляра услуги S3 Object Storage (svcId 12). Например: 6d6061cb-b0c1-44b9-8969-a70f08fe673c",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cpu": schema.Int64Attribute{
				Required:    true,
				Description: "CPU in millicores (param 82)",
			},
			"ram": schema.Int64Attribute{
				Required:    true,
				Description: "Memory in MB (param 81)",
			},
			"disk_size": schema.Int64Attribute{
				Required:    true,
				Description: "Disk size in GB (param 83)",
			},
			"nodes_count": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(1),
				Description: "Number of instances (param 80)",
			},
			"version": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("17"),
				Description: "Postgres Version (param 310)",
			},
			"backup_schedule": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("0 0 * * *"),
				Description: "Backup Schedule (param 265)",
			},
			"backup_retention": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(7),
				Description: "Backup Retention Days (param 266)",
			},
			"parameters": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("{}"),
				Description: "Postgres config as JSON string (param 311)",
			},
			"enable_pgpooler_master": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Enable pgPooler for Master (param 312)",
			},
			"enable_pgpooler_slave": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Enable pgPooler for Slave (param 313)",
			},
			"allow_no_ssl": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Allow No SSL (param 314)",
			},
			"auto_scale": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Enable AutoScale (param 329)",
			},
			"auto_scale_percentage": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(10),
				Description: "AutoScale Percentage (param 330)",
			},
			"auto_scale_tech_window": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				Description: "AutoScale Tech Window (param 331)",
			},
			"auto_scale_quota_gb": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(1),
				Description: "AutoScale Quota GB (param 339)",
			},
			"enable_external_master": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Need External IP for Master (param 145)",
			},
			"enable_external_slave": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Need External IP for Slave (param 146)",
			},
			"ip_space_master": schema.StringAttribute{
				Optional:    true,
				Description: "IP Space Name for Master (param 337)",
			},
			"ip_space_slave": schema.StringAttribute{
				Optional:    true,
				Description: "IP Space Name for Slave (param 338)",
			},
			"deletion_protection": schema.BoolAttribute{
				MarkdownDescription: "If true, the resource will only be removed from Terraform state upon destroy, but will remain in the cloud. If false, destroy will trigger 'suspend' in Nubes.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},

			// Output
			"admin_user": schema.StringAttribute{
				Computed: true,
			},
			"admin_password": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"standby_user": schema.StringAttribute{
				Computed: true,
			},
			"standby_password": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"internal_host": schema.StringAttribute{
				Computed: true,
			},
			"vault_user_path": schema.StringAttribute{
				Computed: true,
			},
			"vault_url": schema.StringAttribute{
				Computed: true,
			},
			"monitoring_url": schema.StringAttribute{
				Computed: true,
			},
			"internal_connect_master": schema.StringAttribute{
				Computed: true,
			},
			"internal_connect_slave": schema.StringAttribute{
				Computed: true,
			},
			"external_connect_master_ip": schema.StringAttribute{
				Computed: true,
			},
			"external_connect_master_fqdn": schema.StringAttribute{
				Computed: true,
			},
			"external_connect_slave_ip": schema.StringAttribute{
				Computed: true,
			},
			"external_connect_slave_fqdn": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *PostgresResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *PostgresResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PostgresResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Creating Postgres resource...")

	// Attempt to resume suspended instance if exists
	tflog.Info(ctx, "Checking for suspended instances to resume...")
	var resumedInstanceUid string
	instances, err := r.client.GetInstances(ctx)
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("Failed to list instances: %s", err))
	} else {
		tflog.Info(ctx, fmt.Sprintf("Scanned %d instances.", len(instances)))
		for _, inst := range instances {
			if inst.DisplayName == plan.Name.ValueString() && inst.ServiceId == 90 {
				tflog.Info(ctx, fmt.Sprintf("Found matching instance name: %s (%s)", inst.DisplayName, inst.InstanceUid))
				details, err := r.client.GetInstanceStateDetails(ctx, inst.InstanceUid)
				if err == nil {
					isSuspended, _ := details["isSuspended"].(bool)
					isDeleted, _ := details["isDeleted"].(bool)
					
					tflog.Info(ctx, fmt.Sprintf("Instance status isSuspended: %v, isDeleted: %v", isSuspended, isDeleted))

					if isSuspended {
						tflog.Info(ctx, fmt.Sprintf("Found suspended instance %s. Resuming...", inst.InstanceUid))
						if err := r.client.RunAction(ctx, inst.InstanceUid, "resume"); err != nil {
							resp.Diagnostics.AddError("Resume Failed", err.Error())
							return
						}
						// Postgres goes to 'running' state, not 'active'
						if err := r.client.WaitForInstanceStatus(ctx, inst.InstanceUid, "running"); err != nil {
							resp.Diagnostics.AddError("Wait for Resume Failed", err.Error())
							return
						}
						resumedInstanceUid = inst.InstanceUid
					} else if !isDeleted {
						tflog.Info(ctx, fmt.Sprintf("Instance %s is already active. Adopting...", inst.InstanceUid))
						resumedInstanceUid = inst.InstanceUid
					}
				}
				break
			}
		}
	}

	params := []InstanceParam{
		{SvcOperationCfsParamId: 102, ParamValue: plan.ResourceRealm.ValueString()},
		{SvcOperationCfsParamId: 23, ParamValue: plan.S3Uid.ValueString()},
		{SvcOperationCfsParamId: 82, ParamValue: fmt.Sprintf("%d", plan.Cpu.ValueInt64())},
		{SvcOperationCfsParamId: 81, ParamValue: fmt.Sprintf("%d", plan.Ram.ValueInt64())},
		{SvcOperationCfsParamId: 83, ParamValue: fmt.Sprintf("%d", plan.DiskSize.ValueInt64())},
		{SvcOperationCfsParamId: 80, ParamValue: fmt.Sprintf("%d", plan.NodesCount.ValueInt64())},
		{SvcOperationCfsParamId: 266, ParamValue: fmt.Sprintf("%d", plan.BackupRetention.ValueInt64())},
		{SvcOperationCfsParamId: 265, ParamValue: plan.BackupSchedule.ValueString()},
		{SvcOperationCfsParamId: 310, ParamValue: plan.Version.ValueString()},
		{SvcOperationCfsParamId: 311, ParamValue: plan.Parameters.ValueString()},
		{SvcOperationCfsParamId: 312, ParamValue: strconv.FormatBool(plan.EnablePgPoolerMaster.ValueBool())},
		{SvcOperationCfsParamId: 313, ParamValue: strconv.FormatBool(plan.EnablePgPoolerSlave.ValueBool())},
		{SvcOperationCfsParamId: 314, ParamValue: strconv.FormatBool(plan.AllowNoSSL.ValueBool())},
		{SvcOperationCfsParamId: 329, ParamValue: strconv.FormatBool(plan.AutoScale.ValueBool())},
		{SvcOperationCfsParamId: 330, ParamValue: fmt.Sprintf("%d", plan.AutoScalePercentage.ValueInt64())},
		{SvcOperationCfsParamId: 331, ParamValue: fmt.Sprintf("%d", plan.AutoScaleTechWindow.ValueInt64())},
		{SvcOperationCfsParamId: 339, ParamValue: fmt.Sprintf("%d", plan.AutoScaleQuotaGb.ValueInt64())},
		{SvcOperationCfsParamId: 145, ParamValue: strconv.FormatBool(plan.EnableExternalMaster.ValueBool())},
		{SvcOperationCfsParamId: 146, ParamValue: strconv.FormatBool(plan.EnableExternalSlave.ValueBool())},
	}

	if !plan.IpSpaceMaster.IsNull() {
		params = append(params, InstanceParam{SvcOperationCfsParamId: 337, ParamValue: plan.IpSpaceMaster.ValueString()})
	} else {
		params = append(params, InstanceParam{SvcOperationCfsParamId: 337, ParamValue: ""})
	}
	if !plan.IpSpaceSlave.IsNull() {
		params = append(params, InstanceParam{SvcOperationCfsParamId: 338, ParamValue: plan.IpSpaceSlave.ValueString()})
	} else {
		params = append(params, InstanceParam{SvcOperationCfsParamId: 338, ParamValue: ""})
	}


	serviceId := 90
	var instanceUid string

	if resumedInstanceUid != "" {
		instanceUid = resumedInstanceUid
	} else {
		svcOperationId, err := r.client.GetOperationId(ctx, serviceId, "create")
		if err != nil {
			resp.Diagnostics.AddError("Failed to get create operation ID", err.Error())
			return
		}

		displayName := plan.Name.ValueString()
		var opUid string
		instanceUid, opUid, err = r.client.CreateInstance(ctx, displayName, serviceId, svcOperationId, params)
		if err != nil {
			resp.Diagnostics.AddError("Error creating Postgres", err.Error())
			return
		}

		// Wait for operation success
		err = r.client.WaitForOperation(ctx, opUid)
		if err != nil {
			resp.Diagnostics.AddError("Error waiting for Postgres creation", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(instanceUid)

	// Initialize computed fields to avoid "Unknown" errors if API doesn't return them
	plan.AdminUser = types.StringNull()
	plan.AdminPassword = types.StringNull()
	plan.StandbyUser = types.StringNull()
	plan.StandbyPassword = types.StringNull()
	plan.InternalHost = types.StringNull()
	plan.VaultUserPath = types.StringNull()
	plan.VaultUrl = types.StringNull()
	plan.MonitoringUrl = types.StringNull()
	plan.InternalConnectMaster = types.StringNull()
	plan.InternalConnectSlave = types.StringNull()
	plan.ExternalConnectMasterIp = types.StringNull()
	plan.ExternalConnectMasterFqdn = types.StringNull()
	plan.ExternalConnectSlaveIp = types.StringNull()
	plan.ExternalConnectSlaveFqdn = types.StringNull()

	// Fetch details to populate Computed fields
	stateData, err := r.client.GetInstanceStateDetails(ctx, instanceUid)
	if err != nil {
		resp.Diagnostics.AddWarning("Failed to populate computed fields", err.Error())
	} else {
		// Vault
		if vault, ok := stateData["vault"].(map[string]interface{}); ok {
			if val, ok := vault["userPath"]; ok {
				plan.VaultUserPath = types.StringValue(fmt.Sprintf("%v", val))
			}
			if val, ok := vault["url"]; ok {
				plan.VaultUrl = types.StringValue(fmt.Sprintf("%v", val))
			}
		}

		// Out (Connect details)
		if out, ok := stateData["out"].(map[string]interface{}); ok {
			// Credentials
			if val, ok := out["adminUser"]; ok {
				plan.AdminUser = types.StringValue(fmt.Sprintf("%v", val))
			}
			if val, ok := out["adminPass"]; ok {
				plan.AdminPassword = types.StringValue(fmt.Sprintf("%v", val))
			}
			if val, ok := out["standbyUser"]; ok {
				plan.StandbyUser = types.StringValue(fmt.Sprintf("%v", val))
			}
			if val, ok := out["standbyPass"]; ok {
				plan.StandbyPassword = types.StringValue(fmt.Sprintf("%v", val))
			}

			// Monitoring
			if monitoring, ok := out["monitoring"].(map[string]interface{}); ok {
				if val, ok := monitoring["allDashboards"]; ok {
					plan.MonitoringUrl = types.StringValue(fmt.Sprintf("%v", val))
				}
			}

			// Internal Connect
			if internalConnect, ok := out["internalConnect"].(map[string]interface{}); ok {
				if val, ok := internalConnect["master"]; ok && val != "" {
					plan.InternalConnectMaster = types.StringValue(fmt.Sprintf("%v", val))
					plan.InternalHost = types.StringValue(fmt.Sprintf("%v", val)) // Sync with master for now
				}
				if val, ok := internalConnect["slave"]; ok && val != "" {
					plan.InternalConnectSlave = types.StringValue(fmt.Sprintf("%v", val))
				}
			}

			// External Connect
			if externalConnect, ok := out["externalConnect"].(map[string]interface{}); ok {
				if master, ok := externalConnect["master"].(map[string]interface{}); ok {
					if val, ok := master["ip"]; ok && val != "" {
						plan.ExternalConnectMasterIp = types.StringValue(fmt.Sprintf("%v", val))
					}
					if val, ok := master["fqdn"]; ok && val != "" {
						plan.ExternalConnectMasterFqdn = types.StringValue(fmt.Sprintf("%v", val))
					}
				}
				if slave, ok := externalConnect["slave"].(map[string]interface{}); ok {
					if val, ok := slave["ip"]; ok && val != "" {
						plan.ExternalConnectSlaveIp = types.StringValue(fmt.Sprintf("%v", val))
					}
					if val, ok := slave["fqdn"]; ok && val != "" {
						plan.ExternalConnectSlaveFqdn = types.StringValue(fmt.Sprintf("%v", val))
					}
				}
			}
		}
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *PostgresResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PostgresResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	stateData, err := r.client.GetInstanceStateDetails(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading Postgres", err.Error())
		return
	}

	// Update calculated fields
	// Vault
	if vault, ok := stateData["vault"].(map[string]interface{}); ok {
		if val, ok := vault["userPath"]; ok {
			state.VaultUserPath = types.StringValue(fmt.Sprintf("%v", val))
		}
		if val, ok := vault["url"]; ok {
			state.VaultUrl = types.StringValue(fmt.Sprintf("%v", val))
		}
	}

	// Out
	if out, ok := stateData["out"].(map[string]interface{}); ok {
		// Credentials
		if val, ok := out["adminUser"]; ok {
			state.AdminUser = types.StringValue(fmt.Sprintf("%v", val))
		}
		if val, ok := out["adminPass"]; ok {
			state.AdminPassword = types.StringValue(fmt.Sprintf("%v", val))
		}
		if val, ok := out["standbyUser"]; ok {
			state.StandbyUser = types.StringValue(fmt.Sprintf("%v", val))
		}
		if val, ok := out["standbyPass"]; ok {
			state.StandbyPassword = types.StringValue(fmt.Sprintf("%v", val))
		}

		// Monitoring
		if monitoring, ok := out["monitoring"].(map[string]interface{}); ok {
			if val, ok := monitoring["allDashboards"]; ok {
				state.MonitoringUrl = types.StringValue(fmt.Sprintf("%v", val))
			}
		}

		// Internal Connect
		if internalConnect, ok := out["internalConnect"].(map[string]interface{}); ok {
			if val, ok := internalConnect["master"]; ok && val != "" {
				state.InternalConnectMaster = types.StringValue(fmt.Sprintf("%v", val))
				state.InternalHost = types.StringValue(fmt.Sprintf("%v", val))
			}
			if val, ok := internalConnect["slave"]; ok && val != "" {
				state.InternalConnectSlave = types.StringValue(fmt.Sprintf("%v", val))
			}
		}

		// External Connect
		if externalConnect, ok := out["externalConnect"].(map[string]interface{}); ok {
			if master, ok := externalConnect["master"].(map[string]interface{}); ok {
				if val, ok := master["ip"]; ok && val != "" {
					state.ExternalConnectMasterIp = types.StringValue(fmt.Sprintf("%v", val))
				}
				if val, ok := master["fqdn"]; ok && val != "" {
					state.ExternalConnectMasterFqdn = types.StringValue(fmt.Sprintf("%v", val))
				}
			}
			if slave, ok := externalConnect["slave"].(map[string]interface{}); ok {
				if val, ok := slave["ip"]; ok && val != "" {
					state.ExternalConnectSlaveIp = types.StringValue(fmt.Sprintf("%v", val))
				}
				if val, ok := slave["fqdn"]; ok && val != "" {
					state.ExternalConnectSlaveFqdn = types.StringValue(fmt.Sprintf("%v", val))
				}
			}
		}
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *PostgresResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state PostgresResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updating Postgres resource...")

	// Validation: Verify Disk Size reduction
	if plan.DiskSize.ValueInt64() < state.DiskSize.ValueInt64() {
		resp.Diagnostics.AddError(
			"Invalid Disk Upgrade",
			fmt.Sprintf("Decreasing disk size is not supported (Current: %d GB, Requested: %d GB)", state.DiskSize.ValueInt64(), plan.DiskSize.ValueInt64()),
		)
		return
	}

	// Find operation ID for 'modify'
	opId, err := r.client.GetOperationIdForInstance(ctx, state.ID.ValueString(), "modify")
	if err != nil {
		resp.Diagnostics.AddError("Failed to find update operation", err.Error())
		return
	}

	// Create Operation
	opPayload := map[string]interface{}{
		"instanceUid":    state.ID.ValueString(),
		"svcOperationId": opId,
		"operation":      "modify",
	}

	var opUid string
	opUid, err = r.client.postIgnoreResponse(ctx, "/instanceOperations", opPayload, true)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create update operation", err.Error())
		return
	}
	if opUid == "" {
		resp.Diagnostics.AddError("Failed to get operation UID", "Empty UID returned")
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

	type NamedParam struct {
		Name  string
		Value string
	}
	namedParams := []NamedParam{
		{"resourceCPU", fmt.Sprintf("%d", plan.Cpu.ValueInt64())},
		{"resourceMemory", fmt.Sprintf("%d", plan.Ram.ValueInt64())},
		{"resourceDisk", fmt.Sprintf("%d", plan.DiskSize.ValueInt64())},
		{"resourceInstances", fmt.Sprintf("%d", plan.NodesCount.ValueInt64())},
		{"ext_BACKUP_NUM_TO_RETAIN", fmt.Sprintf("%d", plan.BackupRetention.ValueInt64())},
		{"ext_BACKUP_SCHEDULE", plan.BackupSchedule.ValueString()},
		{"appVersion", plan.Version.ValueString()},
		{"jsonParameters", plan.Parameters.ValueString()},
		{"enablePgPoolerMaster", strconv.FormatBool(plan.EnablePgPoolerMaster.ValueBool())},
		{"enablePgPoolerSlave", strconv.FormatBool(plan.EnablePgPoolerSlave.ValueBool())},
		{"allowNoSSL", strconv.FormatBool(plan.AllowNoSSL.ValueBool())},
		{"autoScale", strconv.FormatBool(plan.AutoScale.ValueBool())},
		{"autoScalePercentage", fmt.Sprintf("%d", plan.AutoScalePercentage.ValueInt64())},
		{"autoScaleTechWindow", fmt.Sprintf("%d", plan.AutoScaleTechWindow.ValueInt64())},
		{"autoScaleQuotaGb", fmt.Sprintf("%d", plan.AutoScaleQuotaGb.ValueInt64())},
		{"needExternalAddressMaster", strconv.FormatBool(plan.EnableExternalMaster.ValueBool())},
		{"needExternalAddressSlave", strconv.FormatBool(plan.EnableExternalSlave.ValueBool())},
	}

	if !plan.IpSpaceMaster.IsNull() {
		namedParams = append(namedParams, NamedParam{"ipSpaceNameMaster", plan.IpSpaceMaster.ValueString()})
	} else {
		namedParams = append(namedParams, NamedParam{"ipSpaceNameMaster", ""})
	}
	if !plan.IpSpaceSlave.IsNull() {
		namedParams = append(namedParams, NamedParam{"ipSpaceNameSlave", plan.IpSpaceSlave.ValueString()})
	} else {
		namedParams = append(namedParams, NamedParam{"ipSpaceNameSlave", ""})
	}

	// Add Params using dynamic mapping
	for _, np := range namedParams {
		id, ok := paramMapping[np.Name]
		if !ok {
			tflog.Debug(ctx, fmt.Sprintf("Param %s not supported for this operation, skipping", np.Name))
			continue
		}

		paramPayload := map[string]interface{}{
			"instanceOperationUid":   opUid,
			"svcOperationCfsParamId": id,
			"paramValue":             np.Value,
		}
		if _, err := r.client.postIgnoreResponse(ctx, "/instanceOperationCfsParams", paramPayload, false); err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Failed to add param %s (%d)", np.Name, id), err.Error())
			return
		}
	}

	// Run
	runUrl := fmt.Sprintf("/instanceOperations/%s/run", opUid)
	if _, err := r.client.postIgnoreResponse(ctx, runUrl, map[string]interface{}{}, false); err != nil {
		resp.Diagnostics.AddError("Failed to run update", err.Error())
		return
	}
	
	// Wait for operation success
	err = r.client.WaitForOperation(ctx, opUid)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for Postgres update", err.Error())
		return
	}
	
	// Fetch details to populate Computed fields
	stateData, err := r.client.GetInstanceStateDetails(ctx, state.ID.ValueString())
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("Failed to refresh state after update: %s", err))
	}

	// Initialize all computed fields to avoid 'unknown value' errors after apply
	plan.AdminUser = types.StringNull()
	plan.AdminPassword = types.StringNull()
	plan.StandbyUser = types.StringNull()
	plan.StandbyPassword = types.StringNull()
	plan.VaultUserPath = types.StringNull()
	plan.VaultUrl = types.StringNull()
	plan.MonitoringUrl = types.StringNull()
	plan.InternalConnectMaster = types.StringNull()
	plan.InternalHost = types.StringNull()
	plan.InternalConnectSlave = types.StringNull()
	plan.ExternalConnectMasterIp = types.StringNull()
	plan.ExternalConnectMasterFqdn = types.StringNull()
	plan.ExternalConnectSlaveIp = types.StringNull()
	plan.ExternalConnectSlaveFqdn = types.StringNull()

	if err == nil {
		// Vault
		if vault, ok := stateData["vault"].(map[string]interface{}); ok {
			if val, ok := vault["userPath"]; ok {
				plan.VaultUserPath = types.StringValue(fmt.Sprintf("%v", val))
			}
			if val, ok := vault["url"]; ok {
				plan.VaultUrl = types.StringValue(fmt.Sprintf("%v", val))
			}
		}

		// Out (Connect details)
		if out, ok := stateData["out"].(map[string]interface{}); ok {
			// Credentials
			if val, ok := out["adminUser"]; ok {
				plan.AdminUser = types.StringValue(fmt.Sprintf("%v", val))
			}
			if val, ok := out["adminPass"]; ok {
				plan.AdminPassword = types.StringValue(fmt.Sprintf("%v", val))
			}
			if val, ok := out["standbyUser"]; ok {
				plan.StandbyUser = types.StringValue(fmt.Sprintf("%v", val))
			}
			if val, ok := out["standbyPass"]; ok {
				plan.StandbyPassword = types.StringValue(fmt.Sprintf("%v", val))
			}

			// Monitoring
			if monitoring, ok := out["monitoring"].(map[string]interface{}); ok {
				if val, ok := monitoring["allDashboards"]; ok {
					plan.MonitoringUrl = types.StringValue(fmt.Sprintf("%v", val))
				}
			}

			// Internal Connect
			if internalConnect, ok := out["internalConnect"].(map[string]interface{}); ok {
				if val, ok := internalConnect["master"]; ok && val != "" {
					plan.InternalConnectMaster = types.StringValue(fmt.Sprintf("%v", val))
					plan.InternalHost = types.StringValue(fmt.Sprintf("%v", val))
				}
				if val, ok := internalConnect["slave"]; ok && val != "" {
					plan.InternalConnectSlave = types.StringValue(fmt.Sprintf("%v", val))
				}
			}

			// External Connect
			if externalConnect, ok := out["externalConnect"].(map[string]interface{}); ok {
				if master, ok := externalConnect["master"].(map[string]interface{}); ok {
					if val, ok := master["ip"]; ok && val != "" {
						plan.ExternalConnectMasterIp = types.StringValue(fmt.Sprintf("%v", val))
					}
					if val, ok := master["fqdn"]; ok && val != "" {
						plan.ExternalConnectMasterFqdn = types.StringValue(fmt.Sprintf("%v", val))
					}
				}
				if slave, ok := externalConnect["slave"].(map[string]interface{}); ok {
					if val, ok := slave["ip"]; ok && val != "" {
						plan.ExternalConnectSlaveIp = types.StringValue(fmt.Sprintf("%v", val))
					}
					if val, ok := slave["fqdn"]; ok && val != "" {
						plan.ExternalConnectSlaveFqdn = types.StringValue(fmt.Sprintf("%v", val))
					}
				}
			}
		}
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}


func (r *PostgresResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PostgresResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.DeletionProtection.ValueBool() {
		tflog.Warn(ctx, "Deletion Protection is ENABLED for Postgres. Resource will remain active in Nubes Cloud. Manual cleanup required.")
		return
	}

	tflog.Info(ctx, "Deletion Protection is DISABLED for Postgres. Triggering 'suspend'...")

	instanceId := data.ID.ValueString()
	
	// Выполняем операцию suspend
	// Для Postgres используем тот же механизм Instance Run что и для VDC
	err := r.triggerSuspend(ctx, instanceId)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to suspend Postgres: %s", err))
		return
	}

	tflog.Info(ctx, "Postgres suspended successfully. It will be permanently deleted from the cloud in 14 days.")
}

func (r *PostgresResource) triggerSuspend(ctx context.Context, instanceId string) error {
	// Выполняем операцию suspend через новый унифицированный метод
	err := r.client.RunAction(ctx, instanceId, "suspend")
	if err != nil {
		return err
	}

	// Ждем пока статус изменится на suspended
	return r.client.WaitForInstanceStatus(ctx, instanceId, "suspended")
}

func (r *PostgresResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve the full instance data
	inst, err := r.client.GetInstanceFull(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch instance for import", err.Error())
		return
	}

	var state PostgresResourceModel
	state.ID = types.StringValue(req.ID)

	// Top-level fields
	if v, ok := inst["displayName"].(string); ok {
		state.Name = types.StringValue(v)
	}
	if v, ok := inst["resourceRealm"].(string); ok {
		state.ResourceRealm = types.StringValue(v)
	}

	// State params
	if stateMap, ok := inst["state"].(map[string]interface{}); ok {
		if params, ok := stateMap["params"].(map[string]interface{}); ok {
			
			// String or Int handling
			toInt64 := func(v interface{}) int64 {
				switch val := v.(type) {
				case float64: return int64(val)
				case int: return int64(val)
				case string:
					if i, err := strconv.ParseInt(val, 10, 64); err == nil {
						return i
					}
				}
				return 0
			}

			if v, ok := params["resourceCPU"]; ok { state.Cpu = types.Int64Value(toInt64(v)) }
			if v, ok := params["resourceMemory"]; ok { state.Ram = types.Int64Value(toInt64(v)) }
			if v, ok := params["resourceDisk"]; ok { state.DiskSize = types.Int64Value(toInt64(v)) }
			if v, ok := params["resourceInstances"]; ok { state.NodesCount = types.Int64Value(toInt64(v)) }
			
			if v, ok := params["s3Uid"]; ok { state.S3Uid = types.StringValue(fmt.Sprintf("%v", v)) }
			if v, ok := params["appVersion"]; ok { state.Version = types.StringValue(fmt.Sprintf("%v", v)) }
			
			if v, ok := params["ext_BACKUP_SCHEDULE"]; ok { state.BackupSchedule = types.StringValue(fmt.Sprintf("%v", v)) }
			if v, ok := params["ext_BACKUP_NUM_TO_RETAIN"]; ok { state.BackupRetention = types.Int64Value(toInt64(v)) }

			// Booleans
			toBool := func(v interface{}) bool {
				switch val := v.(type) {
				case bool: return val
				case string: return val == "true"
				}
				return false
			}
			
			if v, ok := params["enablePgPoolerMaster"]; ok { state.EnablePgPoolerMaster = types.BoolValue(toBool(v)) }
			if v, ok := params["enablePgPoolerSlave"]; ok { state.EnablePgPoolerSlave = types.BoolValue(toBool(v)) }
			if v, ok := params["allowNoSSL"]; ok { state.AllowNoSSL = types.BoolValue(toBool(v)) }
			if v, ok := params["autoScale"]; ok { state.AutoScale = types.BoolValue(toBool(v)) }
			if v, ok := params["needExternalAddressMaster"]; ok { state.EnableExternalMaster = types.BoolValue(toBool(v)) }
			if v, ok := params["needExternalAddressSlave"]; ok { state.EnableExternalSlave = types.BoolValue(toBool(v)) }

			if v, ok := params["ipSpaceNameMaster"]; ok { 
				if s := fmt.Sprintf("%v", v); s != "" { state.IpSpaceMaster = types.StringValue(s) }
			}
			if v, ok := params["ipSpaceNameSlave"]; ok { 
				if s := fmt.Sprintf("%v", v); s != "" { state.IpSpaceSlave = types.StringValue(s) }
			}

			// Autoscale details
			if v, ok := params["autoScalePercentage"]; ok { state.AutoScalePercentage = types.Int64Value(toInt64(v)) }
			if v, ok := params["autoScaleTechWindow"]; ok { state.AutoScaleTechWindow = types.Int64Value(toInt64(v)) }
			if v, ok := params["autoScaleQuotaGb"]; ok { state.AutoScaleQuotaGb = types.Int64Value(toInt64(v)) }
			
			// JSON Parameters (map -> json string)
			if v, ok := params["jsonParameters"]; ok {
				if jsonBytes, err := json.Marshal(v); err == nil {
					state.Parameters = types.StringValue(string(jsonBytes))
				}
			}
		}
	}
	
	state.DeletionProtection = types.BoolValue(true)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}




