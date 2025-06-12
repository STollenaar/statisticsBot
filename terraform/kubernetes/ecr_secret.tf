resource "kubernetes_manifest" "vault_backend" {
  manifest = {
    apiVersion = "external-secrets.io/v1"
    kind       = "SecretStore"
    metadata = {
      name      = "vault-backend"
      namespace = kubernetes_namespace.statisticsbot.metadata.0.name
    }
    spec = {
      provider = {
        vault = {
          server  = "http://vault.${data.terraform_remote_state.kubernetes_cluster.outputs.vault_namespace.metadata.0.name}.svc.cluster.local:8200"
          path    = "secret"
          version = "v2"
          auth = {
            kubernetes = {
              mountPath = "kubernetes"
              role      = "external-secrets"
            }
          }
        }
      }
    }
  }
}

resource "kubernetes_manifest" "external_secret" {
  manifest = {
    apiVersion = "external-secrets.io/v1"
    kind       = "ExternalSecret"
    metadata = {
      name      = "ecr-auth"
      namespace = kubernetes_namespace.statisticsbot.metadata.0.name
    }
    spec = {
      secretStoreRef = {
        name = kubernetes_manifest.vault_backend.manifest.metadata.name
        kind = kubernetes_manifest.vault_backend.manifest.kind
      }
      target = {
        name = "regcred"
        template = {
          type          = "kubernetes.io/dockerconfigjson"
          mergePolicy   = "Replace"
          engineVersion = "v2"
        }
      }
      data = [
        {
          secretKey = ".dockerconfigjson"
          remoteRef = {
            key      = "ecr-auth"
            property = ".dockerconfigjson"
          }
        }
      ]
    }
  }
}
