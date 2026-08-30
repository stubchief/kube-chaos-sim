# Security group for Kubernetes Master (Control Plane)
resource "yandex_vpc_security_group" "k8s_master" {
  name        = "chaos-sim-k8s-master-sg"
  description = "Security group for Managed Kubernetes master control plane"
  network_id  = yandex_vpc_network.chaos_sim.id

  # External access to Kubernetes API (port 443) for kubectl and CI/CD
  ingress {
    protocol       = "TCP"
    description    = "Kubernetes API server external access"
    port           = 443
    v4_cidr_blocks = ["0.0.0.0/0"]
  }

  # Allow Master to communicate with Worker Nodes (SG-to-SG)
  ingress {
    protocol          = "ANY"
    description       = "Allow traffic from worker nodes"
    predefined_target = "self_security_group"
  }

  # Allow communication across all internal subnets
  ingress {
    protocol       = "ANY"
    description    = "Allow internal subnets communication"
    v4_cidr_blocks = ["10.10.0.0/16", "10.11.0.0/16", "10.12.0.0/16"]
  }

  # Allow health checks from Yandex Cloud Load Balancers
  ingress {
    protocol          = "TCP"
    description       = "Health checks from YC Load Balancer"
    predefined_target = "loadbalancer_healthchecks"
    from_port         = 0
    to_port           = 65535
  }

  # Allow all outbound traffic from Master
  egress {
    protocol       = "ANY"
    description    = "Allow all outbound traffic"
    v4_cidr_blocks = ["0.0.0.0/0"]
    from_port      = 0
    to_port        = 65535
  }
}

# Security group for Worker Nodes and Application Services
resource "yandex_vpc_security_group" "k8s_nodes" {
  name        = "chaos-sim-k8s-nodes-sg"
  description = "Security group for Kubernetes worker nodes and pods"
  network_id  = yandex_vpc_network.chaos_sim.id

  # Node-to-Node and Pod-to-Pod communication
  ingress {
    protocol          = "ANY"
    description       = "Allow internal traffic within the security group"
    predefined_target = "self_security_group"
  }

  # CRITICAL FOR YC LB: Health checks directly to Worker Nodes
  ingress {
    protocol          = "TCP"
    description       = "Health checks from YC Load Balancer"
    predefined_target = "loadbalancer_healthchecks"
    from_port         = 0
    to_port           = 65535
  }

  # Communication across all internal subnets (Master <-> Nodes, Pods <-> Pods)
  ingress {
    protocol       = "ANY"
    description    = "Allow internal subnets communication"
    v4_cidr_blocks = ["10.10.0.0/16", "10.11.0.0/16", "10.12.0.0/16"]
  }

  # Allow NodePort service connections (Load Balancer routes traffic here)
  ingress {
    protocol       = "TCP"
    description    = "NodePort services range"
    v4_cidr_blocks = ["0.0.0.0/0"]
    from_port      = 30000
    to_port        = 32767
  }

  # Allow direct access to backend port if accessed directly
  ingress {
    protocol       = "TCP"
    description    = "Chaos sim backend port"
    port           = 8080
    v4_cidr_blocks = ["0.0.0.0/0"]
  }

  # Allow outbound traffic
  egress {
    protocol       = "ANY"
    description    = "Allow all outbound traffic"
    v4_cidr_blocks = ["0.0.0.0/0"]
    from_port      = 0
    to_port        = 65535
  }
}