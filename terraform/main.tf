terraform {
  backend "s3" {
    region  = "ca-central-1"
    profile = "personal"
    bucket  = "stollenaar-terraform-states"
    key     = "discordbots/statisticsBot.tfstate"
  }
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.13.1"
    }
    awsprofiler = {
      version = "~> 0.0.1"
      source  = "spices.dev/stollenaar/awsprofiler"
    }
  }
  required_version = ">= 1.0.0"
}

locals {
  name         = "statisticsbot"
  used_profile = data.awsprofiler_list.list_profiles.profiles[try(index(data.awsprofiler_list.list_profiles.profiles.*.name, "personal"), 0)]
}


provider "aws" {
  profile = local.used_profile.name
}

resource "aws_iam_role" "statisticsbot_role" {
  name               = "StatisticsbotRole"
  description        = "Role for the statisticsbot"
  assume_role_policy = data.aws_iam_policy_document.assume_policy_document.json
}

resource "aws_iam_role_policy" "statisticsbot_role_policy" {
  role   = aws_iam_role.statisticsbot_role.id
  name   = "inline-role"
  policy = data.aws_iam_policy_document.ssm_access_role_policy_document.json
}


resource "aws_ecs_service" "statisticsbot_service" {
  name            = local.name
  cluster         = data.terraform_remote_state.discord_bots_cluster.outputs.discord_bots_cluster.id
  task_definition = aws_ecs_task_definition.statisticsbot_service.arn
  desired_count   = 1

  capacity_provider_strategy {
    capacity_provider = data.terraform_remote_state.discord_bots_cluster.outputs.discord_bots_capacity_providers[0].name
    weight            = 100
  }

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  service_connect_configuration {
    enabled   = true
    namespace = data.terraform_remote_state.discord_bots_cluster.outputs.discord_bots_namespace.arn
    service {
      client_alias {
        dns_name = local.name
        port     = 3000
      }
      port_name = "api"
    }
  }
}

resource "aws_sqs_queue" "markov_user_request" {
  name                      = "user-request"
  message_retention_seconds = 60 * 10
}

resource "aws_sqs_queue" "markov_user_response" {
  name                      = "user-response"
  message_retention_seconds = 60 * 10
}

resource "aws_ecs_task_definition" "statisticsbot_service" {
  family                   = local.name
  requires_compatibilities = ["EC2"]
  execution_role_arn       = data.terraform_remote_state.discord_bots_cluster.outputs.spices_role.arn

  cpu          = 256
  memory       = 400
  network_mode = "bridge"

  runtime_platform {
    cpu_architecture        = "ARM64"
    operating_system_family = "LINUX"
  }
  container_definitions = jsonencode([
    {
      name      = local.name
      image     = "${data.terraform_remote_state.discord_bots_cluster.outputs.discord_bots_repo.repository_url}:${local.name}-latest-arm64"
      cpu       = 256
      memory    = 400
      essential = true

      portMappings = [
        {
          containerPort = 3000
          name          = "api"
          hostPort      = 3000
        }
      ]
      environment = [
        {
          name  = "AWS_REGION"
          value = data.aws_region.current.name
        },
        {
          name  = "AWS_PARAMETER_NAME"
          value = "/discord_tokens/${local.name}"
        },
        {
          name  = "MONGO_HOST_PARAMETER"
          value = "/mongodb/statsuser/database_host"
        },
        {
          name  = "MONGO_PASSWORD_PARAMETER"
          value = "/mongodb/statsuser/password"
        },
        {
          name  = "MONGO_USERNAME_PARAMETER"
          value = "/mongodb/statsuser/username"
        },
        {
          name  = "SQS_REQUEST"
          value = aws_sqs_queue.markov_user_request.name
        },
        {
          name  = "SQS_RESPONSE"
          value = aws_sqs_queue.markov_user_response.name
        },
      ]
    }
  ])
}
