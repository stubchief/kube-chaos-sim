# VPC network for the cluster
resource "yandex_vpc_network" "chaos_sim" {
  name = "chaos-sim-network"
}

# Subnet in zone ru-central1-a
resource "yandex_vpc_subnet" "a" {
  name           = "chaos-sim-subnet-a"
  zone           = "ru-central1-a"
  network_id     = yandex_vpc_network.chaos_sim.id
  v4_cidr_blocks = ["10.10.0.0/16"]
}

# Subnet in zone ru-central1-b
resource "yandex_vpc_subnet" "b" {
  name           = "chaos-sim-subnet-b"
  zone           = "ru-central1-b"
  network_id     = yandex_vpc_network.chaos_sim.id
  v4_cidr_blocks = ["10.11.0.0/16"]
}

# Subnet in zone ru-central1-d
resource "yandex_vpc_subnet" "d" {
  name           = "chaos-sim-subnet-d"
  zone           = "ru-central1-d"
  network_id     = yandex_vpc_network.chaos_sim.id
  v4_cidr_blocks = ["10.12.0.0/16"]
}