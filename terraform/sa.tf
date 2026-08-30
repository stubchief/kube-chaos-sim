# CI service account is created manually (see bootstrap instructions in README.md)
# because Terraform cannot assign IAM roles without already having IAM admin permissions.
# The SA ID is passed via variable (ci_sa_id) and used by the k8s cluster.

# Node service account — for k8s nodes (pull images from YCR)
resource "yandex_iam_service_account" "node" {
  name        = "kube-chaos-sim-node"
  description = "SA for k8s node group (image puller from YCR)"
}

resource "yandex_resourcemanager_folder_iam_binding" "node_puller" {
  folder_id = var.folder_id
  role      = "container-registry.images.puller"
  members   = ["serviceAccount:${yandex_iam_service_account.node.id}"]
}
