variable "aws_region" {
  description = "AWS region to deploy the instance"
  type        = string
  default     = "eu-north-1"
}

variable "aws_access_key" {
  description = "AWS Access Key ID"
  type        = string
  sensitive   = true
}

variable "aws_secret_key" {
  description = "AWS Secret Access Key"
  type        = string
  sensitive   = true
}

variable "instance_type" {
  description = "EC2 instance type (t2.micro = free tier)"
  type        = string
  default     = "t3.micro"
}

variable "ami_id" {
  description = "AMI ID for Ubuntu 22.04 LTS (us-east-1)"
  type        = string
  default     = "ami-05d62b9bc5a6ca605"
}

variable "key_name" {
  description = "Name of the SSH key pair created in AWS Console"
  type        = string
}

variable "repo_url" {
  description = "Git repository URL for the project"
  type        = string
  default     = "https://github.com/ddsheyh/sre6as.git"
}
