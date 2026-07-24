# A website monitor with keyword + status-code checks and alerting.
resource "enori_monitor" "marketing_site" {
  name                 = "Marketing site"
  url                  = "https://www.example.com"
  type                 = "website"
  interval_seconds     = 60
  expected_status_code = 200
  expected_keyword     = "Welcome"
  follow_redirects     = true
  alert_on_down        = true
  alert_on_recovered   = true
  tags                 = ["production", "marketing"]
}

# A grouped API health check that notifies specific alert channels.
resource "enori_monitor" "api_health" {
  name                 = "Public API health"
  group_name           = "Production"
  url                  = "https://api.example.com/health"
  type                 = "website"
  interval_seconds     = 30
  expected_status_code = 200
  failure_threshold    = 2
  alert_channel_ids    = ["chan_abc123", "chan_def456"]
}

# A raw TCP port check.
resource "enori_monitor" "db_port" {
  name             = "Postgres port"
  url              = "db.example.com"
  type             = "port"
  port             = 5432
  interval_seconds = 60
}

# A DNS monitor.
resource "enori_monitor" "primary_dns" {
  name             = "Primary DNS"
  url              = "example.com"
  type             = "dns"
  interval_seconds = 300
}
