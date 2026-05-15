# GoTicket Terraform Variables
# Copy this file to terraform.tfvars and fill in YOUR values

aws_region     = "us-east-1"
aws_access_key = "YOUR_AWS_ACCESS_KEY_ID"
aws_secret_key = "YOUR_AWS_SECRET_ACCESS_KEY"
instance_type  = "t2.micro"
ami_id         = "ami-0c7217cdde317cfec"    # Ubuntu 22.04 LTS in us-east-1
key_name       = "YOUR_KEY_PAIR_NAME"        # Name of key pair from AWS Console
repo_url       = "https://github.com/ddsheyh/sre6as.git"
