locals {
  name  = "statisticsbot"
  image = var.docker_image != null ? var.docker_image : "${data.terraform_remote_state.ecr_repo.outputs.discord_bots_repo.repository_url}:${local.name}-1.1.16-SNAPSHOT-ab8ea54"
}

resource "kubernetes_deployment" "statisticsbot" {
  metadata {
    name      = "statisticsbot"
    namespace = data.terraform_remote_state.kubernetes_cluster.outputs.discordbots.namespace.metadata.0.name
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
          name = data.terraform_remote_state.kubernetes_cluster.outputs.discordbots.secret_name
        }
        container {
          image = local.image
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
          env {
            name  = "OLLAMA_MODEL"
            value = "llama3.2:3b"
          }
        #   env {
        #     name  = "OLLAMA_URL"
        #     value = "llm.ops.bootleg.technology/chat/completions"
        #   }
        #   env {
        #     name  = "OLLAMA_AUTH_TYPE"
        #     value = "api_key"
        #   }
        #   env {
        #     name  = "OLLAMA_MODEL"
        #     value = "openai/gpt-oss-120b"
        #   }
        #   env {
        #     name  = "AWS_OLLAMA_API_KEY"
        #     value = "/ollama/nint_api_key"
        #   }
          env {
            name  = "ADMIN_USER_ID"
            value = "134337759446958081"
          }

          port {
            container_port = 8080
            name           = "router"
          }
          volume_mount {
            name       = data.terraform_remote_state.kubernetes.outputs.persistent_volume_claim.metadata.0.name
            mount_path = "/duckdb"
          }

        }
        volume {
          name = data.terraform_remote_state.kubernetes.outputs.persistent_volume_claim.metadata.0.name
          persistent_volume_claim {
            claim_name = data.terraform_remote_state.kubernetes.outputs.persistent_volume_claim.metadata.0.name
          }
        }
      }
    }
  }
}
