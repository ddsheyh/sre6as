# SLI/SLO Definitions — GoTicket

## Service Level Indicators (SLIs)

| SLI | Definition | Prometheus Metric | PromQL Query |
|-----|-----------|-------------------|--------------|
| **Availability** | Percentage of time services are reachable | `up` | `avg(up) * 100` |
| **Latency (p95)** | 95th percentile response time | `*_http_duration_seconds` | `histogram_quantile(0.95, rate(order_http_duration_seconds_bucket[5m]))` |
| **Error Rate** | Percentage of requests returning 4xx/5xx | `*_http_requests_total` | `sum(rate(order_http_requests_total{status=~"5.."}[5m])) / sum(rate(order_http_requests_total[5m])) * 100` |
| **Request Success Rate** | Percentage of successful requests (2xx) | `*_http_requests_total` | `sum(rate(event_http_requests_total{status="200"}[5m])) / sum(rate(event_http_requests_total[5m])) * 100` |

## Service Level Objectives (SLOs)

| SLO | Target | Measurement Window | Alert Threshold |
|-----|--------|-------------------|-----------------|
| **Availability** | ≥ 99% | 30-day rolling | `up == 0` for 30s triggers `ServiceDown` alert |
| **Latency (p95)** | ≤ 200ms | 5-minute window | p95 > 1s triggers `HighRequestLatency` alert |
| **Error Rate** | ≤ 1% | 1-minute window | Error rate > 0.08/s triggers `HighOrderErrorRate` alert |

## Mapping to Existing Alert Rules

Our `alert_rules.yml` enforces these SLOs:

| Alert Rule | SLO Enforced | Condition |
|-----------|-------------|-----------|
| `ServiceDown` | Availability ≥ 99% | `up == 0` for 30s |
| `OrderServiceDBDown` | Availability ≥ 99% | `order_db_up == 0` for 15s |
| `HighOrderErrorRate` | Error Rate ≤ 1% | `rate(order_errors_total[1m]) > 0.08` |
| `HighRequestLatency` | Latency p95 ≤ 200ms | `histogram_quantile(0.95, ...) > 1` |
| `ServiceFlapping` | Availability ≥ 99% | `changes(up[5m]) > 3` |

## Grafana Dashboard Queries

To visualize SLO compliance in Grafana:

### Availability (% uptime over last hour)
```promql
avg_over_time(up{job="order-service"}[1h]) * 100
```

### Latency p95 (seconds)
```promql
histogram_quantile(0.95, rate(order_http_duration_seconds_bucket[5m]))
```

### Error Rate (% of total requests)
```promql
sum(rate(order_http_requests_total{status=~"5.."}[5m]))
/ sum(rate(order_http_requests_total[5m])) * 100
```

### Request Success Rate (%)
```promql
sum(rate(event_http_requests_total{status="200"}[5m]))
/ sum(rate(event_http_requests_total[5m])) * 100
```

## Error Budget

With a 99% availability SLO over 30 days:
- **Total minutes in 30 days**: 43,200
- **Allowed downtime (1%)**: 432 minutes (~7.2 hours)
- **Per-incident budget**: aim to resolve within 10 minutes

The Assignment 4 incident (Order Service DB failure) consumed ~10 minutes of error budget, well within the 432-minute allowance.
