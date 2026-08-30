output "cluster_id" {
  description = "Kubernetes cluster ID"
  value       = yandex_kubernetes_cluster.chaos_sim.id
}

output "node_group_id" {
  description = "Node group ID"
  value       = yandex_kubernetes_node_group.chaos_sim_nodes.id
}

output "registry_id" {
  description = "Container registry ID"
  value       = yandex_container_registry.chaos_sim.id
}

output "registry_url" {
  description = "Container registry URL (cr.yandex/<id>)"
  value       = "cr.yandex/${yandex_container_registry.chaos_sim.id}"
}

output "cluster_master_endpoint" {
  description = "External master endpoint for kubectl"
  value       = yandex_kubernetes_cluster.chaos_sim.master[0].external_v4_endpoint
}
