include "root" {
  path = find_in_parent_folders()
}

dependencies {
  paths = ["../sqs"]
}


locals {
  parent_config = read_terragrunt_config("${get_parent_terragrunt_dir()}/terragrunt.hcl")
}

terraform {
  extra_arguments "common_vars" {
    commands = get_terraform_commands_that_need_vars()
    arguments = [
      "-var=kubeconfig_file=${local.parent_config.locals.kubeconfig_file}"
    ]
  }
}