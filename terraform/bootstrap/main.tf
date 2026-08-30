terraform {
  required_providers {
    yandex = {
      source  = "yandex-cloud/yandex"
      version = "~> 0.130"
    }
  }
}

variable "folder_id" {
  type = string
}

provider "yandex" {
  folder_id = var.folder_id
}

resource "yandex_storage_bucket" "tf_state" {
  bucket    = "chaos-sim-tf-state"
  folder_id = var.folder_id

  versioning {
    enabled = true
  }
}