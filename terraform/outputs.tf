output "public_ip" {
  description = "Public IP address of the GoTicket server"
  value       = aws_instance.goticket_server.public_ip
}

output "web_url" {
  description = "URL to access the web application"
  value       = "http://${aws_instance.goticket_server.public_ip}"
}

output "grafana_url" {
  description = "URL to access Grafana dashboard"
  value       = "http://${aws_instance.goticket_server.public_ip}:3000"
}

output "prometheus_url" {
  description = "URL to access Prometheus"
  value       = "http://${aws_instance.goticket_server.public_ip}:9090"
}

output "ssh_command" {
  description = "SSH command to connect to the server"
  value       = "ssh -i ~/.ssh/${var.key_name}.pem ec2-user@${aws_instance.goticket_server.public_ip}"
}
