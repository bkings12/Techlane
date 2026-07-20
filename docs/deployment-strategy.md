# Deployment Strategy

## Stage 1 — Docker Compose

Single host or small VM:

- api-gateway, identity, repair, inventory-supplier, audit-risk, notification, worker
- postgres, redis, rabbitmq
- optional minio (S3-compatible) for local R2 substitute
- web-ops static nginx
- observability compose overlay: otel-collector, prometheus, grafana

## Principles

- Avoid Kubernetes until multi-host need is proven
- One Compose file for core; overlay for obs
- Migrations run as init jobs / service startup with lock
- Secrets via env files not committed
- Healthchecks on all services
- Rolling updates later; restart policies for stage 1

## Environments

| Env | Purpose |
|-----|---------|
| local | Developer compose |
| staging | Mirror prod providers (M-Pesa sandbox) |
| production | Hardened host(s), backups, TLS |

## Backups

- Daily Postgres dumps; retain per policy
- Object storage versioning if available

## Android distribution

- Internal APK / Play internal track
- Version gate against API min version

## Future

- Split Postgres by service when needed
- Managed Redis/MQ/Postgres
- K8s only with clear scaling/HA requirements

## Related

- [observability-plan.md](observability-plan.md)
