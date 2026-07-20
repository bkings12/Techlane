# Risk Register

| ID | Risk | Impact | Likelihood | Mitigation |
|----|------|--------|------------|------------|
| R1 | Scope explosion vs greenfield | High | High | Ruthless MVP; phase gates |
| R2 | Microservices ops overhead | Medium | High | Monorepo + Compose; few deployables |
| R3 | Offline sync duplicates money/stock | Critical | Medium | Idempotency keys; never LWW |
| R4 | Supplier friction causes shadow process | High | Medium | Fast auth-code UX; owner can collect |
| R5 | M-Pesa webhook unreliability | High | Medium | Query API fallback; pending states |
| R6 | Employees bypass system | High | Medium | Orphan-part alerts; credit reconciliation |
| R7 | Shared Postgres failure domain | Medium | Low | Backups; later split DBs |
| R8 | Auth token theft on Android | High | Low | Short TTL, secure storage, device bind |
| R9 | Premature commerce complexity | Medium | Medium | Schema readiness only until Phase 5 |
| R10 | Path/name confusion TechLane vs Techlane/megamart | Medium | High | Canonical repo only TechLane; megamart reference |
| R11 | No git early | High | High | Init git in Phase 0 |
| R12 | Audit loss on overload | High | Low | Durable queue; DLQ |
| R13 | Incorrect stock across channels | Critical | Medium | Reservations; single movement ledger |
| R14 | Hiring/skills mismatch (Go+Compose) | Medium | Medium | Clear docs; generated clients |

## Open operational risks

Track in [open-questions.md](open-questions.md) when decisions pending.
