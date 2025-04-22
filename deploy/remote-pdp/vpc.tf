resource "aws_vpc" "pdp_vpc" {
  cidr_block = "10.0.0.0/16"
  tags = {
    Name = "${var.app}-vpc"
  }
}

# Internet Gateway
resource "aws_internet_gateway" "pdp_igw" {
  vpc_id = aws_vpc.pdp_vpc.id
  tags = {
    Name = "${var.app}-igw"
  }
}

# Public Subnet
resource "aws_subnet" "pdp_public_subnet" {
  vpc_id                  = aws_vpc.pdp_vpc.id
  cidr_block             = "10.0.1.0/24"
  availability_zone       = "us-west-2a"
  map_public_ip_on_launch = true
  tags = {
    Name = "${var.app}-public-subnet"
  }
}

# Route Table
resource "aws_route_table" "pdp_public_rt" {
  vpc_id = aws_vpc.pdp_vpc.id
  tags = {
    Name = "${var.app}-public-rt"
  }
}

# Default route to the Internet
resource "aws_route" "pdp_public_route_igw" {
  route_table_id         = aws_route_table.pdp_public_rt.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.pdp_igw.id
}

# Associate public subnet with the route table
resource "aws_route_table_association" "pdp_public_rta" {
  subnet_id      = aws_subnet.pdp_public_subnet.id
  route_table_id = aws_route_table.pdp_public_rt.id
}
