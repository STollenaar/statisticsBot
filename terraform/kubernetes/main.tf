locals {
  name = "statisticsbot"
}

resource "kubernetes_namespace" "statisticsbot" {
  metadata {
    name = local.name
  }
}

resource "kubernetes_deployment" "statisticsbot" {
  metadata {
    name      = "statisticsbot"
    namespace = kubernetes_namespace.statisticsbot.metadata.0.name
    labels = {
      app = local.name
    }
  }
  spec {
    strategy {
      type = "Recreate"
    }
    selector {
      match_labels = {
        app = local.name
      }
    }
    template {

      metadata {
        annotations = {
          "vault.hashicorp.com/agent-inject" = "true"
          "vault.hashicorp.com/role"         = "internal-app"
          "vault.hashicorp.com/aws-role"     = data.terraform_remote_state.iam_role.outputs.iam.statisticsbot_role.name
          "cache.spicedelver.me/cmtemplate"  = "vault-aws-agent"
        }
        labels = {
          app = local.name
        }
      }
      spec {

        image_pull_secrets {
          name = kubernetes_manifest.external_secret.manifest.spec.target.name
        }
        container {
          image = "${data.terraform_remote_state.discord_bots_cluster.outputs.discord_bots_repo.repository_url}:${local.name}-1.1.16-SNAPSHOT-34de555"
          name  = local.name
          env {
            name  = "AWS_REGION"
            value = data.aws_region.current.name
          }
          env {
            name  = "AWS_SHARED_CREDENTIALS_FILE"
            value = "/vault/secrets/aws/credentials"
          }
          env {
            name  = "AWS_PARAMETER_NAME"
            value = "/discord_tokens/${local.name}"
          }
          env {
            name  = "DUCKDB_PATH"
            value = "/duckdb"
          }
          env {
            name  = "OLLAMA_URL"
            value = "ollama.ollama.svc.cluster.local:11434"
          }
        #   env {
        #     name  = "OLLAMA_URL"
        #     value = "ollama.danielpower.ca"
        #   }
        #   env {
        #     name  = "OLLAMA_AUTH_TYPE"
        #     value = "basic"
        #   }
          env {
            name  = "AWS_OLLAMA_AUTH_USERNAME"
            value = "/ollama/dan_username"
          }
          env {
            name  = "AWS_OLLAMA_AUTH_PASSWORD"
            value = "/ollama/dan_password"
          }
          port {
            container_port = 8080
            name           = "router"
          }
          volume_mount {
            name       = kubernetes_persistent_volume_claim_v1.duckdb.metadata.0.name
            mount_path = "/duckdb"
          }

        }
        volume {
          name = kubernetes_persistent_volume_claim_v1.duckdb.metadata.0.name
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim_v1.duckdb.metadata.0.name
          }
        }
      }
    }
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
