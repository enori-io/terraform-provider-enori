package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// testAccProtoV6ProviderFactories wires the provider under test for acceptance tests.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"enori": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck skips acceptance tests unless a real API key is present. resource.Test itself also
// no-ops unless TF_ACC=1 is set, so `go test ./...` in CI stays fast + network-free.
func testAccPreCheck(t *testing.T) {
	if os.Getenv("ENORI_API_KEY") == "" {
		t.Skip("ENORI_API_KEY not set — skipping acceptance test (set TF_ACC=1 + ENORI_API_KEY to run)")
	}
}

// TestAccMonitorResource exercises the full lifecycle (create → import → update) against the REAL
// Enori API. It creates and destroys a throwaway website monitor, so it needs a live key with
// monitors:read/write. Run with: TF_ACC=1 ENORI_API_KEY=... go test ./internal/provider/ -run TestAcc
func TestAccMonitorResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ // create
				Config: testAccMonitorConfig("tf-acc-example", "https://example.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("enori_monitor.test", "name", "tf-acc-example"),
					resource.TestCheckResourceAttr("enori_monitor.test", "type", "website"),
					resource.TestCheckResourceAttr("enori_monitor.test", "url", "https://example.com"),
					resource.TestCheckResourceAttrSet("enori_monitor.test", "id"),
				),
			},
			{ // import
				ResourceName:      "enori_monitor.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{ // update (in place — name is mutable)
				Config: testAccMonitorConfig("tf-acc-renamed", "https://example.com"),
				Check: resource.TestCheckResourceAttr(
					"enori_monitor.test", "name", "tf-acc-renamed"),
			},
		},
	})
}

func testAccMonitorConfig(name, url string) string {
	return fmt.Sprintf(`
resource "enori_monitor" "test" {
  name = %q
  url  = %q
  type = "website"
}
`, name, url)
}
