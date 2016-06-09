variable "imageId" {
  description = "AMI ID of the F5 Image."
}
variable "aws_access_key"{}
variable "aws_secret_key"{}
variable "aws_region"{}

provider "aws" {
  access_key = "${var.aws_access_key}"
  secret_key = "${var.aws_secret_key}"
  region = "${var.aws_region}"
}

resource "aws_iam_user" "main" {
  name = "terraform-provider-bigip-test"
}

resource "aws_iam_access_key" "main" {
  user = "${aws_iam_user.main.name}"
}

resource "aws_instance" "main" {
  ami = "${var.imageId}"
  instance_type = "m3.large"
  security_groups = ["${aws_security_group.main.name}"]
  tags {
    Name = "terraform-provider-bigip-test"
  }
}

resource "aws_security_group" "main" {
  name = "terraform-provider-bigip-test"
  description = "Allow traffic to the test F5s"

  ingress {
    from_port = 443
    to_port = 443
    protocol = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_iam_user_policy" "main" {
  name = "terraform-provider-bigip-test"
  user = "${aws_iam_user.main.name}"
  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ec2:DescribeInstances",
                "ec2:StartInstances",
                "ec2:StopInstances"
            ],
            "Resource": [
                "arn:aws:ec2:${var.aws_region}:*:instance/${aws_instance.main.id}"
            ]
        },
        {
            "Effect": "Allow",
            "Action": [
                "ec2:DescribeInstances"
            ],
            "Resource": [
                "*"
            ]
        }
    ]
}
EOF
}