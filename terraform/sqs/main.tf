resource "aws_sqs_queue" "markov_user_request" {
  name                       = "user-request"
  message_retention_seconds  = 60 * 10
  receive_wait_time_seconds  = 10
  visibility_timeout_seconds = 60 * 5
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dead_letter_user_request.arn
    maxReceiveCount     = 1
  })
}

resource "aws_sqs_queue" "markov_user_response" {
  name                       = "user-response"
  message_retention_seconds  = 60 * 10
  visibility_timeout_seconds = 60 * 5
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dead_letter_user_response.arn
    maxReceiveCount     = 1
  })

}

# TODO: Add alerting if these fillup
resource "aws_sqs_queue" "dead_letter_user_request" {
  name = "dead-user-request"
}

resource "aws_sqs_queue" "dead_letter_user_response" {
  name = "dead-user-response"
}