-- Money as integer kobo (avoid float drift). Convert existing naira values ×100.
ALTER TABLE requests
    ALTER COLUMN amount_requested DROP DEFAULT,
    ALTER COLUMN max_amount       DROP DEFAULT,
    ALTER COLUMN offer_amount     DROP DEFAULT;

ALTER TABLE requests
    ALTER COLUMN amount_requested TYPE BIGINT USING round(amount_requested * 100)::bigint,
    ALTER COLUMN max_amount       TYPE BIGINT USING round(max_amount * 100)::bigint,
    ALTER COLUMN offer_amount     TYPE BIGINT USING round(offer_amount * 100)::bigint;

ALTER TABLE requests
    ALTER COLUMN amount_requested SET DEFAULT 0,
    ALTER COLUMN max_amount       SET DEFAULT 0,
    ALTER COLUMN offer_amount     SET DEFAULT 0;

-- Loan products store maxAmount inside JSONB; convert each ×100 to kobo.
UPDATE workspaces SET loan_products = COALESCE((
    SELECT jsonb_agg(jsonb_set(p, '{maxAmount}', to_jsonb((round((p->>'maxAmount')::numeric * 100))::bigint)))
    FROM jsonb_array_elements(loan_products) p
), '[]'::jsonb)
WHERE jsonb_array_length(loan_products) > 0;
