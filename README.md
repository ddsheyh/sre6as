# GoTicket — Event Ticketing Platform

> Comprehensive SRE project: 6 Go microservices, Docker Compose, Docker Swarm, Kubernetes, Terraform, Ansible, Prometheus + Grafana monitoring.

**GitHub Repository:** https://github.com/ddsheyh/sre6as

---

## Architecture

```
┌──────────────┐
│   Browser    │
└──────┬───────┘
       │ :80
┌──────▼───────┐
│    Nginx     │── Static Frontend (HTML/CSS/JS)
│ Reverse Proxy│
└──────┬───────┘
       │
   ┌───┼──────────────────────────────────┐
   │   ├──► auth-service         :8080    │
   │   ├──► user-service         :8080    │
   │   ├──► event-service        :8080    │
   │   ├──► order-service        :8080    │
   │   ├──► chat-service         :8080    │
   │   └──► notification-service :8080    │
   │                                      │
   │   PostgreSQL :5432                   │
   │   Prometheus :9090 → Grafana :3000   │
   └──────────────────────────────────────┘
```

## Services (6 microservices)

| Service | Responsibility | Endpoints |
|---------|---------------|-----------|
| **auth-service** | Registration, login, JWT tokens | `POST /api/auth/register`, `POST /api/auth/login` |
| **user-service** | User profile retrieval | `GET /api/users/me` |
| **event-service** | Event listing and details | `GET /api/events`, `GET /api/events/{id}` |
| **order-service** | Ticket order creation and listing | `POST /api/orders`, `GET /api/orders` |
| **chat-service** | Support chat messages | `GET /api/messages`, `POST /api/messages` |
| **notification-service** | Notification simulation | `GET /api/notifications`, `POST /api/notifications` |

All services expose `/health` and `/metrics` endpoints.

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend | Go 1.22 |
| Frontend | HTML/CSS/JavaScript |
| Database | PostgreSQL 15 |
| Reverse Proxy | Nginx |
| Containerization | Docker |
| Orchestration | Docker Compose / Docker Swarm / Kubernetes |
| Monitoring | Prometheus + Grafana |
| IaC | Terraform (AWS) |
| Configuration Management | Ansible |

---

## Quick Start (Docker Compose)

```bash
# 1. Clone
git clone https://github.com/ddsheyh/sre6as.git && cd sre6as

# 2. Validate config
./scripts/validate-config.sh

# 3. Start all services
docker compose up -d --build

# 4. Access
# Web App:    http://localhost
# Grafana:    http://localhost:3000 (admin/admin)
# Prometheus: http://localhost:9090
```

## Docker Swarm Deployment

```bash
# Initialize swarm and deploy
./scripts/deploy-swarm.sh

# Or manually:
docker swarm init
docker compose build
docker stack deploy -c docker-compose.yml goticket

# Check services
docker stack services goticket

# Remove
docker stack rm goticket
docker swarm leave --force
```

## Kubernetes Deployment (Minikube)

```bash
# Start Minikube
minikube start
eval $(minikube docker-env)

# Build images
docker build -t goticket/auth-service ./services/auth
docker build -t goticket/event-service ./services/event
docker build -t goticket/order-service ./services/order

# Deploy
kubectl apply -f k8s/postgres/
kubectl apply -f k8s/auth/
kubectl apply -f k8s/event/
kubectl apply -f k8s/order/

# Verify
kubectl get pods
kubectl get services
```

See [k8s/README.md](k8s/README.md) for detailed instructions.

## Ansible Deployment

```bash
cd ansible/

# Update inventory.ini with your server IP
# Then:
ansible-playbook -i inventory.ini install.yml     # Install Docker
ansible-playbook -i inventory.ini deploy.yml       # Deploy app
ansible-playbook -i inventory.ini monitoring.yml   # Verify monitoring
```

See [ansible/README.md](ansible/README.md) for details.

## Monitoring

- **Prometheus**: http://localhost:9090 — metrics collection, alert rules
- **Grafana**: http://localhost:3000 — dashboards (login: admin/admin)
- **Alert rules**: Service down, DB disconnected, high error rate, high latency

## SLI/SLO

| SLO | Target |
|-----|--------|
| Availability | ≥ 99% |
| Latency (p95) | ≤ 200ms |
| Error Rate | ≤ 1% |

See [docs/sli-slo.md](docs/sli-slo.md) for PromQL queries and alert mapping.

## Incident Simulation

```bash
# Break order-service
sed -i '' 's/ORDER_DB_HOST=postgres/ORDER_DB_HOST=invalid-host/' .env
docker compose up -d --force-recreate order-service

# Observe: logs, Prometheus, Grafana
docker compose logs order-service --tail 20

# Fix
sed -i '' 's/ORDER_DB_HOST=invalid-host/ORDER_DB_HOST=postgres/' .env
docker compose up -d --force-recreate order-service
```

See [docs/incident-report.md](docs/incident-report.md) and [docs/postmortem.md](docs/postmortem.md).

## Terraform

```bash
cd terraform/
terraform init
terraform plan
terraform apply
terraform output   # Get public IP
```

See [docs/terraform-explanation.md](docs/terraform-explanation.md).

## Repository Structure

```
sre6as/
├── services/
│   ├── auth/          # JWT authentication
│   ├── user/          # User profiles
│   ├── event/         # Event catalog
│   ├── order/         # Ticket orders
│   ├── chat/          # Support chat
│   └── notification/  # Notifications (6th service)
├── frontend/          # HTML/CSS/JS
├── nginx/             # Reverse proxy config
├── db/                # PostgreSQL init schema
├── monitoring/
│   ├── prometheus/    # Scrape config + alert rules
│   └── grafana/       # Dashboard provisioning
├── k8s/               # Kubernetes manifests
│   ├── postgres/
│   ├── auth/
│   ├── event/
│   └── order/
├── ansible/           # Deployment playbooks
├── terraform/         # AWS EC2 provisioning
├── scripts/           # Automation scripts
├── docs/              # Reports and documentation
├── docker-compose.yml
├── .env
└── README.md
```

## Assignment Coverage

| Requirement | How Satisfied |
|-------------|---------------|
| 6+ microservices | auth, user, event, order, chat, notification |
| Docker Compose | `docker-compose.yml` with all 10 containers |
| Docker Swarm | `scripts/deploy-swarm.sh`, deploy sections in compose |
| Kubernetes | `k8s/` manifests for postgres, auth, event, order |
| Terraform | `terraform/` — AWS EC2 + security group |
| Ansible | `ansible/` — install, deploy, monitoring playbooks |
| Monitoring | Prometheus scraping + Grafana dashboards + alert rules |
| SLI/SLO | `docs/sli-slo.md` with PromQL queries |
| Incident simulation | Order Service DB misconfiguration scenario |
| Postmortem | `docs/postmortem.md` |
| Health checks | `/health` on every service + Docker healthchecks |
| Git repository | https://github.com/ddsheyh/sre6as |
