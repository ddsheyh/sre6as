#!/bin/bash
# GoTicket — Docker Swarm Deployment Script
set -e

echo "============================================"
echo "  GoTicket — Docker Swarm Deployment"
echo "============================================"

# Step 1: Build all images using docker compose
echo ""
echo "[1/4] Building service images with docker compose..."
docker compose build
echo "  ✓ Images built"

# Step 2: Initialize Swarm if not active
echo ""
echo "[2/4] Checking Docker Swarm..."
if ! docker info 2>/dev/null | grep -q "Swarm: active"; then
    echo "  Initializing Docker Swarm..."
    docker swarm init 2>/dev/null || echo "  Swarm already initialized"
else
    echo "  ✓ Swarm already active"
fi

# Step 3: Create named volumes and copy config files into them
echo ""
echo "[3/4] Initializing Swarm Config Volumes..."
# Create dummy containers to copy local files into named volumes
docker run --rm -v frontend_data:/data -v $(pwd)/frontend:/source alpine cp -r /source/. /data/
docker run --rm -v nginx_config:/data -v $(pwd)/nginx:/source alpine cp -r /source/. /data/
docker run --rm -v prometheus_config:/data -v $(pwd)/monitoring/prometheus:/source alpine cp -r /source/. /data/
docker run --rm -v grafana_provisioning:/data -v $(pwd)/monitoring/grafana/provisioning:/source alpine cp -r /source/. /data/
docker run --rm -v db_init:/data -v $(pwd)/db:/source alpine cp -r /source/. /data/
echo "  ✓ Configs copied to named volumes"

# Step 4: Deploy stack using the Swarm-specific file
echo ""
echo "[4/4] Deploying GoTicket stack..."
docker stack deploy -c docker-stack.yml goticket

echo ""
echo "============================================"
echo "  Stack deployed! Waiting for services..."
echo "============================================"
sleep 5

docker stack services goticket

echo ""
echo "  Useful commands:"
echo "    docker stack services goticket"
echo "    docker stack ps goticket"
echo "    docker stack rm goticket"
echo "    docker swarm leave --force"
echo ""
echo "  Web App:    http://localhost"
echo "  Grafana:    http://localhost:3000"
echo "  Prometheus: http://localhost:9090"
echo ""
