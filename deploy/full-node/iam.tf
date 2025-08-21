resource "aws_iam_role" "piri_instance" {
  name = "piri-instance-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name        = "piri-instance-role"
    Environment = var.environment
  }
}

resource "aws_iam_role_policy" "piri_instance" {
  name = "piri-instance-policy"
  role = aws_iam_role.piri_instance.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ec2:DescribeVolumes",
          "ec2:AttachVolume",
          "ec2:CreateTags"
        ]
        Resource = "*"
      }
    ]
  })
}

resource "aws_iam_instance_profile" "piri" {
  name = "piri-instance-profile"
  role = aws_iam_role.piri_instance.name
}