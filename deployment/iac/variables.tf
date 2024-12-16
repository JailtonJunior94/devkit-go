variable "do_token" {
  default = "dop_v1_0d7b36dbbffaf37e06370a7f1fc72b597a3ddc70ed602d24a816aad49435e12b"
}

variable "region" {
  default = "nyc1"
}

variable "cluster_version" {
  default = "1.31.1-do.5"
}

variable "cluster_name" {
  default = "estudos-k8s"
}

variable "node_name" {
  default = "estudos-node-pool"
}

variable "node_size" {
  default = "s-4vcpu-8gb"
}

variable "node_count" {
  default = 1
}
