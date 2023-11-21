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
    selector {
      match_labels = {
        app = local.name
      }
    }
    template {
      metadata {
        annotations = {
          "vault.hashicorp.com/agent-inject"        = "true"
          "vault.hashicorp.com/agent-internal-role" = "internal-app"
          "vault.hashicorp.com/agent-aws-role"      = aws_iam_role.statisticsbot_role.name
          "cache.spices.dev/cmtemplate"             = "vault-aws-agent"
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
          image = "${data.terraform_remote_state.discord_bots_cluster.outputs.discord_bots_repo.repository_url}:${local.name}-1.1.11-SNAPSHOT-81b0c20-amd64"
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
            name  = "MONGO_HOST_PARAMETER"
            value = "/mongodb/statsuser/database_host"
          }
          env {
            name  = "MONGO_PASSWORD_PARAMETER"
            value = "/mongodb/statsuser/password"
          }
          env {
            name  = "MONGO_USERNAME_PARAMETER"
            value = "/mongodb/statsuser/username"
          }
          env {
            name  = "SQS_REQUEST"
            value = data.terraform_remote_state.sqs_queues.outputs.sqs_queue.markov_user_request.url
          }
          env {
            name  = "SQS_RESPONSE"
            value = data.terraform_remote_state.sqs_queues.outputs.sqs_queue.markov_user_response.url
          }
        }
      }
    }
  }
}
