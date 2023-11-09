data "hcp_vault_secrets_app" "statisticsbot" {
  app_name = local.name
}

resource "kubernetes_secret" "vault_auth" {
  metadata {
    name      = "default"
    namespace = kubernetes_namespace.statisticsbot.metadata.0.name
  }
  data = {
    clientID     = data.aws_ssm_parameter.vault_client_id.value
    clientSecret = data.aws_ssm_parameter.vault_client_secret.value
  }
}

resource "kubernetes_manifest" "hcp_vault_auth" {
  manifest = {
    apiVersion = "secrets.hashicorp.com/v1beta1"
    kind       = "HCPAuth"
    metadata = {
      name      = "default"
      namespace = kubernetes_namespace.statisticsbot.metadata.0.name
    }
    spec = {
      allowedNamespaces = ["*"]
      organizationID    = data.hcp_vault_secrets_app.statisticsbot.organization_id
      projectID         = data.hcp_vault_secrets_app.statisticsbot.project_id
      servicePrincipal = {
        secretRef = kubernetes_secret.vault_auth.metadata.0.name
      }
    }
  }
}

resource "kubernetes_manifest" "statisticsbot_keys" {
  manifest = {
    apiVersion = "secrets.hashicorp.com/v1beta1"
    kind       = "HCPVaultSecretsApp"
    metadata = {
      name      = local.name
      namespace = kubernetes_namespace.statisticsbot.metadata.0.name
    }
    spec = {
      appName = local.name
      destination = {
        create = true
        labels = {
          hvs = "true"
        }
        name = local.name
      }
      hcpAuthRef   = kubernetes_manifest.hcp_vault_auth.manifest.metadata.name
      refreshAfter = "1h"
    }
  }
}


resource "kubernetes_manifest" "vault_backend" {
  manifest = {
    apiVersion = "external-secrets.io/v1beta1"
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
    apiVersion = "external-secrets.io/v1beta1"
    kind       = "ExternalSecret"
    metadata = {
      name      = data.terraform_remote_state.kubernetes_cluster.outputs.vault_ecr_role.name
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
            key      = data.terraform_remote_state.kubernetes_cluster.outputs.vault_ecr_role.name
            property = ".dockerconfigjson"
          }
        }
      ]
    }
  }
}
