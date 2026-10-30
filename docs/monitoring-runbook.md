# Local monitoring and API metrics

The Go API exposes Prometheus metrics at `GET /metrics` only when `PROMETHEUS_BEARER_TOKEN` is set. The token must be at least 32 characters. Scrapes require `Authorization: Bearer <token>`. The endpoint records only route templates, HTTP methods/status codes, bounded checkout and webhook outcomes, and database readiness; it does not label customer identifiers, addresses, payment bodies, query parameters, or credentials.

## Start locally (Windows PowerShell)

1. Generate a local random scrape token and save the same value in `backend/.env` as `PROMETHEUS_BEARER_TOKEN` and in `infrastructure/monitoring/metrics-token`. Both are ignored by Git. Do not paste either value into source control.
2. Create an ignored `infrastructure/monitoring/.env` with a local-only `GRAFANA_ADMIN_USER` and a strong `GRAFANA_ADMIN_PASSWORD`.
3. Run the API with `API_ADDR=:18080`, the normal database configuration, and the token set. The bind address allows Docker Desktop's `host.docker.internal` bridge to scrape the local API; restrict host firewall access to the local development machine/network.
4. From the repository root, run `docker compose --env-file infrastructure/monitoring/.env -f infrastructure/monitoring/compose.yaml up -d`.
5. Open Prometheus at `http://localhost:9090` and Grafana at `http://localhost:3000`. Grafana provisions the Prometheus datasource and the Aramya API dashboard on startup.

If Docker is unavailable, scrape `http://localhost:18080/metrics` with an Authorization bearer header using a local Prometheus installation configured from `infrastructure/monitoring/prometheus.yml`.

## Metrics currently backed by real events

- `aramya_http_requests_total{method,route,status}` and `aramya_http_request_duration_seconds{method,route}` record all API traffic using Gin route templates, never raw paths.
- `aramya_database_ready` is updated on each `/ready` database ping.
- `aramya_order_creation_attempts_total{outcome}` records order creations, idempotent replays, business rejections, and server failures.
- `aramya_inventory_reservation_attempts_total{outcome}` records tracked-stock reservations as `reserved`, stock rejections as `rejected`, and reservation database/ledger failures as `failed`. Outcomes are fixed labels; IDs and SQL error text are never labels. A failure alert is provisioned in `infrastructure/monitoring/alerts.yml`.
- `aramya_payment_webhook_events_total{outcome}` records accepted events, duplicates, invalid signatures/payloads, conflicting event IDs, missing headers, oversized bodies, and processing failures. All outcomes are fixed labels; event IDs, payment data, and error messages are excluded. Processing failures alert through `AramyaPaymentWebhookProcessingFailures`.
- Go runtime metrics are also available from the process collector.

R2 and database slow-query metrics are not emitted yet because those corresponding instrumented operations are not implemented. Add counters/timers alongside those real operations when those milestones are built; do not infer success from request counts.

## Alert triage

- **High server-error rate:** check structured API logs using the request ID and route; distinguish client validation from 5xx errors before rollback.
- **Database not ready:** verify PostgreSQL health, connection limits, credentials/configuration, and migrations. Do not repeatedly restart a database that is still recovering.
- **High p95 latency:** inspect the route-level dashboard, database load, lock waits, and pool saturation. Correlate with request IDs; metrics contain no PII.

Prometheus alert rules are in `infrastructure/monitoring/alerts.yml`. The local stack does not configure an outbound alert receiver. Production alert delivery, private metrics ingress, GCP service monitoring, and operator paging still require deployment/account configuration.
