-- Publish existing POS demo accessories for online catalog (idempotent).
UPDATE inventory.products
SET online_visible = true, updated_at = now()
WHERE COALESCE(pos_visible, true) = true
  AND COALESCE(online_visible, false) = false;
