data "terraform_remote_state" "vault_setup" {
  backend = "s3"
  config = {
    region = "ca-central-1"
    bucket = "stollenaar-terraform-states"
    key    = "infrastructure/vault-setup/terraform.tfstate"
  }
}

data "terraform_remote_state" "iam_role" {
  backend = "s3"
  config = {
    region = "ca-central-1"
    bucket = "stollenaar-terraform-states"
    key    = "discordbots/statisticsbot/iam/terraform.tfstate"
  }
}

data "hcp_vault_secrets_secret" "vault_root" {
  app_name    = "proxmox"
  secret_name = "root"
}

data "aws_ssm_parameter" "vault_client_id" {
  name = "/vault/serviceprincipals/talos/client_id"
}

data "aws_ssm_parameter" "vault_client_secret" {
  name = "/vault/serviceprincipals/talos/client_secret"
}
