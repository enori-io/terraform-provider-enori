package provider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Test the pure mapping helpers — the part most likely to harbour a silent bug (the Go zero-value
// omitempty trap, enum casing, null round-tripping). No API/network needed.

func TestOptBool_FalseIsSent(t *testing.T) {
	// Regression: an explicit `false` must survive as a non-nil pointer, NOT be dropped like the
	// classic `omitempty` bool trap would drop it. This is why the wire struct uses *bool.
	got := optBool(types.BoolValue(false))
	if got == nil {
		t.Fatal("optBool(false) returned nil — false would be silently omitted on the wire")
	}
	if *got != false {
		t.Fatalf("optBool(false) = %v, want false", *got)
	}
}

func TestOptBool_NullAndUnknownOmit(t *testing.T) {
	if optBool(types.BoolNull()) != nil {
		t.Error("optBool(null) should be nil (omitted)")
	}
	if optBool(types.BoolUnknown()) != nil {
		t.Error("optBool(unknown) should be nil (omitted)")
	}
}

func TestOptInt64(t *testing.T) {
	if optInt64(types.Int64Null()) != nil {
		t.Error("optInt64(null) should be nil")
	}
	// Explicit zero must survive (e.g. failure_threshold = 0).
	got := optInt64(types.Int64Value(0))
	if got == nil || *got != 0 {
		t.Fatalf("optInt64(0) = %v, want 0", got)
	}
}

func TestOptString(t *testing.T) {
	if optString(types.StringNull()) != nil {
		t.Error("optString(null) should be nil")
	}
	got := optString(types.StringValue("GET"))
	if got == nil || *got != "GET" {
		t.Fatalf("optString(GET) = %v, want GET", got)
	}
}

func TestToClientMonitor_FollowRedirectsFalse(t *testing.T) {
	m := MonitorResourceModel{
		ID:              types.StringValue(""),
		Name:            types.StringValue("site"),
		URL:             types.StringValue("https://example.com"),
		Type:            types.StringValue("website"),
		FollowRedirects: types.BoolValue(false),
		AlertChannelIds: types.SetNull(types.StringType),
		Tags:            types.SetNull(types.StringType),
	}
	// Leave the rest null.
	m.GroupName = types.StringNull()
	m.IntervalSeconds = types.Int64Null()
	m.TimeoutSeconds = types.Int64Null()
	m.HTTPMethod = types.StringNull()
	m.ExpectedStatusCode = types.Int64Null()
	m.ExpectedKeyword = types.StringNull()
	m.RequestBody = types.StringNull()
	m.CustomUserAgent = types.StringNull()
	m.Port = types.Int64Null()
	m.SslExpiryWarningDays = types.Int64Null()
	m.FailureThreshold = types.Int64Null()
	m.AlertOnDown = types.BoolNull()
	m.AlertOnRecovered = types.BoolNull()

	out, diags := toClientMonitor(context.Background(), m)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if out.FollowRedirects == nil || *out.FollowRedirects != false {
		t.Fatalf("follow_redirects=false must be sent as an explicit false, got %v", out.FollowRedirects)
	}
	if out.GroupName != nil {
		t.Errorf("unset group_name should be nil (omitted), got %v", *out.GroupName)
	}
	if out.Name != "site" || out.Type != "website" {
		t.Errorf("name/type not mapped: %q/%q", out.Name, out.Type)
	}
}

func TestApplyClientMonitor_LowercasesTypeAndHandlesNulls(t *testing.T) {
	keyword := "healthy"
	interval := int64(60)
	followRedirects := false
	api := &Monitor{
		ID:              "mon_123",
		Name:            "My site",
		URL:             "https://example.com",
		Type:            "Website", // API returns PascalCase enum name
		ExpectedKeyword: &keyword,
		IntervalSeconds: &interval,
		FollowRedirects: &followRedirects,
		// GroupName / Port etc. nil → should map to Null.
	}

	var model MonitorResourceModel
	diags := applyClientMonitor(context.Background(), &model, api)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}

	if model.Type.ValueString() != "website" {
		t.Errorf("type not lowercased: got %q, want website", model.Type.ValueString())
	}
	if model.ID.ValueString() != "mon_123" {
		t.Errorf("id: got %q", model.ID.ValueString())
	}
	if model.ExpectedKeyword.ValueString() != "healthy" {
		t.Errorf("keyword: got %q", model.ExpectedKeyword.ValueString())
	}
	if model.IntervalSeconds.ValueInt64() != 60 {
		t.Errorf("interval: got %d", model.IntervalSeconds.ValueInt64())
	}
	if model.FollowRedirects.IsNull() || model.FollowRedirects.ValueBool() != false {
		t.Errorf("follow_redirects=false should round-trip, got null=%v val=%v",
			model.FollowRedirects.IsNull(), model.FollowRedirects.ValueBool())
	}
	if !model.GroupName.IsNull() {
		t.Errorf("nil group_name should map to Null, got %q", model.GroupName.ValueString())
	}
	if !model.Port.IsNull() {
		t.Errorf("nil port should map to Null")
	}
}

func TestTypeOneOfValidator(t *testing.T) {
	ctx := context.Background()
	v := stringvalidator.OneOf(monitorTypes...)

	for _, ty := range monitorTypes {
		resp := &validator.StringResponse{}
		v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue(ty)}, resp)
		if resp.Diagnostics.HasError() {
			t.Errorf("supported type %q should pass validation", ty)
		}
	}
	// PascalCase (the API's own casing) and unsupported types must be rejected at config-validate time —
	// this is what prevents the "inconsistent result after apply" crash class.
	for _, bad := range []string{"Website", "WEBSITE", "browser", "apiflow", "ssl", "nope"} {
		resp := &validator.StringResponse{}
		v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue(bad)}, resp)
		if !resp.Diagnostics.HasError() {
			t.Errorf("type %q should be rejected by the OneOf validator", bad)
		}
	}
}

func TestNormalizeMonitorType(t *testing.T) {
	cases := map[string]string{
		"Website":    "website", // PascalCase → lowercase
		"website":    "website",
		"DNS":        "dns",
		"Ssl":        "website", // legacy alias → modern
		"Https":      "website",
		"Http":       "website",
		"Api":        "website",
		"Reputation": "website",
		"Ping":       "ping",
	}
	for in, want := range cases {
		if got := normalizeMonitorType(in); got != want {
			t.Errorf("normalizeMonitorType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStringSetRoundTrip(t *testing.T) {
	ctx := context.Background()
	set, diags := stringSetOrNull(ctx, []string{"a", "b"})
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	back, diags := optStringSet(ctx, set)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if back == nil || len(*back) != 2 || (*back)[0] != "a" || (*back)[1] != "b" {
		t.Fatalf("set round-trip mismatch: %v", back)
	}

	// nil slice → empty set (valid, non-null) for Computed attributes.
	empty, diags := stringSetOrNull(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if empty.IsNull() {
		t.Error("nil slice should map to an empty (non-null) set")
	}
}

func TestOptStringSet_NullVsEmpty(t *testing.T) {
	ctx := context.Background()

	// Null set → nil pointer → field omitted on the wire ("no change").
	got, _ := optStringSet(ctx, types.SetNull(types.StringType))
	if got != nil {
		t.Errorf("null set should map to nil pointer (omit), got %v", *got)
	}

	// Explicit empty set → NON-nil pointer to empty slice → sends `[]` (clears the value).
	emptySet, d := types.SetValueFrom(ctx, types.StringType, []string{})
	if d.HasError() {
		t.Fatalf("unexpected diags: %v", d)
	}
	got2, _ := optStringSet(ctx, emptySet)
	if got2 == nil {
		t.Fatal("explicit empty set must map to a non-nil pointer so `[]` is sent (clear), not omitted")
	}
	if len(*got2) != 0 {
		t.Errorf("expected empty slice, got %v", *got2)
	}
}

// TestMonitorJSON_EmptySetClears is the regression guard for the P0 the reviewer caught: an explicit
// empty tags/channels set MUST serialize to `[]` (clear), while an unset one MUST be omitted.
func TestMonitorJSON_EmptySetClears(t *testing.T) {
	empty := []string{}
	withEmpty := Monitor{Name: "x", URL: "y", Type: "website", Tags: &empty}
	b, err := json.Marshal(withEmpty)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"tags":[]`) {
		t.Errorf("explicit empty tags must serialize to `\"tags\":[]` (clear); got %s", b)
	}

	unset := Monitor{Name: "x", URL: "y", Type: "website", Tags: nil}
	b2, err := json.Marshal(unset)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b2), `"tags"`) {
		t.Errorf("unset tags must be omitted from the wire; got %s", b2)
	}
}
