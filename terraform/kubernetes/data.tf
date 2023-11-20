data "terraform_remote_state" "discord_bots_cluster" {
  backend = "s3"
  config = {
    region = "ca-central-1"
    bucket = "stollenaar-terraform-states"
    key    = "infrastructure/terraform.tfstate"
  }
}

data "terraform_remote_state" "vault_setup" {
  backend = "s3"
  config = {
    region = "ca-central-1"
    bucket = "stollenaar-terraform-states"
    key    = "infrastructure/vault-setup/terraform.tfstate"
  }
}

data "terraform_remote_state" "kubernetes_cluster" {
  backend = "s3"
  config = {
    region = "ca-central-1"
    bucket = "stollenaar-terraform-states"
    key    = "infrastructure/kubernetes/terraform.tfstate"
  }
}

data "terraform_remote_state" "sqs_queues" {
  backend = "s3"
  config = {
    region = "ca-central-1"
    bucket = "stollenaar-terraform-states"
    key    = "discordbots/statisticsbot/sqs/terraform.tfstate"
  }
}

data "aws_iam_policy_document" "ssm_access_role_policy_document" {
  statement {
    sid    = "KMSDecryption"
    effect = "Allow"
    actions = [
      "kms:ListKeys",
      "kms:GetPublicKey",
      "kms:DescribeKey",
      "kms:Decrypt",
    ]
    resources = [
      "*"
    ]
  }
  statement {
    sid    = "SSMAccess"
    effect = "Allow"
    actions = [
      "ssm:GetParametersByPath",
      "ssm:GetParameters",
      "ssm:GetParameter",
      "ssm:DescribeParameters",
    ]
    resources = ["*"]
  }
  source_policy_documents = [
    data.aws_iam_policy_document.sqs_role_policy_document.json,
  ]
}

# IAM policy document for the container to access the sqs queue
data "aws_iam_policy_document" "sqs_role_policy_document" {
  statement {
    sid    = "SQSSendMessage"
    effect = "Allow"
    actions = [
      "sqs:DeleteMessage",
      "sqs:ReceiveMessage",
      "sqs:SendMessage",
    ]
    resources = [
      data.terraform_remote_state.sqs_queues.outputs.sqs_queue.markov_user_request.arn,
      data.terraform_remote_state.sqs_queues.outputs.sqs_queue.markov_user_response.arn,
    ]
  }
}

data "aws_iam_policy_document" "assume_policy_document" {
  statement {
    effect = "Allow"
    principals {
      identifiers = [data.terraform_remote_state.kubernetes_cluster.outputs.vault_user.arn]
      type        = "AWS"
    }
    actions = ["sts:AssumeRole"]
  }
}

data "aws_region" "current" {}

data "aws_caller_identity" "current" {}

data "aws_ssm_parameter" "vault_client_id" {
  name = "/vault/serviceprincipals/talos/client_id"
}

data "aws_ssm_parameter" "vault_client_secret" {
  name = "/vault/serviceprincipals/talos/client_secret"
}

data "hcp_vault_secrets_secret" "vault_root" {
  app_name    = "proxmox"
  secret_name = "root"
}
