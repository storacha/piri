resource "aws_sqs_queue" "piece_aggregator" {
  count                       = var.use_pdp ? 1 : 0
  name                        = "${terraform.workspace}-${var.app}-piece-aggregator.fifo"
  fifo_queue                  = true
  content_based_deduplication = true
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.piece_aggregator_deadletter[0].arn
    maxReceiveCount     = 4
  })
  tags = {
    Name = "${terraform.workspace}-${var.app}-piece-aggregator"
  }
  visibility_timeout_seconds = 60
}

resource "aws_sqs_queue" "piece_aggregator_deadletter" {
  count                       = var.use_pdp ? 1 : 0
  fifo_queue                  = true
  content_based_deduplication = true
  name                        = "${terraform.workspace}-${var.app}-piece-aggregator-deadletter.fifo"
}

resource "aws_sqs_queue_redrive_allow_policy" "piece_aggregator" {
  count     = var.use_pdp ? 1 : 0
  queue_url = aws_sqs_queue.piece_aggregator_deadletter[0].id

  redrive_allow_policy = jsonencode({
    redrivePermission = "byQueue",
    sourceQueueArns   = [aws_sqs_queue.piece_aggregator[0].arn]
  })
}

resource "aws_sqs_queue" "piece_accepter" {
  count                       = var.use_pdp ? 1 : 0
  name                        = "${terraform.workspace}-${var.app}-piece-accepter.fifo"
  fifo_queue                  = true
  content_based_deduplication = true
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.piece_accepter_deadletter[0].arn
    maxReceiveCount     = 4
  })
  tags = {
    Name = "${terraform.workspace}-${var.app}-piece-accepter"
  }
  visibility_timeout_seconds = 60
}

resource "aws_sqs_queue" "piece_accepter_deadletter" {
  count                       = var.use_pdp ? 1 : 0
  fifo_queue                  = true
  content_based_deduplication = true
  name                        = "${terraform.workspace}-${var.app}-piece-accepter-deadletter.fifo"
}

resource "aws_sqs_queue_redrive_allow_policy" "piece_accepter" {
  count     = var.use_pdp ? 1 : 0
  queue_url = aws_sqs_queue.piece_accepter_deadletter[0].id

  redrive_allow_policy = jsonencode({
    redrivePermission = "byQueue",
    sourceQueueArns   = [aws_sqs_queue.piece_accepter[0].arn]
  })
}

resource "aws_sqs_queue" "aggregate_submitter" {
  count                       = var.use_pdp ? 1 : 0
  name                        = "${terraform.workspace}-${var.app}-aggregate-submitter.fifo"
  fifo_queue                  = true
  content_based_deduplication = true
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.aggregate_submitter_deadletter[0].arn
    maxReceiveCount     = 4
  })
  tags = {
    Name = "${terraform.workspace}-${var.app}-aggregate-submitter"
  }
  visibility_timeout_seconds = 60
}

resource "aws_sqs_queue" "aggregate_submitter_deadletter" {
  count                       = var.use_pdp ? 1 : 0
  fifo_queue                  = true
  content_based_deduplication = true
  name                        = "${terraform.workspace}-${var.app}-aggregate-submitter-deadletter.fifo"
}

resource "aws_sqs_queue_redrive_allow_policy" "aggregate_submitter" {
  count     = var.use_pdp ? 1 : 0
  queue_url = aws_sqs_queue.aggregate_submitter_deadletter[0].id

  redrive_allow_policy = jsonencode({
    redrivePermission = "byQueue",
    sourceQueueArns   = [aws_sqs_queue.aggregate_submitter[0].arn]
  })
}

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
