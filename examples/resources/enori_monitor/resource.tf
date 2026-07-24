resource "enori_monitor" "marketing_site" {
  name             = "Marketing site"
  url              = "https://www.example.com"
  type             = "website"
  interval_seconds = 60
  timeout_seconds  = 30
}

resource "enori_monitor" "api_health" {
  name             = "Public API health"
  group_name       = "Production"
  url              = "https://api.example.com/health"
  type             = "website"
  interval_seconds = 30
}

resource "enori_monitor" "primary_dns" {
  name             = "Primary DNS"
  url              = "example.com"
  type             = "dns"
  interval_seconds = 300
}
