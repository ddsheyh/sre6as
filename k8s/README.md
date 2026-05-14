# Kubernetes Deployment — GoTicket

## Prerequisites
- Minikube installed: `brew install minikube` (macOS) or see https://minikube.sigs.k8s.io
- kubectl installed: `brew install kubectl`

## Quick Start

```bash
# 1. Start Minikube
minikube start

# 2. Use Minikube's Docker daemon (so k8s can find local images)
eval $(minikube docker-env)

# 3. Build images inside Minikube
docker build -t goticket/auth-service ./services/auth
docker build -t goticket/event-service ./services/event
docker build -t goticket/order-service ./services/order

# 4. Apply all manifests
kubectl apply -f k8s/postgres/
kubectl apply -f k8s/auth/
kubectl apply -f k8s/event/
kubectl apply -f k8s/order/

# 5. Check status
kubectl get pods
kubectl get services

# 6. Access services (via port-forward)
kubectl port-forward svc/event-service 8083:8080 &
curl http://localhost:8083/api/events
```

## Cleanup
```bash
kubectl delete -f k8s/order/ -f k8s/event/ -f k8s/auth/ -f k8s/postgres/
minikube stop
```
