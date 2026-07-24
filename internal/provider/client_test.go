package provider

import "testing"

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	cases := map[string]string{
		"https://api.enori.io/":   "https://api.enori.io",
		"https://api.enori.io///": "https://api.enori.io",
		"https://api.enori.io":    "https://api.enori.io",
		"":                        defaultBaseURL, // empty falls back to the default (no trailing slash)
	}
	for in, want := range cases {
		if got := NewClient(in, "key").baseURL; got != want {
			t.Errorf("NewClient(%q).baseURL = %q, want %q", in, got, want)
		}
	}
}
