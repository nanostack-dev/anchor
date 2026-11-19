-- failing_since dates the first failed call of the current streak and
-- last_failed_at its latest one. A success or a save of the endpoint clears
-- both. The worker retires a 410 Gone endpoint only after a long streak, and
-- skips dialing an endpoint that failed moments ago.
ALTER TABLE product_event_endpoint_configs
    ADD COLUMN failing_since TIMESTAMPTZ,
    ADD COLUMN last_failed_at TIMESTAMPTZ;
