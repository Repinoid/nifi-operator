package generated

import (
	"context"
	"strconv"
	"strings"

	"terraform-provider-mycloud/internal/core"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ========= RESOURCE DEFINITION (Auto-Generated, Universal Flow + Lifecycle) =========
// Service: Bolvanka (Dummy)
// Service ID: 1

var _ resource.Resource = &BolvankaUniversalLifecycleResource{}

func NewBolvankaUniversalLifecycleResource() resource.Resource {
	return &BolvankaUniversalLifecycleResource{}
}

type BolvankaUniversalLifecycleResource struct {
	client *core.UniversalClient
}

type BolvankaUniversalLifecycleModel struct {
	ID             types.String `tfsdk:"id"`
	DisplayName    types.String `tfsdk:"display_name"`
	DurationMs     types.Int64  `tfsdk:"duration_ms"`
	FailAtStart    types.Bool   `tfsdk:"fail_at_start"`
	DeleteMode     types.String `tfsdk:"delete_mode"`
	ResumeIfExists types.Bool   `tfsdk:"resume_if_exists"`
}

func (r *BolvankaUniversalLifecycleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bolvanka_universal_lifecycle"
}

func (r *BolvankaUniversalLifecycleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"display_name": schema.StringAttribute{
				Required: true,
			},
			"duration_ms": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Sleep duration in ms (Param 198)",
			},
			"fail_at_start": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Fail immediately (Param 199)",
			},
			"delete_mode": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("state_only"),
				MarkdownDescription: "Deletion mode: delete | suspend | state_only",
			},
			"resume_if_exists": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "If true, attempt resume/adopt when instance already exists",
			},
		},
	}
}

func (r *BolvankaUniversalLifecycleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data BolvankaUniversalLifecycleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	displayName := data.DisplayName.ValueString()

	// If resume_if_exists: try to find existing instance by display_name
	if data.ResumeIfExists.ValueBool() {
		existing, err := r.client.FindInstanceByDisplayName(ctx, 1, displayName)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", err.Error())
			return
		}
		if existing != nil {
			status := strings.ToLower(existing.ExplainedStatus)
			if strings.Contains(status, "suspend") || strings.Contains(status, "suspended") {
				if err := r.client.RunInstanceOperationUniversal(ctx, existing.InstanceUid, "resume", nil); err != nil {
					resp.Diagnostics.AddError("Client Error", err.Error())
					return
				}
			}

			data.ID = types.StringValue(existing.InstanceUid)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	params := map[int]string{
		198: strconv.FormatInt(data.DurationMs.ValueInt64(), 10),
		199: boolToString(data.FailAtStart.ValueBool()),
	}

	id, err := r.client.CreateGenericInstanceUniversalV6(ctx, 1, displayName, params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	data.ID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BolvankaUniversalLifecycleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Implemented as shim for now
}

func (r *BolvankaUniversalLifecycleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data BolvankaUniversalLifecycleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := map[int]string{
		198: strconv.FormatInt(data.DurationMs.ValueInt64(), 10),
		199: boolToString(data.FailAtStart.ValueBool()),
	}

	if err := r.client.RunInstanceOperationUniversal(ctx, data.ID.ValueString(), "modify", params); err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BolvankaUniversalLifecycleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BolvankaUniversalLifecycleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mode := strings.ToLower(state.DeleteMode.ValueString())
	if mode == "" {
		mode = "state_only"
	}

	switch mode {
	case "state_only":
		return
	case "suspend", "delete":
		if err := r.client.RunInstanceOperationUniversal(ctx, state.ID.ValueString(), mode, nil); err != nil {
			resp.Diagnostics.AddError("Client Error", err.Error())
			return
		}
	default:
		resp.Diagnostics.AddError("Invalid delete_mode", "Use: delete | suspend | state_only")
		return
	}
}

func (r *BolvankaUniversalLifecycleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*core.UniversalClient)
	if !ok {
		resp.Diagnostics.AddError("Error", "Wrong client type expected *core.UniversalClient")
		return
	}

	r.client = client
}

func boolToString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
