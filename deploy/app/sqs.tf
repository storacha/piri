resource "aws_sqs_queue" "ipni_publisher" {
  name                        = "${terraform.workspace}-${var.app}-ipni-publisher.fifo"
  fifo_queue                  = true
  content_based_deduplication = true
  deduplication_scope         = "messageGroup"
  fifo_throughput_limit       = "perMessageGroupId"
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.ipni_publisher_deadletter.arn
    maxReceiveCount     = 4
  })
  tags = {
    Name = "${terraform.workspace}-${var.app}-ipni-publisher"
  }
  visibility_timeout_seconds = 60
}

resource "aws_sqs_queue" "ipni_publisher_deadletter" {
  fifo_queue                  = true
  content_based_deduplication = true
  deduplication_scope         = "messageGroup"
  fifo_throughput_limit       = "perMessageGroupId"
  name                        = "${terraform.workspace}-${var.app}-ipni-publisher-deadletter.fifo"
}

resource "aws_sqs_queue_redrive_allow_policy" "ipni_publisher" {
  queue_url = aws_sqs_queue.ipni_publisher_deadletter.id

  redrive_allow_policy = jsonencode({
    redrivePermission = "byQueue",
    sourceQueueArns   = [aws_sqs_queue.ipni_publisher.arn]
  })
}
