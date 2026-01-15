locals {
  name = "statisticsbot"
}

resource "kubernetes_service_v1" "statisticsbot" {
  metadata {
    name      = "statisticsbot"
    namespace = data.terraform_remote_state.kubernetes_cluster.outputs.discordbots.namespace.metadata.0.name
  }
  spec {
    selector = {
      "app" = local.name
    }
    port {
      name        = "router"
      target_port = 8080
      port        = 80
    }
  }
}

resource "kubernetes_persistent_volume_claim_v1" "statisticsbot" {
  metadata {
    name      = "statisticsbot"
    namespace = data.terraform_remote_state.kubernetes_cluster.outputs.discordbots.namespace.metadata.0.name
  }
  spec {
    access_modes = ["ReadWriteOnce"]
    volume_name = "pvc-9f813513-1e6d-4761-a55e-529769e1d3bc"
    resources {
      requests = {
        "storage" = "3Gi"
      }
    }
  }
}
