data "terraform_remote_state" "kubernetes_cluster" {
  backend = "s3"
  config = {
    region = "ca-central-1"
    bucket = "stollenaar-terraform-states"
    key    = "infrastructure/kubernetes/terraform.tfstate"
  }
}

data "terraform_remote_state" "kubernetes" {
  backend = "s3"
  config = {
    region = "ca-central-1"
    bucket = "stollenaar-terraform-states"
    key    = "discordbots/statisticsbot/kubernetes/terraform.tfstate"
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

data "terraform_remote_state" "ecr_repo" {
  backend = "s3"
  config = {
    region = "ca-central-1"
    bucket = "stollenaar-terraform-states"
    key    = "infrastructure/aws/ecr/terraform.tfstate"
  }
}

data "aws_region" "current" {}
