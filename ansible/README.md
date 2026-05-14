# Ansible Playbooks — GoTicket

## Prerequisites
- Ansible installed: `pip install ansible` or `brew install ansible`
- SSH access to target server
- Update `inventory.ini` with your server IP

## Usage

```bash
cd ansible/

# 1. Install Docker, Docker Compose, kubectl on target server
ansible-playbook -i inventory.ini install.yml

# 2. Deploy the application
ansible-playbook -i inventory.ini deploy.yml

# 3. Verify monitoring is working
ansible-playbook -i inventory.ini monitoring.yml
```

## Playbook Descriptions

| Playbook | Purpose |
|----------|---------|
| `install.yml` | Installs Docker, Docker Compose, kubectl on Ubuntu 22.04 |
| `deploy.yml` | Clones repo, builds and starts all services via Docker Compose |
| `monitoring.yml` | Verifies Prometheus and Grafana are healthy and scraping targets |

## Local Testing (without remote server)
```bash
# Test playbook syntax
ansible-playbook install.yml --syntax-check
ansible-playbook deploy.yml --syntax-check

# Run against localhost (requires local Docker)
ansible-playbook -i "localhost," -c local deploy.yml
```
