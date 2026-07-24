package provider

// enori_monitor resource — the flagship P1 resource. STATUS: pre-alpha, compiles clean — see main.go.
//
// Scope (v0.1.0): the common cross-type + HTTP/website + alerting core, every field of which round-
// trips through BOTH CreateMonitorRequest/UpdateMonitorRequest AND the MonitorDto response (verified
// 2026-07-24) so there is no read-back drift. Deep type-specific config (browser steps, ApiFlow,
// DNS routing, device emulation, encrypted variables) is intentionally deferred — see DESIGN.md §2.
//
// Design notes:
//   - type is RequiresReplace: UpdateMonitorRequest has no Type field (a monitor's type is immutable).
//   - Optional fields are Optional+Computed+UseStateForUnknown. The Enori update endpoint is a partial
//     merge (a null field = "no change") and supplies server-side defaults, so Computed lets the server
//     be authoritative and avoids perpetual "known after apply" churn. Caveat (documented): removing an
//     attribute from config keeps its last value rather than clearing it — set it explicitly to change.
//   - type/status casing: the API returns the PascalCase enum name ("Website"); we lowercase on read so
//     state matches the lowercase values used in config. Send is case-insensitive on the API side.

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// monitorTypes is the set of monitor types this provider supports (lowercase). Restricting input to
// lowercase is what makes the read-time lowercasing of the API's PascalCase response consistent with
// config for the Required (non-Computed) `type` attribute — without it, `type = "Website"` would trip
// Terraform's "provider produced inconsistent result after apply" check. Browser/ApiFlow are omitted
// because they require step definitions the provider does not yet model.
var monitorTypes = []string{"website", "ping", "port", "dns", "domain", "job"}

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
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	GroupName            types.String `tfsdk:"group_name"`
	URL                  types.String `tfsdk:"url"`
	Type                 types.String `tfsdk:"type"`
	IntervalSeconds      types.Int64  `tfsdk:"interval_seconds"`
	TimeoutSeconds       types.Int64  `tfsdk:"timeout_seconds"`
	HTTPMethod           types.String `tfsdk:"http_method"`
	ExpectedStatusCode   types.Int64  `tfsdk:"expected_status_code"`
	ExpectedKeyword      types.String `tfsdk:"expected_keyword"`
	RequestBody          types.String `tfsdk:"request_body"`
	CustomUserAgent      types.String `tfsdk:"custom_user_agent"`
	FollowRedirects      types.Bool   `tfsdk:"follow_redirects"`
	Port                 types.Int64  `tfsdk:"port"`
	SslExpiryWarningDays types.Int64  `tfsdk:"ssl_expiry_warning_days"`
	FailureThreshold     types.Int64  `tfsdk:"failure_threshold"`
	AlertOnDown          types.Bool   `tfsdk:"alert_on_down"`
	AlertOnRecovered     types.Bool   `tfsdk:"alert_on_recovered"`
	AlertChannelIds      types.Set    `tfsdk:"alert_channel_ids"`
	Tags                 types.Set    `tfsdk:"tags"`
}

func (r *MonitorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (r *MonitorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	// Shared plan modifiers for the Optional+Computed attributes.
	strKeep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	intKeep := []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}
	boolKeep := []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}
	setKeep := []planmodifier.Set{setplanmodifier.UseStateForUnknown()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Enori uptime monitor.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Monitor identifier assigned by Enori.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable monitor name (1–100 chars).",
				Required:            true,
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "Target URL or host to monitor.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Monitor type (lowercase): `website`, `ping`, `port`, `dns`, `domain`, or `job`. " +
					"Changing the type forces a new monitor (the type is immutable).",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{stringvalidator.OneOf(monitorTypes...)},
			},
			"group_name": schema.StringAttribute{
				MarkdownDescription: "Optional group the monitor belongs to.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       strKeep,
			},
			"interval_seconds": schema.Int64Attribute{
				MarkdownDescription: "Check interval in seconds (30–31104000). Server default: 300.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       intKeep,
			},
			"timeout_seconds": schema.Int64Attribute{
				MarkdownDescription: "Per-check timeout in seconds (5–300). Server default: 30.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       intKeep,
			},
			"http_method": schema.StringAttribute{
				MarkdownDescription: "HTTP method for website checks (e.g. `GET`, `POST`, `HEAD`). Server default: `GET`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       strKeep,
			},
			"expected_status_code": schema.Int64Attribute{
				MarkdownDescription: "Expected HTTP status code (100–599) for website checks. Server default: 200.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       intKeep,
			},
			"expected_keyword": schema.StringAttribute{
				MarkdownDescription: "Keyword that must appear in the response body (website checks).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       strKeep,
			},
			"request_body": schema.StringAttribute{
				MarkdownDescription: "Request body sent with the check (e.g. for POST website checks).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       strKeep,
			},
			"custom_user_agent": schema.StringAttribute{
				MarkdownDescription: "Custom User-Agent header for website checks (max 512 chars).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       strKeep,
			},
			"follow_redirects": schema.BoolAttribute{
				MarkdownDescription: "Whether to follow HTTP redirects (website checks). Server default: true.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       boolKeep,
			},
			"port": schema.Int64Attribute{
				MarkdownDescription: "Target port (1–65535) for port checks.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       intKeep,
			},
			"ssl_expiry_warning_days": schema.Int64Attribute{
				MarkdownDescription: "Days before SSL expiry to warn (7, 14, 30, or 60). Server default: 30.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       intKeep,
			},
			"failure_threshold": schema.Int64Attribute{
				MarkdownDescription: "Consecutive failures before the monitor is marked down (0–10).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       intKeep,
			},
			"alert_on_down": schema.BoolAttribute{
				MarkdownDescription: "Send an alert when the monitor goes down. Server default: true.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       boolKeep,
			},
			"alert_on_recovered": schema.BoolAttribute{
				MarkdownDescription: "Send an alert when the monitor recovers. Server default: true.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       boolKeep,
			},
			"alert_channel_ids": schema.SetAttribute{
				MarkdownDescription: "IDs of alert channels to notify.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       setKeep,
			},
			"tags": schema.SetAttribute{
				MarkdownDescription: "Tags for organizing and filtering monitors (lowercase alphanumeric + hyphens).",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       setKeep,
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

// ---- framework type <-> pointer helpers ----

func optString(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

func optInt64(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := v.ValueInt64()
	return &i
}

func optBool(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

// optStringSet returns nil for an unset (null/unknown) Set so the field is omitted, but a NON-NIL
// pointer (to a possibly-empty slice) for a set the user provided — including an explicit empty set,
// so `tags = []` sends `[]` and actually clears the value rather than being a silent no-op.
func optStringSet(ctx context.Context, v types.Set) (*[]string, diag.Diagnostics) {
	if v.IsNull() || v.IsUnknown() {
		return nil, nil
	}
	out := []string{}
	d := v.ElementsAs(ctx, &out, false)
	return &out, d
}

func strOrNull(v *string) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}

func int64OrNull(v *int64) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*v)
}

func boolOrNull(v *bool) types.Bool {
	if v == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*v)
}

func stringSetOrNull(ctx context.Context, s []string) (types.Set, diag.Diagnostics) {
	if s == nil {
		s = []string{}
	}
	return types.SetValueFrom(ctx, types.StringType, s)
}

// toClientMonitor builds the wire payload from the plan/state model.
func toClientMonitor(ctx context.Context, m MonitorResourceModel) (Monitor, diag.Diagnostics) {
	var diags diag.Diagnostics
	channels, d := optStringSet(ctx, m.AlertChannelIds)
	diags.Append(d...)
	tags, d2 := optStringSet(ctx, m.Tags)
	diags.Append(d2...)

	return Monitor{
		ID:                   m.ID.ValueString(),
		Name:                 m.Name.ValueString(),
		URL:                  m.URL.ValueString(),
		Type:                 m.Type.ValueString(),
		GroupName:            optString(m.GroupName),
		IntervalSeconds:      optInt64(m.IntervalSeconds),
		TimeoutSeconds:       optInt64(m.TimeoutSeconds),
		HTTPMethod:           optString(m.HTTPMethod),
		ExpectedStatusCode:   optInt64(m.ExpectedStatusCode),
		ExpectedKeyword:      optString(m.ExpectedKeyword),
		RequestBody:          optString(m.RequestBody),
		CustomUserAgent:      optString(m.CustomUserAgent),
		FollowRedirects:      optBool(m.FollowRedirects),
		Port:                 optInt64(m.Port),
		SslExpiryWarningDays: optInt64(m.SslExpiryWarningDays),
		FailureThreshold:     optInt64(m.FailureThreshold),
		AlertOnDown:          optBool(m.AlertOnDown),
		AlertOnRecovered:     optBool(m.AlertOnRecovered),
		AlertChannelIds:      channels,
		Tags:                 tags,
	}, diags
}

// applyClientMonitor copies the API response back into the model (state). type/status are lowercased
// so state matches the lowercase values used in config.
func applyClientMonitor(ctx context.Context, m *MonitorResourceModel, api *Monitor) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(api.ID)
	m.Name = types.StringValue(api.Name)
	m.URL = types.StringValue(api.URL)
	m.Type = types.StringValue(strings.ToLower(api.Type))
	m.GroupName = strOrNull(api.GroupName)
	m.IntervalSeconds = int64OrNull(api.IntervalSeconds)
	m.TimeoutSeconds = int64OrNull(api.TimeoutSeconds)
	m.HTTPMethod = strOrNull(api.HTTPMethod)
	m.ExpectedStatusCode = int64OrNull(api.ExpectedStatusCode)
	m.ExpectedKeyword = strOrNull(api.ExpectedKeyword)
	m.RequestBody = strOrNull(api.RequestBody)
	m.CustomUserAgent = strOrNull(api.CustomUserAgent)
	m.FollowRedirects = boolOrNull(api.FollowRedirects)
	m.Port = int64OrNull(api.Port)
	m.SslExpiryWarningDays = int64OrNull(api.SslExpiryWarningDays)
	m.FailureThreshold = int64OrNull(api.FailureThreshold)
	m.AlertOnDown = boolOrNull(api.AlertOnDown)
	m.AlertOnRecovered = boolOrNull(api.AlertOnRecovered)

	channels, d := stringSetOrNull(ctx, derefSlice(api.AlertChannelIds))
	diags.Append(d...)
	m.AlertChannelIds = channels

	tags, d2 := stringSetOrNull(ctx, derefSlice(api.Tags))
	diags.Append(d2...)
	m.Tags = tags

	return diags
}

func derefSlice(v *[]string) []string {
	if v == nil {
		return nil
	}
	return *v
}

func (r *MonitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MonitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, d := toClientMonitor(ctx, plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateMonitor(ctx, payload)
	if err != nil {
		resp.Diagnostics.AddError("Error creating monitor", err.Error())
		return
	}

	resp.Diagnostics.Append(applyClientMonitor(ctx, &plan, created)...)
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
		if err == errNotFound {
			// The monitor was deleted out-of-band — drop it from state so Terraform re-creates it.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading monitor", err.Error())
		return
	}

	resp.Diagnostics.Append(applyClientMonitor(ctx, &state, monitor)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *MonitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan MonitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, d := toClientMonitor(ctx, plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateMonitor(ctx, plan.ID.ValueString(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Error updating monitor", err.Error())
		return
	}

	resp.Diagnostics.Append(applyClientMonitor(ctx, &plan, updated)...)
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
