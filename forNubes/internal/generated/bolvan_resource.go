package generated

import (
	"context"
	"strconv"

	"terraform-provider-mycloud/internal/core"
	// "terraform-provider-mycloud/internal/provider" REMOVED CYCLE

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ========= RESOURCE DEFINITION (Auto-Generated) =========
// Service: Bolvanka (Dummy)
// Service ID: 1

var _ resource.Resource = &BolvankaResource{}

func NewBolvankaResource() resource.Resource {
	return &BolvankaResource{}
}

type BolvankaResource struct {
	client *core.UniversalClient
}

type BolvankaResourceModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`

	// Params
	DurationMs  types.Int64 `tfsdk:"duration_ms"`   // ID 198
	FailAtStart types.Bool  `tfsdk:"fail_at_start"` // ID 199
}

func (r *BolvankaResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bolvanka"
}

func (r *BolvankaResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Computed: true},
			"display_name": schema.StringAttribute{Required: true},
			"duration_ms": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Sleep duration in ms (Param 198)",
			},
			"fail_at_start": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Fail immediately (Param 199)",
			},
		},
	}
}

func (r *BolvankaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data BolvankaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// === MAP PARAMETERS (The Stringifier Logic) ===
	params := make(map[int]string)

	// 198: durationMs (int -> string)
	params[198] = strconv.FormatInt(data.DurationMs.ValueInt64(), 10)

	// 199: failAtStart (bool -> string)
	valRef := "false"
	if data.FailAtStart.ValueBool() {
		valRef = "true"
	}
	params[199] = valRef

	// Call Core
	id, err := r.client.CreateGenericInstance(ctx, 1, data.DisplayName.ValueString(), params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	data.ID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BolvankaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Implemented as shim for now
}

func (r *BolvankaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}
func (r *BolvankaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (r *BolvankaResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	// Assuming Provider passes the correct *UniversalClient or compatible interface
	client, ok := req.ProviderData.(*core.UniversalClient)
	if !ok {
		resp.Diagnostics.AddError("Error", "Wrong client type expected *core.UniversalClient")
		return
	}

	r.client = client
}
