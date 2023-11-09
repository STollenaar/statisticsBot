output "sqs_queue" {
  value = {
    markov_user_request  = aws_sqs_queue.markov_user_request
    markov_user_response = aws_sqs_queue.markov_user_response
  }
}
