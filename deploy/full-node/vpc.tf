resource "aws_vpc" "piri" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name        = "piri-vpc"
    Environment = var.environment
  }
}

resource "aws_subnet" "public" {
  vpc_id                  = aws_vpc.piri.id
  cidr_block              = "10.0.1.0/24"
  availability_zone       = data.aws_availability_zones.available.names[0]
  map_public_ip_on_launch = true

  tags = {
    Name        = "piri-public-subnet"
    Environment = var.environment
  }
}

resource "aws_internet_gateway" "piri" {
  vpc_id = aws_vpc.piri.id

  tags = {
    Name        = "piri-igw"
    Environment = var.environment
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.piri.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.piri.id
  }

  tags = {
    Name        = "piri-public-rt"
    Environment = var.environment
  }
}

resource "aws_route_table_association" "public" {
  subnet_id      = aws_subnet.public.id
  route_table_id = aws_route_table.public.id
}

data "aws_availability_zones" "available" {
  state = "available"
}