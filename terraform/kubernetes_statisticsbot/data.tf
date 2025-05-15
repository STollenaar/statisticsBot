data "terraform_remote_state" "discord_bots_cluster" {
  backend = "s3"
  config = {
    region = "ca-central-1"
    bucket = "stollenaar-terraform-states"
    key    = "infrastructure/terraform.tfstate"
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

data "aws_region" "current" {}
