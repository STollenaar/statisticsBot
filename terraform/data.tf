data "terraform_remote_state" "discord_bots_cluster" {
  backend = "s3"
  config = {
    profile = local.used_profile.name
    region  = "ca-central-1"
    bucket  = "stollenaar-terraform-states"
    key     = "infrastructure/terraform.tfstate"
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
    data.aws_iam_policy_document.ecr_role_policy_document.json,
    data.aws_iam_policy_document.sqs_role_policy_document.json,
    data.aws_iam_policy_document.cloudwatch_role_policy_document.json
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
      aws_sqs_queue.markov_user_request.arn,
      aws_sqs_queue.markov_user_response.arn,
    ]
  }
}

# IAM policy document for the container to access ECR
data "aws_iam_policy_document" "ecr_role_policy_document" {
  statement {
    sid    = "ECRAccess"
    effect = "Allow"
    actions = [
      "ec2:DescribeTags",
      "ecs:CreateCluster",
      "ecs:DeregisterContainerInstance",
      "ecs:DiscoverPollEndpoint",
      "ecs:Poll",
      "ecs:RegisterContainerInstance",
      "ecs:StartTelemetrySession",
      "ecs:UpdateContainerInstancesState",
      "ecs:Submit*",
      "ecr:GetAuthorizationToken",
      "ecr:BatchCheckLayerAvailability",
      "ecr:GetDownloadUrlForLayer",
      "ecr:BatchGetImage",
      "logs:CreateLogStream",
      "logs:PutLogEvents"
    ]
    resources = [
      "*"
    ]
  }
}

# IAM policy document for the container to access cloudwatch
data "aws_iam_policy_document" "cloudwatch_role_policy_document" {
  statement {
    sid    = "CloudwatchAccess"
    effect = "Allow"
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
      "logs:DescribeLogStreams"
    ]
    resources = [
      "arn:aws:logs:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:*"
    ]
  }
}


data "aws_iam_policy_document" "assume_policy_document" {
  statement {
    effect = "Allow"
    principals {
      identifiers = ["ec2.amazonaws.com", "ecs.amazonaws.com", "ecs-tasks.amazonaws.com"]
      type        = "Service"
    }
    actions = ["sts:AssumeRole"]
  }
}

data "awsprofiler_list" "list_profiles" {}

data "aws_region" "current" {}

data "aws_caller_identity" "current" {}
