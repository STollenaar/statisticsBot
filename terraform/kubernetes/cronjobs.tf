
locals {
  host            = "http://${kubernetes_service_v1.statisticsbot.metadata.0.name}.${kubernetes_namespace.statisticsbot.metadata.0.name}.svc.cluster.local"
  image           = "curlimages/curl:7.85.0"
  history_success = 3
  history_fail    = 1
  backoff_limit   = 2
}

# DELETE /fixMessages; Removes invalid entries from the database
resource "kubernetes_cron_job_v1" "delete_fix_messages" {
  metadata {
    name      = "delete-fix-messages"
    namespace = kubernetes_namespace.statisticsbot.metadata.0.name
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
          metadata {}
          spec {
            restart_policy = "OnFailure"

            container {
              name  = "curl-delete"
              image = local.image
              args  = ["-X", "DELETE", "${local.host}/fixMessages"]
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
    namespace = kubernetes_namespace.statisticsbot.metadata.0.name
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
          metadata {}
          spec {
            restart_policy = "OnFailure"

            container {
              name  = "curl-put-messages"
              image = local.image
              args  = ["-X", "PUT", "${local.host}/fixMessages"]
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
    namespace = kubernetes_namespace.statisticsbot.metadata.0.name
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
          metadata {}
          spec {
            restart_policy = "OnFailure"

            container {
              name  = "curl-put-emojis"
              image = local.image
              args  = ["-X", "PUT", "${local.host}/fixEmojis"]
            }
          }
        }
      }
    }
  }
}
