#!/bin/bash

BASE_URL="${1:-http://localhost}"
DURATION="${2:-30}"
CONCURRENCY="${3:-5}"

echo "============================================"
echo "  GoTicket Load Test"
echo "============================================"
echo "  Target:      $BASE_URL"
echo "  Duration:    ${DURATION}s"
echo "  Concurrency: $CONCURRENCY workers"
echo "============================================"
echo ""

worker() {
    local id=$1
    local end=$((SECONDS + DURATION))
    local count=0
    local errors=0

    while [ $SECONDS -lt $end ]; do
        status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/events" 2>/dev/null)
        if [ "$status" != "200" ]; then
            errors=$((errors + 1))
        fi
        count=$((count + 1))

        status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/events/1" 2>/dev/null)
        if [ "$status" != "200" ]; then
            errors=$((errors + 1))
        fi
        count=$((count + 1))

        sleep 0.1
    done

    echo "  Worker $id: $count requests, $errors errors"
}

echo "[Phase 1] Warm-up — checking service health..."
for svc in auth users events orders messages; do
    endpoint="$BASE_URL/api/$svc"
    if [ "$svc" = "auth" ]; then endpoint="$BASE_URL/api/auth/login"; fi
    if [ "$svc" = "users" ]; then endpoint="$BASE_URL/api/users/me"; fi
    if [ "$svc" = "events" ]; then endpoint="$BASE_URL/api/events"; fi
    if [ "$svc" = "orders" ]; then endpoint="$BASE_URL/api/orders"; fi
    if [ "$svc" = "messages" ]; then endpoint="$BASE_URL/api/messages"; fi

    status=$(curl -s -o /dev/null -w "%{http_code}" "$endpoint" 2>/dev/null)
    echo "  $svc → HTTP $status"
done

echo ""
echo "[Phase 2] Running load test with $CONCURRENCY concurrent workers for ${DURATION}s..."
echo "  Sending GET /api/events and GET /api/events/1 in parallel loops"
echo ""

START_TIME=$SECONDS

for i in $(seq 1 $CONCURRENCY); do
    worker $i &
done

wait

ELAPSED=$((SECONDS - START_TIME))
echo ""
echo "  Load test completed in ${ELAPSED}s"

echo ""
echo "[Phase 3] Post-load metrics check..."
echo ""

echo "  Event Service request count:"
curl -s "$BASE_URL/api/events" > /dev/null 2>&1
echo "    → Service responding: $(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/api/events")"

echo ""
echo "  Prometheus metrics (if accessible):"
echo "    → Check http://localhost:9090/graph for request rate graphs"
echo "    → Check http://localhost:3000 for Grafana dashboards"

echo ""
echo "============================================"
echo "  Load test complete."
echo "  Check Grafana dashboard for:"
echo "    - Request rate spike during test"
echo "    - Response time changes"
echo "    - Error rate (should be ~0)"
echo "    - Resource usage patterns"
echo "============================================"
