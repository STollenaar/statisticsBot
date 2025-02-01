include "root" {
  path = find_in_parent_folders()
}

locals {
  parent_config = read_terragrunt_config("${get_parent_terragrunt_dir()}/terragrunt.hcl")
}

terraform {
  before_hook "before_hook" {
    commands = ["apply", "destroy", "plan"]
    execute  = ["./conf/start_service.sh", read_terragrunt_config("${find_in_parent_folders()}").locals.kubeconfig_file]
    # get_env("KUBES_ENDPOINT", "somedefaulturl") you can get env vars or pass in params as needed to the script
  }

  after_hook "after_hook" {
    commands     = ["apply", "destroy", "plan"]
    execute      = ["./conf/stop_service.sh"]
    run_on_error = true
  }
  extra_arguments "common_vars" {
    commands = get_terraform_commands_that_need_vars()
    arguments = [
      "-var=kubeconfig_file=${local.parent_config.locals.kubeconfig_file}"
    ]
  }
}

dependencies {
  paths = ["../iam"]
}
