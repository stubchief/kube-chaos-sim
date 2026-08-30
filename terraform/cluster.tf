# Managed Kubernetes cluster
resource "yandex_kubernetes_cluster" "chaos_sim" {
  name        = var.cluster_name
  description = "Managed k8s cluster for chaos-sim demo"

  network_id              = yandex_vpc_network.chaos_sim.id
  service_account_id      = var.ci_sa_id
  node_service_account_id = yandex_iam_service_account.node.id

  master {
    version = "1.35"

    security_group_ids = [
      yandex_vpc_security_group.k8s_master.id,
      yandex_vpc_security_group.k8s_nodes.id
    ]

    zonal {
      zone      = yandex_vpc_subnet.a.zone
      subnet_id = yandex_vpc_subnet.a.id
    }
    public_ip = true # for CI/CD access
  }
}

# Node group: 3 nodes fixed (1 per zone)
# Budget-optimized: preemptible, 20% CPU guarantee, minimal resources
resource "yandex_kubernetes_node_group" "chaos_sim_nodes" {
  cluster_id  = yandex_kubernetes_cluster.chaos_sim.id
  name        = "chaos-sim-nodes"
  description = "Node group: 1 node per zone, preemptible, budget-optimized"

  version = "1.35"

  instance_template {
    platform_id = "standard-v2" # Intel Cascade Lake

    network_interface {
      subnet_ids = [
        yandex_vpc_subnet.a.id,
        yandex_vpc_subnet.b.id,
        yandex_vpc_subnet.d.id,
      ]
      nat                = true
      security_group_ids = [yandex_vpc_security_group.k8s_nodes.id]
    }

    resources {
      cores         = 2
      memory        = 2
      core_fraction = 20 # 20% CPU guarantee (balance: cost vs throttling risk)
    }

    boot_disk {
      type = "network-hdd"
      size = 30
    }

    scheduling_policy {
      preemptible = true # Preemptible VMs: cluster is short-lived, cost savings > risk
    }
  }

  scale_policy {
    fixed_scale {
      size = 3 # Exactly 3 nodes (1 per zone)
    }
  }

  allocation_policy {
    location {
      zone = "ru-central1-a"
    }
    location {
      zone = "ru-central1-b"
    }
    location {
      zone = "ru-central1-d"
    }
  }
}