#!/bin/bash

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "============================================"
echo "  GoTicket Log Troubleshooting"
echo "============================================"
echo ""

SERVICES="auth-service user-service event-service order-service chat-service postgres nginx"
ISSUES=0

for svc in $SERVICES; do
    echo "--- $svc ---"

    status=$(docker compose ps --format "{{.Status}}" $svc 2>/dev/null | head -1)
    if [ -z "$status" ]; then
        echo -e "  Status: ${RED}NOT RUNNING${NC}"
        ISSUES=$((ISSUES + 1))
        echo ""
        continue
    fi
    echo -e "  Status: ${GREEN}$status${NC}"

    logs=$(docker compose logs --tail 50 $svc 2>/dev/null)

    db_errors=$(echo "$logs" | grep -ci "cannot connect\|connection refused\|no such host\|FATAL.*database" 2>/dev/null || true)
    if [ "$db_errors" -gt 0 ]; then
        echo -e "  ${RED}⚠ Database connection errors: $db_errors occurrences${NC}"
        ISSUES=$((ISSUES + 1))
    fi

    health_errors=$(echo "$logs" | grep -ci "HEALTH CHECK FAILED\|DB HEALTH CHECK FAILED" 2>/dev/null || true)
    if [ "$health_errors" -gt 0 ]; then
        echo -e "  ${YELLOW}⚠ Health check failures: $health_errors occurrences${NC}"
        ISSUES=$((ISSUES + 1))
    fi

    restart_count=$(docker inspect --format '{{.RestartCount}}' "$(docker compose ps -q $svc 2>/dev/null | head -1)" 2>/dev/null || echo "0")
    if [ "$restart_count" -gt 2 ]; then
        echo -e "  ${RED}⚠ Container restarted $restart_count times${NC}"
        ISSUES=$((ISSUES + 1))
    fi

    panics=$(echo "$logs" | grep -ci "panic\|FATAL" 2>/dev/null || true)
    if [ "$panics" -gt 0 ]; then
        echo -e "  ${RED}⚠ Panic/Fatal errors: $panics occurrences${NC}"
        ISSUES=$((ISSUES + 1))
    fi

    if [ "$db_errors" -eq 0 ] && [ "$health_errors" -eq 0 ] && [ "$restart_count" -le 2 ] && [ "$panics" -eq 0 ]; then
        echo -e "  ${GREEN}✓ No issues detected${NC}"
    fi

    echo ""
done

echo "============================================"
if [ $ISSUES -eq 0 ]; then
    echo -e "  ${GREEN}All services healthy — no issues found${NC}"
else
    echo -e "  ${YELLOW}$ISSUES issue(s) found — review logs above${NC}"
    echo ""
    echo "  Quick commands:"
    echo "    docker compose logs order-service --tail 20"
    echo "    docker compose restart order-service"
    echo "    docker compose ps"
fi
echo "============================================"
