# SMTP integration verification lifecycle

Outbound SMTP integration instances implement `provider.ConnectionVerifier` and must pass a live connection check before reaching `ACTIVE`.

## State transitions

- **Create**: a new SMTP instance is saved as `CONFIGURING` (`is_enabled: true`) and an asynchronous verification run dials and authenticates against the configured host. On success it transitions to `ACTIVE`; on failure it transitions to `ERROR` with a sanitized `last_error`.
- **Config update (`config`)**:
  - When the instance is enabled, every config save sets status to `CONFIGURING`, clears `last_error`, and starts a fresh verification run before returning to `ACTIVE` or `ERROR`.
  - When the instance is disabled (`is_enabled: false`, including `{is_enabled: false, config}` in one request), the config is validated and saved, status stays `INACTIVE`, and no verification runs.
- **Enable / disable (`is_enabled`)**:
  - Setting `is_enabled: false` sets status to `INACTIVE` and cancels any in-flight verification on the handling replica.
  - Setting `is_enabled: true` on any non-`ACTIVE` SMTP instance (`INACTIVE`, `ERROR`, or `CONFIGURING`) sets status to `CONFIGURING`, clears `last_error`, and starts a fresh verification run. Sending `is_enabled: true` on an `ERROR` instance acts as an explicit verification retry.
- **Webhook secret (`webhook_secret`)**: SMTP does not implement `provider.WebhookIngestor`. Sending any non-null `webhook_secret` is refused with `400 INTEGRATION_WEBHOOK_SECRET_UNSUPPORTED` and never mutates status.

## Stale-run guard across replicas

`verifyAndActivate` (`apps/anchor/internal/service/integration_service.go`) persists its outcome through `IntegrationInstanceRepository.ApplyVerificationResult`, which updates only `status`, `last_error`, and `updated_at` conditioned on:

```sql
WHERE id = $1
  AND platform_tenant_id = $2
  AND status = 'CONFIGURING'
  AND is_enabled IS TRUE
  AND updated_at = $3 -- updated_at of the snapshot passed to this run
```

If an instance is deleted, disabled, or updated to a newer config on any replica while a verification run is in flight, the stale run matches zero rows and leaves the newer row untouched. Any `UpdateInstance` write that leaves an enabled `ConnectionVerifier` in `CONFIGURING` (including an empty `{}` update while verification is in flight) starts a fresh verification run keyed to the new `updated_at` so the instance cannot strand in `CONFIGURING`.
