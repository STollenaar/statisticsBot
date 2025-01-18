
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

        image_pull_secrets {
          name = kubernetes_manifest.external_secret.manifest.spec.target.name
        }
        container {
          name              = "sentence-transformers"
          image             = "${data.aws_ecr_repository.sentence_transformers.repository_url}:0.0.3"
          image_pull_policy = "IfNotPresent"
          port {
            container_port = 8000
            name           = "transformer"
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
