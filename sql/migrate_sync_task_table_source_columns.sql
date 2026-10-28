-- Freeze the source-column whitelist captured by the latest successful task precheck.
-- Run this against the MergeWong PostgreSQL metadata database before deploying the code.
ALTER TABLE public.sync_task_tables
    ADD COLUMN IF NOT EXISTS source_columns json;

COMMENT ON COLUMN public.sync_task_tables.source_columns IS
    'Source-column whitelist captured by the latest successful precheck; new source fields are ignored until the task is saved and checked again.';
