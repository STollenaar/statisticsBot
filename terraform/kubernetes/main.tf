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
        container {
          image = "${data.terraform_remote_state.discord_bots_cluster.outputs.discord_bots_repo.repository_url}:${local.name}-1.1.16-SNAPSHOT-4011e7e-amd64"
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
            name  = "SQS_REQUEST"
            value = data.terraform_remote_state.sqs_queues.outputs.sqs_queue.markov_user_request.url
          }
          env {
            name  = "SQS_RESPONSE"
            value = data.terraform_remote_state.sqs_queues.outputs.sqs_queue.markov_user_response.url
          }
          env {
            name  = "DATABASE_HOST"
            value = "${kubernetes_service_v1.database.metadata.0.name}:19530"
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
        container {
          name  = "sentence-transformers"
          image = "${data.aws_ecr_repository.sentence_transformers.repository_url}:latest"
          port {
            container_port = 8000
            name           = "transformer"
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

resource "kubernetes_stateful_set_v1" "database" {
  metadata {
    name      = "database"
    namespace = kubernetes_namespace.statisticsbot.metadata.0.name
    labels = {
      app = "database"
    }
  }
  spec {
    service_name = kubernetes_service_v1.database.metadata.0.name
    selector {
      match_labels = {
        app = "database"
      }
    }
    template {
      metadata {
        labels = {
          app = "database"
        }
      }
      spec {
        container {
          name    = "milvus"
          image   = "milvusdb/milvus:v2.4.15"
          command = ["milvus"]
          args    = ["run", "standalone"]
          env {
            name  = "ETCD_ENDPOINT"
            value = "localhost:2379"
          }
          env {
            name  = "MINIO_ADDRESS"
            value = "localhost:9000"
          }
          port {
            container_port = 9091
            name           = "milvus"
          }
          port {
            container_port = 19530
            name           = "milvus-api"
          }
          volume_mount {
            name       = kubernetes_persistent_volume_claim_v1.milvus.metadata.0.name
            mount_path = "/var/lib/milvus"
          }
        }
        container {
          name    = "etcd"
          image   = "quay.io/coreos/etcd:v3.5.5"
          command = ["etcd"]
          args = [
            "-advertise-client-urls=http://127.0.0.1:2379",
            "-listen-client-urls=http://0.0.0.0:2379",
            "--data-dir=/etcd"
          ]
          port {
            name           = "etcd"
            container_port = 2379
          }
          volume_mount {
            name       = kubernetes_persistent_volume_claim_v1.etcd.metadata.0.name
            mount_path = "/etcd"
          }
        }
        container {
          name    = "minio"
          image   = "minio/minio:RELEASE.2023-03-20T20-16-18Z"
          command = ["minio"]
          args = [
            "server",
            "/minio_data",
            "--console-address=:9001"
          ]
          env {
            name  = "MINIO_ACCESS_KEY"
            value = "minioadmin"
          }
          env {
            name  = "MINIO_SECRET_KEY"
            value = "minioadmin"
          }
          port {
            name           = "minio"
            container_port = 9001
          }
          port {
            name           = "minio-api"
            container_port = 9000
          }
          volume_mount {
            name       = kubernetes_persistent_volume_claim_v1.minio.metadata.0.name
            mount_path = "/minio_data"
          }
        }
        volume {
          name = kubernetes_persistent_volume_claim_v1.minio.metadata.0.name
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim_v1.minio.metadata.0.name
          }

        }
        volume {
          name = kubernetes_persistent_volume_claim_v1.etcd.metadata.0.name
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim_v1.etcd.metadata.0.name
          }
        }
        volume {
          name = kubernetes_persistent_volume_claim_v1.milvus.metadata.0.name
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim_v1.milvus.metadata.0.name
          }
        }
      }
    }
  }
}

resource "kubernetes_persistent_volume_claim_v1" "minio" {
  metadata {
    name      = "minio"
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

resource "kubernetes_persistent_volume_claim_v1" "milvus" {
  metadata {
    name      = "milvus"
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

resource "kubernetes_persistent_volume_claim_v1" "etcd" {
  metadata {
    name      = "etcd"
    namespace = kubernetes_namespace.statisticsbot.metadata.0.name
  }
  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = {
        "storage" = "2Gi"
      }
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

resource "kubernetes_service_v1" "database" {
  metadata {
    name      = "database"
    namespace = kubernetes_namespace.statisticsbot.metadata.0.name
  }
  spec {
    selector = {
      "app" = "database"
    }
    port {
      name = "milvus"
      port = 9091
    }
    port {
      name = "milvus-api"
      port = 19530
    }

    port {
      name = "etcd"
      port = 2379
    }

    port {
      name = "minio"
      port = 9001
    }
    port {
      name = "minio-api"
      port = 9000
    }
  }
}

resource "kubernetes_deployment" "attu" {
  metadata {
    name      = "attu"
    namespace = kubernetes_namespace.statisticsbot.metadata.0.name
    labels = {
      app = "attu"
    }
  }
  spec {
    selector {
      match_labels = {
        app = "attu"
      }
    }
    template {
      metadata {
        labels = {
          app = "attu"
        }
      }
      spec {
        container {
          image = "zilliz/attu:latest"
          name  = "attu"
          env {
            name  = "MILVUS_URL"
            value = "http://${kubernetes_service_v1.database.metadata.0.name}:19530"
          }
        }
      }
    }
  }
}


# resource "kubernetes_ingress_v1" "ingress" {
#   metadata {
#     name      = "statisticsbot"
#     namespace = kubernetes_namespace.statisticsbot.metadata.0.name
#   }
#   spec {
#     ingress_class_name = "tailscale"
#     rule {
#       http {
#         path {
#           path      = "/"
#           path_type = "Prefix"
#           backend {
#             service {
#               name = kubernetes_service_v1.statisticsbot.metadata.0.name
#               port {
#                 number = 80
#               }
#             }
#           }
#         }
#       }
#     }
#   }
# }
