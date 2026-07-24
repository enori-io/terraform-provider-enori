terraform {
  required_providers {
    enori = {
      source  = "hpatsev/enori"
      version = "~> 0.1"
    }
  }
}

provider "enori" {
  # API key is read from ENORI_API_KEY (recommended). You may also set it here:
  #   api_key = var.enori_api_key   # mark the variable `sensitive = true`
  #
  # endpoint defaults to https://api.enori.io; override for a self-hosted/staging API:
  #   endpoint = "https://api.enori.io"
}
