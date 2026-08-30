terraform {
  backend "s3" {
    # bucket is passed via -backend-config at terraform init time
    # (backend block is processed before variables are available)
    endpoints = {
      s3 = "https://storage.yandexcloud.net"
    }
    region                      = "us-east-1"
    key                         = "terraform.tfstate"
    skip_region_validation      = true
    skip_credentials_validation = true
    skip_requesting_account_id  = true
    skip_s3_checksum            = true
    use_path_style              = true
    # access_key/secret_key are read from env:
    # AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY (already in GitHub Secrets)
  }

  required_providers {
    yandex = {
      source  = "yandex-cloud/yandex"
      version = "~> 0.130"
    }
  }
}

provider "yandex" {
  service_account_key_file = "/tmp/key.json"
}
