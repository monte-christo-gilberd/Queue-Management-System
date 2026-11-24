# Database migrations

The PostgreSQL schema of the Queue Management System, split by domain. This folder is only for an **empty database**.
An existing database uses [`../scripts/apply_schema.sql`](../scripts/apply_schema.sql) instead: that file is idempotent and contains all recent schema changes (columns and table constraints, `notification_outbox` with `queue_id`, nullable `ticket_id`, `uq_outbox_kind_queue` and the `queue-closed` kind, plus the functions and triggers `tickets_notify`, `tickets_email_outbox`, `queues_email_outbox`, and the `push_devices`, `payment_orders` tables and their FK to `users`). Run it with `psql -v ON_ERROR_STOP=1 < db/scripts/apply_schema.sql` (the full dev and prod commands are in the header of that file), then restart `backend` and `notify-service`. Take a backup first in production.

## Order and file map

Docker runs the files in `/docker-entrypoint-initdb.d` in name order, and only when the data directory is empty
(`../Dockerfile`). Name order = dependency order.

| File | Contents | Depends on |
|---|---|---|
| `01a_accounts.sql` | `users`, `owners`, `payments` | - |
| `01b_queues.sql` | `queues`, `queue_settings`, `counters`, `queue_displays` | 01a |
| `01c_tickets.sql` | `tickets` (+ two unique indexes that enforce business rules), `ticket_notes` | 01a, 01b |
| `01d_history.sql` | `queue_history` (archive, no FK to live tables) | 01a |
| `01e_notifications.sql` | `push_subscriptions`, `notification_outbox` (`ticket_id` may be NULL, `queue_id` column, partial unique index `uq_outbox_kind_queue`, `kind` includes `queue-closed`) | 01a, 01b, 01c |
| `01f_profile_security.sql` | `user_avatars`, `user_action_tokens` | 01a |
| `01g_triggers.sql` | functions and triggers `tickets_notify`, `tickets_email_outbox`, `queues_email_outbox` (email to the owner when a queue changes from `active` to `closed`, following `users.email_notifications`) | 01a, 01b, 01c, 01e |
| `01i_payment_orders.sql` | `payment_orders` (BorderPay checkout orders; `status` pending, paid, expired, failed; `owner_id` references `users(id)`) | 01a |
| `01j_push_devices.sql` | `push_devices` (FCM tokens of Android devices; one token per row, moved to the account that logged in last) | 01a |
| `01h_role_grants.sql` | conditional `GRANT` blocks for the `mail_rw` role | 01a, 01e |
| `02_Indexes.sql` | all non-unique indexes, grouped by table | all `01*` |
| `03_notify_role.sh` | `notify_ro` role (notify-service) | all `01*` |
| `04_mail_role.sh` | `mail_rw` role (mail-service) | all `01*` |
| `06_auth_role.sh` | `auth_rw` role (auth-service): `users` and `user_action_tokens` only | all `01*` |
| `07_metrics_role.sh` | `metrics_ro` role (postgres_exporter): only `pg_monitor`, no access to data tables. Skipped when `METRICS_DB_PASSWORD` is empty; `make obs-up` runs it idempotently on an existing database | - |
| `08_payment_role.sh` | `payment_rw` role (payment-service): `owners` (SELECT/INSERT/UPDATE on subscription columns), `payment_orders` (SELECT, INSERT, UPDATE on status/pay_url/paid_at/updated_at columns), `payments` (INSERT). Also fixes the `payment_orders.owner_id` FK to `users(id)` in older databases. Skipped when `PAYMENT_DB_PASSWORD` is empty. There is no `05_*`; the privileges are checked by `../scripts/check_payment_grants.sql` | all `01*` |

Running manually on an empty database (without Docker):

```bash
createdb qms_db
for f in db/migrations/01*.sql db/migrations/02_Indexes.sql; do psql -d qms_db -v ON_ERROR_STOP=1 -f "$f"; done
```

## Rules

- The `01*` schema files are **not idempotent** (`CREATE` without `IF NOT EXISTS`). Running them twice fails.
  `02_Indexes.sql` is idempotent (`IF NOT EXISTS`).
- **Do not rename** tables, columns, constraints, or indexes without a migration. The same names are used by existing
  databases and by `apply_schema.sql`. Some names are not uniform (e.g. `user_action_tokens_user_idx` without the `idx_`
  prefix) for that reason.
- Non-unique indexes belong only in `02_Indexes.sql`. `PRIMARY KEY` and `UNIQUE` constraints, and the two partial unique
  indexes that enforce business rules (`tickets_queue_id_user_id_active_key`, `tickets_one_serving_per_counter`), are in
  the files of their tables.
- The body of a trigger function is stored as-is in the Postgres catalog, **including comments inside the function body**.
  Changing a single character in the function body is a schema change and must also be applied to `apply_schema.sql`.
- The backend integration tests load all `01*.sql` files, then `02_Indexes.sql`, into a temporary schema (in name
  order). Do not use schema-qualified names (`public.xxx`) or database-level commands (`CREATE DATABASE`, `\connect`).
- The role scripts (`03..08`) grant their own privileges. Tables with sensitive data (`user_avatars`,
  `user_action_tokens`, `queue_displays`) are deliberately not granted to any role other than their owner, except
  `user_action_tokens` for `auth_rw`.

## Adding or changing the schema

1. Change the table in its domain file (or add a new file `01x_name.sql` with a letter that preserves dependency order;
   update the table above and the `COPY` list if needed). Add its indexes to `02_Indexes.sql`.
2. Add the same change in idempotent form to `../scripts/apply_schema.sql`.
3. If another service reads a new table or column, update the privileges in the relevant role script.
4. Prove that a new database and an upgraded database have the same catalog (compare `pg_dump --schema-only` of both).

## Limitations

- `apply_schema.sql` adds `tickets_serving_has_counter` as `NOT VALID`, then runs `VALIDATE CONSTRAINT` when there are no violations (tickets `currently_served` without a counter). If there are violations: a `WARNING` is raised and the constraint stays `NOT VALID`; fix the data, then validate manually.
- The body of `tickets_notify` in `apply_schema.sql` is now identical to `01g_triggers.sql` (it previously differed by one comment line).
- `01h_role_grants.sql` refers to the role scripts `03..05`, and the `payments` comment in `01a_accounts.sql` refers to an extend endpoint; both are outdated (the payment role is `08_payment_role.sh`, and `payments` is written by payment-service).
- `queue_history` exists but is not used by the Go code. There are no `updated_at` triggers.
- `../scripts/reset_seed_dev.sql` now also `TRUNCATE`s `payment_orders` (previously it did not, so `make seed` failed when order rows existed).
- The backend connects as the main user `qms`, not as a restricted role; restricted roles exist only for notify, mail, auth, metrics, and payment.

## Columns that exist but are not used by the code

`queue_settings`: `registration_limit`, `display_next_count`, `display_skipped_count`, `fallback_mode`,
`fallback_offset`, `fallback_max_attempts`, `last_rebalanced_at`. These features were removed from the product (31 Oct 2026)
or replaced by other tables. They are kept so the schema matches existing databases; removing them requires a migration
in `apply_schema.sql`.