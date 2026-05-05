variable "aws_region" {
  description = "AWS region to deploy the instance"
  type        = string
  default     = "us-east-1"
}

variable "instance_type" {
  description = "EC2 instance type"
  type        = string
  default     = "t2.micro"
}

variable "ami_id" {
  description = "AMI ID for Amazon Linux 2023"
  type        = string
  default     = "ami-0c02fb55956c7d316"
}

variable "key_name" {
  description = "Name of the SSH key pair"
  type        = string
}

variable "repo_url" {
  description = "Git repository URL for the project"
  type        = string
  default     = "https://github.com/your-username/goticket.git"
}
