
locals {
  host            = "http://${kubernetes_service_v1.statisticsbot.metadata.0.name}.${data.terraform_remote_state.kubernetes_cluster.outputs.discordbots.namespace.metadata.0.name}.svc.cluster.local"
  image           = "curlimages/curl:7.85.0"
  history_success = 3
  history_fail    = 1
  backoff_limit   = 2

  # The internal API requires a shared token. It lives in SSM, and these pods
  # reach it the same way the bot does: the Vault agent injects short-lived AWS
  # credentials for the statisticsbot role, and something with an AWS client
  # exchanges them for the parameter. Nothing is stored in a Kubernetes Secret
  # or in terraform state as a result.
  vault_annotations = {
    "vault.hashicorp.com/agent-inject" = "true"
    "vault.hashicorp.com/role"         = "internal-app"
    "vault.hashicorp.com/aws-role"     = data.terraform_remote_state.iam_role.outputs.iam.statisticsbot_role.name
    "cache.spicedelver.me/cmtemplate"  = "vault-aws-agent"
  }

  aws_image = "amazon/aws-cli:2.17.0"

  # Written as a curl config file rather than exported as an env var, so the
  # token never appears in the container's args or environment — and so the curl
  # containers below keep their plain argument lists.
  fetch_token_script = <<-EOT
    set -eu
    TOKEN="$(aws ssm get-parameter --name '/statisticsbot/auth_token' --with-decryption --query Parameter.Value --output text)"
    printf 'header = "X-Auth-Token: %s"\n' "$TOKEN" > /token/curlrc
  EOT
}

# DELETE /fixMessages; Removes invalid entries from the database
resource "kubernetes_cron_job_v1" "delete_fix_messages" {
  metadata {
    name      = "delete-fix-messages"
    namespace = data.terraform_remote_state.kubernetes_cluster.outputs.discordbots.namespace.metadata.0.name
  }
  spec {
    schedule                      = "0 * * * 6" # every hour at minute 5
    suspend                       = true
    successful_jobs_history_limit = local.history_success
    failed_jobs_history_limit     = local.history_fail

    job_template {
      metadata {}
      spec {
        backoff_limit = local.backoff_limit

        template {
          metadata {
            annotations = local.vault_annotations
          }
          spec {
            restart_policy = "OnFailure"

            volume {
              name = "token"
              empty_dir {}
            }

            # Runs after the injector's own init container, so the credentials
            # it renders are already on disk.
            init_container {
              name    = "fetch-token"
              image   = local.aws_image
              command = ["/bin/sh", "-c"]
              args    = [local.fetch_token_script]
              env {
                name  = "AWS_SHARED_CREDENTIALS_FILE"
                value = "/vault/secrets/aws/credentials"
              }
              env {
                name  = "AWS_REGION"
                value = data.aws_region.current.name
              }
              volume_mount {
                name       = "token"
                mount_path = "/token"
              }
            }

            container {
              name  = "curl-delete"
              image = local.image
              args  = ["-K", "/token/curlrc", "-X", "DELETE", "${local.host}/fixMessages"]
              volume_mount {
                name       = "token"
                mount_path = "/token"
              }
            }
          }
        }
      }
    }
  }
}

# PUT /fixMessages; Adds missing messages
resource "kubernetes_cron_job_v1" "put_fix_messages" {
  metadata {
    name      = "put-fix-messages"
    namespace = data.terraform_remote_state.kubernetes_cluster.outputs.discordbots.namespace.metadata.0.name
  }
  spec {
    schedule                      = "2 * * * 6" # every hour at minute 10
    suspend                       = true
    successful_jobs_history_limit = local.history_success
    failed_jobs_history_limit     = local.history_fail

    job_template {
      metadata {}
      spec {
        backoff_limit = local.backoff_limit

        template {
          metadata {
            annotations = local.vault_annotations
          }
          spec {
            restart_policy = "OnFailure"

            volume {
              name = "token"
              empty_dir {}
            }

            # Runs after the injector's own init container, so the credentials
            # it renders are already on disk.
            init_container {
              name    = "fetch-token"
              image   = local.aws_image
              command = ["/bin/sh", "-c"]
              args    = [local.fetch_token_script]
              env {
                name  = "AWS_SHARED_CREDENTIALS_FILE"
                value = "/vault/secrets/aws/credentials"
              }
              env {
                name  = "AWS_REGION"
                value = data.aws_region.current.name
              }
              volume_mount {
                name       = "token"
                mount_path = "/token"
              }
            }

            container {
              name  = "curl-put-messages"
              image = local.image
              args  = ["-K", "/token/curlrc", "-X", "PUT", "${local.host}/fixMessages"]
              volume_mount {
                name       = "token"
                mount_path = "/token"
              }
            }
          }
        }
      }
    }
  }
}

# PUT /fixEmojis; add missing guild emojis
resource "kubernetes_cron_job_v1" "put_fix_emojis" {
  metadata {
    name      = "put-fix-emojis"
    namespace = data.terraform_remote_state.kubernetes_cluster.outputs.discordbots.namespace.metadata.0.name
  }
  spec {
    schedule                      = "2 * * * 6" # every hour at minute 15
    suspend                       = true
    successful_jobs_history_limit = local.history_success
    failed_jobs_history_limit     = local.history_fail

    job_template {
      metadata {}
      spec {
        backoff_limit = local.backoff_limit

        template {
          metadata {
            annotations = local.vault_annotations
          }
          spec {
            restart_policy = "OnFailure"

            volume {
              name = "token"
              empty_dir {}
            }

            # Runs after the injector's own init container, so the credentials
            # it renders are already on disk.
            init_container {
              name    = "fetch-token"
              image   = local.aws_image
              command = ["/bin/sh", "-c"]
              args    = [local.fetch_token_script]
              env {
                name  = "AWS_SHARED_CREDENTIALS_FILE"
                value = "/vault/secrets/aws/credentials"
              }
              env {
                name  = "AWS_REGION"
                value = data.aws_region.current.name
              }
              volume_mount {
                name       = "token"
                mount_path = "/token"
              }
            }

            container {
              name  = "curl-put-emojis"
              image = local.image
              args  = ["-K", "/token/curlrc", "-X", "PUT", "${local.host}/fixEmojis"]
              volume_mount {
                name       = "token"
                mount_path = "/token"
              }
            }
          }
        }
      }
    }
  }
}

# POST /backup; export the DuckDB database and upload it to Backblaze
resource "kubernetes_cron_job_v1" "backup" {
  metadata {
    name      = "backup"
    namespace = data.terraform_remote_state.kubernetes_cluster.outputs.discordbots.namespace.metadata.0.name
  }
  spec {
    schedule                      = "0 3 * * *" # daily at 03:00
    concurrency_policy            = "Forbid"
    successful_jobs_history_limit = local.history_success
    failed_jobs_history_limit     = local.history_fail

    job_template {
      metadata {}
      spec {
        backoff_limit           = local.backoff_limit
        active_deadline_seconds = 3600

        template {
          metadata {
            annotations = local.vault_annotations
          }
          spec {
            restart_policy = "OnFailure"

            volume {
              name = "token"
              empty_dir {}
            }

            # Runs after the injector's own init container, so the credentials
            # it renders are already on disk.
            init_container {
              name    = "fetch-token"
              image   = local.aws_image
              command = ["/bin/sh", "-c"]
              args    = [local.fetch_token_script]
              env {
                name  = "AWS_SHARED_CREDENTIALS_FILE"
                value = "/vault/secrets/aws/credentials"
              }
              env {
                name  = "AWS_REGION"
                value = data.aws_region.current.name
              }
              volume_mount {
                name       = "token"
                mount_path = "/token"
              }
            }

            container {
              name  = "curl-backup"
              image = local.image
              # --fail is what makes a failed backup fail the job instead of
              # silently recording a 500 response as a success.
              args = ["-sS", "--fail", "-K", "/token/curlrc", "-X", "POST", "--max-time", "3600", "${local.host}/backup"]
              volume_mount {
                name       = "token"
                mount_path = "/token"
              }
            }
          }
        }
      }
    }
  }
}

# PUT /fixEmbeddings; generate embeddings for messages missing them
resource "kubernetes_cron_job_v1" "put_fix_embeddings" {
  metadata {
    name      = "put-fix-embeddings"
    namespace = data.terraform_remote_state.kubernetes_cluster.outputs.discordbots.namespace.metadata.0.name
  }
  spec {
    schedule                      = "2 * * * 6" # every hour at minute 20
    suspend                       = true
    successful_jobs_history_limit = local.history_success
    failed_jobs_history_limit     = local.history_fail

    job_template {
      metadata {}
      spec {
        backoff_limit = local.backoff_limit
        # Backfilling embeds every missing message synchronously within the
        # request, so give it plenty of time before it is killed.
        active_deadline_seconds = 7200

        template {
          metadata {
            annotations = local.vault_annotations
          }
          spec {
            restart_policy = "OnFailure"

            volume {
              name = "token"
              empty_dir {}
            }

            # Runs after the injector's own init container, so the credentials
            # it renders are already on disk.
            init_container {
              name    = "fetch-token"
              image   = local.aws_image
              command = ["/bin/sh", "-c"]
              args    = [local.fetch_token_script]
              env {
                name  = "AWS_SHARED_CREDENTIALS_FILE"
                value = "/vault/secrets/aws/credentials"
              }
              env {
                name  = "AWS_REGION"
                value = data.aws_region.current.name
              }
              volume_mount {
                name       = "token"
                mount_path = "/token"
              }
            }

            container {
              name  = "curl-put-embeddings"
              image = local.image
              args  = ["-K", "/token/curlrc", "-X", "PUT", "--max-time", "7200", "${local.host}/fixEmbeddings"]
              volume_mount {
                name       = "token"
                mount_path = "/token"
              }
            }
          }
        }
      }
    }
  }
}
