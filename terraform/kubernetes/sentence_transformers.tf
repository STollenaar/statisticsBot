
resource "kubernetes_deployment" "sentence_transformers" {
  metadata {
    name      = "sentence-transformers"
    namespace = kubernetes_namespace.statisticsbot.metadata.0.name
    labels = {
      app = "sentence-transformers"
    }
  }
  spec {
    selector {
      match_labels = {
        app = "sentence-transformers"
      }
    }
    template {

      metadata {
        annotations = {
          "vault.hashicorp.com/agent-inject" = "false"
        }
        labels = {
          app = "sentence-transformers"
        }
      }
      spec {
        affinity {
          node_affinity {
            required_during_scheduling_ignored_during_execution {
              node_selector_term {
                match_expressions {
                  key      = "node-role.kubernetes.io/worker"
                  operator = "In"
                  values = [
                    "hard-worker"
                  ]
                }
              }
            }
          }
        }

        image_pull_secrets {
          name = kubernetes_manifest.external_secret.manifest.spec.target.name
        }
        
        container {
          name              = "sentence-transformers"
          image             = "${data.aws_ecr_repository.sentence_transformers.repository_url}:0.0.9"
          image_pull_policy = "IfNotPresent"
          port {
            container_port = 8000
            name           = "transformer"
          }
          volume_mount {
            name       = kubernetes_persistent_volume_claim_v1.sentence_cache.metadata.0.name
            mount_path = "/root/.cache"
          }
        }
        volume {
          name = kubernetes_persistent_volume_claim_v1.sentence_cache.metadata.0.name
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim_v1.sentence_cache.metadata.0.name
          }
        }
      }
    }
  }
}

resource "kubernetes_service_v1" "sentence_transformers" {
  metadata {
    name      = "sentence-transformers"
    namespace = kubernetes_namespace.statisticsbot.metadata.0.name
  }
  spec {
    selector = {
      "app" = "sentence-transformers"
    }
    port {
      name        = "router"
      target_port = 8000
      port        = 8000
    }
  }
}

resource "kubernetes_persistent_volume_claim_v1" "sentence_cache" {
  metadata {
    name      = "sentence-transformers-cache"
    namespace = kubernetes_namespace.statisticsbot.metadata.0.name
  }
  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = {
        "storage" = "10Gi"
      }
    }
  }
}
