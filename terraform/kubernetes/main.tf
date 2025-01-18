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
          "vault.hashicorp.com/aws-role"     = aws_iam_role.statisticsbot_role.name
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
        init_container {
          name  = "wait-for-pod"
          image = "busybox"
          env {
            name  = "SENTENCE_TRANSFORMERS"
            value = "${kubernetes_service_v1.sentence_transformers.metadata.0.name}.${kubernetes_namespace.statisticsbot.metadata.0.name}:8000"
          }
          command = [
            "sh", "-c",
            "until wget -q --spider http://$SENTENCE_TRANSFORMERS/healthz; do echo 'Waiting for sentence-transformers /healthz endpoint...'; sleep 5; done"
          ]
        }
        container {
          image = "${data.terraform_remote_state.discord_bots_cluster.outputs.discord_bots_repo.repository_url}:${local.name}-1.1.16-SNAPSHOT-8329a2b"
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
            name  = "DATABASE_HOST"
            value = "${kubernetes_service_v1.database.metadata.0.name}:19530"
          }
          env {
            name  = "DUCKDB_PATH"
            value = "/duckdb"
          }
          env {
            name  = "SENTENCE_TRANSFORMERS"
            value = "${kubernetes_service_v1.sentence_transformers.metadata.0.name}.${kubernetes_namespace.statisticsbot.metadata.0.name}:8000"
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
