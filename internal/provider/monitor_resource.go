package provider

// enori_monitor resource — the flagship P1 resource. STATUS: first-draft, uncompiled — see main.go.
// The attribute set is the MVP subset (name, group_name, url, type, interval_seconds, timeout_seconds,
// paused). Extend to the full CreateMonitorRequest field set during the Go-verified build (DESIGN.md §2).

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &MonitorResource{}
	_ resource.ResourceWithImportState = &MonitorResource{}
	_ resource.ResourceWithConfigure   = &MonitorResource{}
)

func NewMonitorResource() resource.Resource {
	return &MonitorResource{}
}

type MonitorResource struct {
	client *Client
}

// MonitorResourceModel maps the resource schema to a Go type.
type MonitorResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	GroupName       types.String `tfsdk:"group_name"`
	URL             types.String `tfsdk:"url"`
	Type            types.String `tfsdk:"type"`
	IntervalSeconds types.Int64  `tfsdk:"interval_seconds"`
	TimeoutSeconds  types.Int64  `tfsdk:"timeout_seconds"`
	Paused          types.Bool   `tfsdk:"paused"`
}

func (r *MonitorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (r *MonitorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Enori uptime monitor.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Monitor identifier assigned by Enori.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable monitor name.",
				Required:            true,
			},
			"group_name": schema.StringAttribute{
				MarkdownDescription: "Optional group the monitor belongs to.",
				Optional:            true,
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "Target URL or host to monitor.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Monitor type: `website`, `ping`, `port`, `dns`, `domain`, `job`, `browser`, or `apiflow`.",
				Required:            true,
			},
			"interval_seconds": schema.Int64Attribute{
				MarkdownDescription: "Check interval in seconds. Defaults to `60`.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(60),
			},
			"timeout_seconds": schema.Int64Attribute{
				MarkdownDescription: "Per-check timeout in seconds. Defaults to `30`.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(30),
			},
			"paused": schema.BoolAttribute{
				MarkdownDescription: "Whether the monitor is paused. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
}

func (r *MonitorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Client, got: %T. Please report this to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (m MonitorResourceModel) toClientMonitor() Monitor {
	return Monitor{
		ID:              m.ID.ValueString(),
		Name:            m.Name.ValueString(),
		GroupName:       m.GroupName.ValueString(),
		URL:             m.URL.ValueString(),
		Type:            m.Type.ValueString(),
		IntervalSeconds: m.IntervalSeconds.ValueInt64(),
		TimeoutSeconds:  m.TimeoutSeconds.ValueInt64(),
		Paused:          m.Paused.ValueBool(),
	}
}

func applyClientMonitor(m *MonitorResourceModel, api *Monitor) {
	m.ID = types.StringValue(api.ID)
	m.Name = types.StringValue(api.Name)
	if api.GroupName == "" {
		m.GroupName = types.StringNull()
	} else {
		m.GroupName = types.StringValue(api.GroupName)
	}
	m.URL = types.StringValue(api.URL)
	m.Type = types.StringValue(api.Type)
	m.IntervalSeconds = types.Int64Value(api.IntervalSeconds)
	m.TimeoutSeconds = types.Int64Value(api.TimeoutSeconds)
	m.Paused = types.BoolValue(api.Paused)
}

func (r *MonitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MonitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateMonitor(ctx, plan.toClientMonitor())
	if err != nil {
		resp.Diagnostics.AddError("Error creating monitor", err.Error())
		return
	}

	applyClientMonitor(&plan, created)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *MonitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state MonitorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	monitor, err := r.client.GetMonitor(ctx, state.ID.ValueString())
	if err != nil {
		// TODO (Go-verified build): distinguish 404 → resp.State.RemoveResource(ctx) from transient errors.
		resp.Diagnostics.AddError("Error reading monitor", err.Error())
		return
	}

	applyClientMonitor(&state, monitor)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *MonitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan MonitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateMonitor(ctx, plan.ID.ValueString(), plan.toClientMonitor())
	if err != nil {
		resp.Diagnostics.AddError("Error updating monitor", err.Error())
		return
	}

	applyClientMonitor(&plan, updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *MonitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state MonitorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteMonitor(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting monitor", err.Error())
		return
	}
}

func (r *MonitorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
