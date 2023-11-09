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
    template {
      metadata {
        labels = {
          app = local.name
        }
      }
      spec {
        image_pull_secrets {
          name = kubernetes_manifest.external_secret.manifest.spec.target.name
        }
        container {
          image = "${data.terraform_remote_state.discord_bots_cluster.outputs.discord_bots_repo.repository_url}:${local.name}-SNAPSHOT-6c26712-amd4"
          name  = local.name
          env {
            name  = "AWS_REGION"
            value = data.aws_region.current.name
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
