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
  name            = "statisticsbot"
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
}


resource "aws_ecs_task_definition" "statisticsbot_service" {
  family                   = "statisticsbot"
  requires_compatibilities = ["EC2"]
  execution_role_arn       = aws_iam_role.statisticsbot_role.arn
  task_role_arn            = aws_iam_role.statisticsbot_role.arn

  cpu          = 256
  memory       = 400
  network_mode = "bridge"

  runtime_platform {
    cpu_architecture        = "ARM64"
    operating_system_family = "LINUX"
  }
  container_definitions = jsonencode([
    {
      name      = "statisticsbot"
      image     = "${data.terraform_remote_state.discord_bots_cluster.outputs.discord_bots_repo.repository_url}:${"statisticsbot"}-SNAPSHOT-6c26712-arm64"
      cpu       = 256
      memory    = 400
      essential = true

      environment = [
        {
          name  = "AWS_REGION"
          value = data.aws_region.current.name
        },
        {
          name  = "AWS_PARAMETER_NAME"
          value = "/discord_tokens/${"statisticsbot"}"
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
          value = data.terraform_remote_state.sqs_queues.outputs.sqs_queue.markov_user_request.url
        },
        {
          name  = "SQS_RESPONSE"
          value = data.terraform_remote_state.sqs_queues.outputs.sqs_queue.markov_user_response.url
        },
      ]
    }
  ])
}
