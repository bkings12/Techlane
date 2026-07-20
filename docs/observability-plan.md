# Observability Plan

## Pillars

1. **Traces** — OpenTelemetry across gateway and services  
2. **Metrics** — Prometheus  
3. **Logs** — Structured JSON (stdout) with correlation_id, tenant_id, actor_id  

## Trace propagation

- W3C traceparent through HTTP and RabbitMQ headers  
- Span per handler, DB query (sampled), external payment call  

## Key metrics

- Request rate/latency/error by route  
- Payment confirm latency and failure rate  
- Idempotency hit rate  
- Sync command success/reject/conflict  
- Queue depth and DLQ depth  
- Orphan part alert count  
- Cash shortage count  

## Logs

Never log secrets, full card data, or raw refresh tokens. Redact phone numbers partially where possible.

## Dashboards (Grafana)

- Service health  
- Payments pipeline  
- Inventory / risk alerts  
- Sync health  

## Alerts

- DLQ growth  
- Payment webhook failures  
- Error rate SLO burn  
- Orphan parts above threshold  

## Local

`deploy/docker-compose.obs.yml` optional profile.
