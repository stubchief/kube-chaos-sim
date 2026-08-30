# Yandex Container Registry for Docker images
resource "yandex_container_registry" "chaos_sim" {
  name      = var.registry_name
  folder_id = var.folder_id
}