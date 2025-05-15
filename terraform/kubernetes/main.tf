locals {
  name = "statisticsbot"
}

resource "kubernetes_namespace" "statisticsbot" {
  metadata {
    name = local.name
  }
}

resource "kubernetes_service_v1" "statisticsbot" {
  metadata {
    name      = "statisticsbot"
    namespace = kubernetes_namespace.statisticsbot.metadata.0.name
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

resource "kubernetes_persistent_volume_claim_v1" "duckdb" {
  metadata {
    name      = "duckdb"
    namespace = kubernetes_namespace.statisticsbot.metadata.0.name
  }
  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = {
        "storage" = "3Gi"
      }
    }
  }
}
