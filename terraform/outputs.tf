output "sqs" {
  value = {
    request  = aws_sqs_queue.markov_user_request,
    response = aws_sqs_queue.markov_user_response,
  }
  sensitive = true
}
