locals {
  name            = "statisticsbot"
  kubeconfig_file = get_env("KUBECONFIG")

  # Automatically load provider variables
  provider_vars = read_terragrunt_config("${get_original_terragrunt_dir()}/provider.hcl")

  # Extract the variables we need for easy access
  providers = local.provider_vars.locals.providers
}

remote_state {
  backend = "s3"
  generate = {
    path      = "grunt_backend.tf"
    if_exists = "overwrite_terragrunt"
  }
  config = {
    bucket = "stollenaar-terraform-states"

    key     = "discordbots/${local.name}/${path_relative_to_include()}/terraform.tfstate"
    region  = "ca-central-1"
    encrypt = true
  }
}

generate "provider" {
  path      = "grunt_providers.tf"
  if_exists = "overwrite"
  contents  = <<EOF
        %{if contains(local.providers, "kubernetes")}
        provider "kubernetes" {
            config_path = "${local.kubeconfig_file}"
        }
        %{endif}
        %{if contains(local.providers, "helm")}
        provider "helm" {
            kubernetes {
                config_path = "${local.kubeconfig_file}"
            }
        }
        %{endif}
        %{if contains(local.providers, "vault")}
        provider "vault" {
            token   = data.hcp_vault_secrets_secret.vault_root.secret_value
            address = "http://localhost:8200"
        }
        %{endif}
    EOF
}

generate "versions" {
  path      = "grunt_versions.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF
    terraform {
        required_providers {
            %{if contains(local.providers, "aws")}
            aws = {
            source  = "hashicorp/aws"
            version = "~> 5.24.0"
            }
            %{endif}
            helm = {
            version = "~> 2.10.1"
            source  = "hashicorp/helm"
            }
            %{if contains(local.providers, "kubernetes")}
            kubernetes = {
            version = "~> 2.23.0"
            source  = "hashicorp/kubernetes"
            }
            %{endif}
            %{if contains(local.providers, "vault")}
            vault = {
                source  = "hashicorp/vault"
                version = "~> 3.21.0"
            }
            %{endif}
        }
        required_version = ">= 1.2.2"
    }
    EOF
}
