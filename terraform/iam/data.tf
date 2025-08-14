data "terraform_remote_state" "kubernetes_cluster" {
  backend = "s3"
  config = {
    region = "ca-central-1"
    bucket = "stollenaar-terraform-states"
    key    = "infrastructure/kubernetes/terraform.tfstate"
  }
}

data "terraform_remote_state" "aws_iam" {
  backend = "s3"
  config = {
    region = "ca-central-1"
    bucket = "stollenaar-terraform-states"
    key    = "infrastructure/aws/iam/terraform.tfstate"
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
}

data "aws_iam_policy_document" "assume_policy_document" {
  statement {
    effect = "Allow"
    principals {
      identifiers = [data.terraform_remote_state.aws_iam.outputs.iam_users.vault_user.arn]
      type        = "AWS"
    }
    actions = ["sts:AssumeRole"]
  }
}
