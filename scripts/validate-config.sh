#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

ERRORS=0

echo "============================================"
echo "  GoTicket Configuration Validation"
echo "============================================"
echo ""

echo -n "Checking .env file exists... "
if [ -f .env ]; then
    echo -e "${GREEN}OK${NC}"
else
    echo -e "${RED}FAIL${NC} — .env file not found"
    ERRORS=$((ERRORS + 1))
fi

echo ""
echo "Checking required environment variables:"
REQUIRED_VARS=(
    "POSTGRES_USER"
    "POSTGRES_PASSWORD"
    "POSTGRES_DB"
    "DB_HOST"
    "DB_PORT"
    "DB_USER"
    "DB_PASSWORD"
    "DB_NAME"
    "JWT_SECRET"
    "SERVICE_PORT"
    "ORDER_DB_HOST"
)

if [ -f .env ]; then
    source .env
fi

for var in "${REQUIRED_VARS[@]}"; do
    echo -n "  $var... "
    val=$(eval echo \$$var)
    if [ -z "$val" ]; then
        echo -e "${RED}MISSING${NC}"
        ERRORS=$((ERRORS + 1))
    else
        echo -e "${GREEN}OK${NC} ($val)"
    fi
done

echo ""
echo -n "Checking ORDER_DB_HOST is valid... "
if [ "$ORDER_DB_HOST" = "postgres" ]; then
    echo -e "${GREEN}OK${NC} (points to postgres container)"
elif [ "$ORDER_DB_HOST" = "invalid-db-host" ] || [ "$ORDER_DB_HOST" = "invalid-host" ]; then
    echo -e "${RED}FAIL${NC} — ORDER_DB_HOST is set to '$ORDER_DB_HOST' (incident config!)"
    ERRORS=$((ERRORS + 1))
else
    echo -e "${YELLOW}WARNING${NC} — ORDER_DB_HOST='$ORDER_DB_HOST' (unusual value)"
fi

echo ""
echo -n "Checking Docker is available... "
if docker info > /dev/null 2>&1; then
    echo -e "${GREEN}OK${NC}"
else
    echo -e "${RED}FAIL${NC} — Docker is not running"
    ERRORS=$((ERRORS + 1))
fi

echo -n "Checking Docker Compose... "
if docker compose version > /dev/null 2>&1; then
    echo -e "${GREEN}OK${NC}"
else
    echo -e "${RED}FAIL${NC} — Docker Compose not found"
    ERRORS=$((ERRORS + 1))
fi

echo -n "Validating docker-compose.yml... "
if docker compose config > /dev/null 2>&1; then
    echo -e "${GREEN}OK${NC}"
else
    echo -e "${RED}FAIL${NC} — docker-compose.yml has errors"
    ERRORS=$((ERRORS + 1))
fi

echo ""
echo "Checking required files:"
REQUIRED_FILES=(
    "docker-compose.yml"
    "db/init.sql"
    "nginx/nginx.conf"
    "monitoring/prometheus/prometheus.yml"
    "monitoring/prometheus/alert_rules.yml"
    "services/auth/Dockerfile"
    "services/user/Dockerfile"
    "services/event/Dockerfile"
    "services/order/Dockerfile"
    "services/chat/Dockerfile"
    "frontend/index.html"
)

for file in "${REQUIRED_FILES[@]}"; do
    echo -n "  $file... "
    if [ -f "$file" ]; then
        echo -e "${GREEN}OK${NC}"
    else
        echo -e "${RED}MISSING${NC}"
        ERRORS=$((ERRORS + 1))
    fi
done

echo ""
echo "Checking port availability:"
for port in 80 3000 5432 9090; do
    echo -n "  Port $port... "
    if ! lsof -i :$port > /dev/null 2>&1; then
        echo -e "${GREEN}AVAILABLE${NC}"
    else
        echo -e "${YELLOW}IN USE${NC} (may conflict)"
    fi
done

echo ""
echo "============================================"
if [ $ERRORS -eq 0 ]; then
    echo -e "  ${GREEN}VALIDATION PASSED${NC} — Ready to deploy"
    echo "  Run: docker compose up -d --build"
else
    echo -e "  ${RED}VALIDATION FAILED${NC} — $ERRORS error(s) found"
    echo "  Fix the issues above before deploying."
fi
echo "============================================"

exit $ERRORS
