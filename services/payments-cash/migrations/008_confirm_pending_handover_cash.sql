-- Cash dual-control handover is retired: cash taken at the counter is final.
-- Backfill stuck payments so balances and collections treat them as settled.
-- Do NOT drop payments.cash_handovers here — that is a separate, backed-up cleanup.

UPDATE payments.payments
SET status = 'confirmed', updated_at = now(), version = version + 1
WHERE status = 'pending_handover';
