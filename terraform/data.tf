data "terraform_remote_state" "discord_bots_cluster" {
  backend = "s3"
  config = {
    profile = local.used_profile.name
    region = "ca-central-1"
    bucket = "stollenaar-terraform-states"
    key    = "infrastructure/terraform.tfstate"
  }
}
data "awsprofiler_list" "list_profiles" {}

data "aws_region" "current" {}
