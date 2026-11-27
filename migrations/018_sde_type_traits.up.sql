-- Add structured ship/module traits (hull bonuses) JSON column populated from CCP SDE types.jsonl
ALTER TABLE sde_types
    ADD COLUMN IF NOT EXISTS traits_json TEXT NOT NULL DEFAULT '';
