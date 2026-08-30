variable "folder_id" {
  description = "Yandex Cloud folder ID"
  type        = string
}

variable "ci_sa_id" {
  description = "ID of the CI service account (created manually, see README.md)"
  type        = string
}

variable "zone" {
  description = "Primary zone for master"
  type        = string
  default     = "ru-central1-a"
}

variable "cluster_name" {
  description = "Kubernetes cluster name"
  type        = string
  default     = "chaos-sim"
}

variable "registry_name" {
  description = "Container registry name"
  type        = string
  default     = "chaos-sim-registry"
}
