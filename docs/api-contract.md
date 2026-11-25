# API Contract — Queue Management System

The HTTP and SSE contract between the frontend and the backend. The source of truth is the backend code
(`backend/internal/router`, `backend/internal/handler`, `backend/internal/dto`); this document summarizes the
request and response shapes that the frontend uses.

## Summary

| Item | Value |
|---|---|
| Base path | `/api/v1` |
| Format | JSON, `Content-Type: application/json` |
| Field names | `snake_case` |
| Empty fields | `null` (not an empty string or `0`) |
| Authentication | `Authorization: Bearer <token>` (RS256 JWT issued by `auth-service`, valid for 24 hours, claims `sub` = user id, `iat`). Verified locally with the public key |
| Time | RFC 3339 (`2026-10-17T16:29:57.635239+07:00`) |

## Response envelope

Success: an object with a `data` key.

```json
{ "data": { "action": "list_queues", "queues": [ ... ] } }
```

Failure: an object with an `error` key that contains `code` (a fixed string) and `message` (text for humans).

```json
{ "error": { "code": "NOT_FOUND", "message": "queue not found" } }
```

Some endpoints (join, next) return the ticket object directly inside `data`, without `action`.

## Error codes

| HTTP | `code` | Meaning |
|---|---|---|
| 400 | `BAD_REQUEST` | Invalid JSON body, a required field is empty, or `queue_id` is not a number |
| 401 | `UNAUTHORIZED` | Token missing, invalid, expired, or the user was not found |
| 402 | `SUBSCRIPTION_INACTIVE` | The owner's subscription is not active (creating, changing, or setting queue permissions) |
| 403 | `EMAIL_NOT_VERIFIED` | The account's email is not verified (creating a queue, or joining with a new number). Only applies when email verification is enabled |
| 404 | `NOT_FOUND` | The queue does not exist, or the requester has no right to see it |
| 409 | `CONFLICT` | Email or username is already registered (register) |
| 409 | `QUEUE_CLOSED` | The queue has been closed (join) |
| 409 | `QUEUE_EMPTY` | No ticket is waiting (next) |
| 409 | `ALREADY_JOINED` | Joining again (active ticket) returns the existing ticket with 200, so `POST /join` never answers with this code even though the handler maps it; this code only appears in `insert` (the account already has an active ticket) |
| 409 | `AMBIGUOUS_ACCOUNT` | The username used in `insert` belongs to more than one account |
| 409 | `NOTHING_TO_SKIP` / `NOTHING_TO_RECALL` | The counter is not serving any number |
| 409 | `RECALL_TOO_SOON` / `ALREADY_RECALLED` | Recall is not allowed yet (less than 3 seconds) or was already used |
| 409 | `COUNTER_STATE_CHANGED` | Next/Skip/Recall: the counter's number differs from `expected_ticket_id` (the counter was just advanced from another device); no data was changed |
| 409 | `DUPLICATE_COUNTER_NAME` | The counter name is already used in that queue |
| 409 | `COUNTER_LIMIT` | The queue already has 10 counters |
| 409 | `DEFAULT_COUNTER` | The default counter cannot be deleted |
| 400 | `WEAK_PASSWORD` | Password is outside 8 characters to 72 bytes |
| 400 | `INVALID_TOKEN` | Password reset or email verification token does not exist, has expired, or was already used |
| 429 | `TOO_MANY_REQUESTS` | Resending the verification email exceeds 3 per hour per account |
| 429 | `TOO_MANY_DISPLAYS` | Too many public display connections for one queue |
| 503 | `BUSY` | Too many forgot-password emails are being processed |
| 413 | `PAYLOAD_TOO_LARGE` | Profile photo is larger than 5 MB |
| 415 | `UNSUPPORTED_MEDIA_TYPE` | Profile photo is not JPEG, PNG, or WebP |
| 503 | `SERVICE_UNAVAILABLE` | An SSE connection (queue or public display) was rejected because the hub is stopped or full |
| 409 | `QR_REPLACE_NOT_ALLOWED` | Replacing the QR code is only allowed in static mode |
| 400 | `QR_TYPE_INVALID` | The QR `type` is not `static` or `dynamic` |
| 404 | `QR_INVALID` | The join code is invalid, has been replaced, or the queue does not exist |
| 410 | `QR_EXPIRED` | The dynamic join code has expired |
| 500 | `INTERNAL_ERROR` | Server error |
| 503 | `RETRY_EXHAUSTED` | Join failed after several attempts because of many concurrent requests |

Note: 404 is also used for "not yours". This is intentional so that the existence of other people's queues does not leak.


## Authentication

### POST /api/v1/users/register
No token.

Request:
```json
{ "email": "owner@example.com", "username": "owner1", "password": "secret123" }
```
All fields are required. `email` and `username` must not contain HTML tags. `password` is 8 characters to 72 bytes (the bcrypt limit).

200:
```json
{ "data": { "token": "eyJhbGciOi..." } }
```
Errors: 400 `BAD_REQUEST`, 400 `WEAK_PASSWORD` (outside 8 characters to 72 bytes), 409 `CONFLICT`, 500 `INTERNAL_ERROR`.

When email verification is enabled, the new account is immediately sent a verification link in the background (a send failure is only logged; registration still returns 200). See `POST /users/verify-email`.

The token contains the `sub`, `iat`, and `exp` claims (24 hours). A token whose `iat` is older than the user's `password_changed_at` is rejected with 401 (see password reset below). A token without `iat` (issued before this feature) remains valid until `exp`.

### POST /api/v1/users/login
No token. Accepts an email or a username in `identifier`. The `email` field is still accepted as an alias.

Request:
```json
{ "identifier": "owner@example.com", "password": "secret123" }
```

200: `{ "data": { "token": "..." } }`
Errors: 400 `BAD_REQUEST` (identifier and email both empty), 401 `UNAUTHORIZED` (wrong credentials; the message does not say which part was wrong).

### POST /api/v1/users/google
No token. Exchanges a **Google ID token** for a QMS login token. Served by `auth-service`, and only available when `GOOGLE_CLIENT_IDS` is set (otherwise 404). The client obtains the ID token from Google (web: `expo-auth-session`; native: development build).

Request: `{ "id_token": "eyJhbGciOi..." }` (required, at most 8192 characters).

The server verifies the ID token itself: the RS256 signature with Google's public keys, the `iss` from Google, `aud` matching one of `GOOGLE_CLIENT_IDS`, `exp`/`iat` (60-second tolerance), and `email_verified`. The email and `sub` sent by the client are never trusted. Accounts are matched by `google_id` (the `sub` claim), not by email.

| Condition | Result |
|---|---|
| Account is already linked to this `google_id` | **200** `{ "data": { "token": "..." } }` |
| Email is not registered yet | A new account is created without a password, with `email_verified_at` set (Google already guarantees it), and a username taken from the Google name (at most 50 characters, without `<` `>`). **200** with a token |
| Email is registered **with a password** (case-insensitive) | **409** `GOOGLE_LINK_REQUIRED`. The account is **not** linked automatically (this prevents account takeover through email). It includes `error.link_token` |
| Email is registered but the account is already linked to another Google account, or has no password | 409 `CONFLICT` |

409 `GOOGLE_LINK_REQUIRED`:
```json
{ "error": { "code": "GOOGLE_LINK_REQUIRED", "message": "...", "link_token": "eyJhbGciOi..." } }
```

Other errors: 400 `BAD_REQUEST` (malformed body), 401 `UNAUTHORIZED` (invalid ID token), 403 `GOOGLE_EMAIL_UNVERIFIED` (Google does not vouch for the email), 503 `GOOGLE_UNAVAILABLE` (Google's public keys cannot be fetched), 503 `AUTH_UNAVAILABLE` (from nginx when auth-service is down). The response includes `Cache-Control: no-store`. Rate limited per IP in nginx (`auth_zone`).

### POST /api/v1/users/google/link
No token. Links a Google account to an existing account after its owner proves the password.

Request: `{ "link_token": "...", "password": "secret123" }` (`link_token` comes from the 409 above).

200: `{ "data": { "token": "..." } }`. The account is linked to that `google_id` and its email is marked verified. The old password still works, so the account can log in with either.
Errors: 400 `INVALID_TOKEN` (the link token is invalid, expired, already used, or the password was wrong 5 times), 401 `UNAUTHORIZED` (wrong password; same as a failed login), 409 `CONFLICT` (the account is already linked to another Google account), 503 `AUTH_UNAVAILABLE`.

**Link token:** a 5-minute RS256 JWT with the claims `purpose: "google_link"`, `gid` (the Google id), and a random `jti`. Single use; after 5 wrong passwords the token is burned. It is signed with the same key as login tokens and contains `sub`, but it is **not a login token**: `auth-service` and the backend reject any token with a `purpose` (401). Do not store it in storage and do not log it.

### POST /api/v1/users/forgot-password
No token. Requests a link to set a new password. The link is sent to the account's email through `mail-service` (template `password-reset`).

Request: `{ "email": "budi@example.com" }`

**202** with the same message for every well-formed email, whether registered or not. The sending happens asynchronously, so the response does not reveal whether an account exists:
```json
{ "data": { "message": "If the email is registered, a link to set a new password has been sent." } }
```
Errors: 400 `BAD_REQUEST` (malformed body, empty or invalid email, contains HTML tags), 503 `BUSY` (too many sends in progress), 429 from nginx (1 request per second per IP, burst 5; no JSON body).

Rules: a random 32-byte token (`crypto/rand`, base64url), stored only as a SHA-256 hash in `user_action_tokens`; valid for 30 minutes and single use; a new request cancels older unused tokens; at most 3 tokens per account per hour (extra requests are silently not sent but still get 202). Link: `${PUBLIC_APP_URL}/reset-password?token=...`. Email sending errors are only written to the server log.

### POST /api/v1/users/reset-password
No token. Sets a new password from the email link.

Request: `{ "token": "<from the link>", "password": "newpassword1" }`

200: `{ "data": { "status": "ok" } }`. The password is bcrypt-hashed, `users.password_changed_at` is set (all older sessions are revoked), and the token is marked used together with the user's other reset tokens. All in one transaction.

Errors: 400 `WEAK_PASSWORD` (the password is checked **before** the token is touched, so the token is not consumed), 400 `INVALID_TOKEN` (does not exist, expired, or already used; one message for all three), 400 `BAD_REQUEST`, 429 from nginx.

### POST /api/v1/users/verify-email
No token (the credential is the token in the email link). Served by `auth-service`. This route, together with `forgot-password`, `reset-password`, and `me/verify-email/resend`, is only registered when `MAIL_SERVICE_URL` is set. In auth-service, an empty `EMAIL_VERIFICATION_REQUIRED` means "follow whether `MAIL_SERVICE_URL` is set", and `true` without mail-service makes auth-service fail to start. In the backend, an empty value means `true` (the guard is on). The value must match in both services.

Request: `{ "token": "<from the link>" }`

200: `{ "data": { "status": "ok" } }` (header `Cache-Control: no-store`). Sets `users.email_verified_at` and marks all of that user's verification tokens as used, in one transaction.

Errors: 400 `INVALID_TOKEN` (does not exist, expired, or already used; one message for all three), 400 `BAD_REQUEST`.

Confirmation email: if this token turns an account that was **not** yet verified into a verified one, `auth-service` sends the `email-verified` template through `mail-service`. Delivery is best effort and asynchronous: a failure is only logged and never changes the 200 response above. Nothing is sent if the account was already verified (for example through Google) or if `MAIL_SERVICE_URL` is empty.

Rules: a single-use token valid for 24 hours; only its SHA-256 hash is stored in `user_action_tokens` (`purpose = 'email_verify'`). Link: `${PUBLIC_APP_URL}/verify-email?token=...`; the email is sent through `mail-service` (template `email-verify`).

### POST /api/v1/users/me/verify-email/resend
Requires a token. Resends the verification link. 202 `{ "data": { "message": "If the email is not verified yet, a verification link has been sent." } }`, also when the account is already verified (nothing is sent). Older unused tokens are cancelled. Errors: 429 `TOO_MANY_REQUESTS` (at most 3 per hour per account), 401 `UNAUTHORIZED`.

While the account is not verified: `POST /queues` and `POST /join` (only for a new number; an existing ticket is still returned) answer 403 `EMAIL_NOT_VERIFIED`. The status is read from the database on each request, so it takes effect without logging in again.

### GET /api/v1/users/me
Requires a token. A token issued before the last password change is answered with 401 `UNAUTHORIZED`.

200:
```json
{ "data": { "id": 1, "email": "owner@example.com", "username": "owner1", "is_owner": true, "subscription_expires_at": "2026-12-04T02:01:06Z", "avatar_updated_at": null, "email_notifications": true, "email_verified": true } }
```
`email_verified` = the email has been verified; the client shows a verification banner on the Profile page when it is `false`. It is always `true` when email verification is turned off. `email_notifications` = ticket email notifications are on (default `true`). `is_owner` = true when the user has a row in the `owners` table. `subscription_expires_at` is only set for owners (`null` for customers). `avatar_updated_at` = the time the profile photo was last changed, `null` when there is no photo (the client uses it as a photo indicator and as a cache breaker).

### PUT /api/v1/users/me
Requires a token. Changes the username of the logged-in user. The email cannot be changed here (other fields are ignored).

Body:
```json
{ "username": "New Name" }
```
Leading and trailing spaces are removed. Usernames do not have to be unique.

200: the latest profile, in the same shape as `GET /users/me`.

Errors: 400 `BAD_REQUEST` (empty, more than 50 characters, or contains HTML tags such as `<` and `>`), 401 `UNAUTHORIZED` (no token or an invalid token).

### PUT /api/v1/users/me/preferences
Requires a token. Turns ticket email notifications on or off (ticket events: joined, cancelled, removed by the owner, finished being served; owner events: a queue they own is closed). It applies to future events only: emails already in the queue are still sent, and emails missed while the switch was off are not sent later.

Body:
```json
{ "email_notifications": false }
```
200: the latest profile, in the same shape as `GET /users/me`. Errors: 400 `BAD_REQUEST` (the field is missing, `null`, or not a boolean), 401 `UNAUTHORIZED`.

### PUT /api/v1/users/me/avatar
Requires a token. Uploads or replaces the profile photo. `multipart/form-data` with a field named `file`.

The server determines the type from the **content of the file** (not from the client's name or headers). Accepted: JPEG, PNG, WebP, at most 5 MB and 24 megapixels. The result is cropped to a square at the center, resized to 512x512 (smaller photos are not enlarged), and stored as JPEG (at most 150 KB). Re-encoding removes EXIF and other metadata, including GPS location. Transparent areas become white. One photo per user; a new upload replaces the old one.

200: the latest profile, in the same shape as `GET /users/me` (`avatar_updated_at` is set).

Errors: 400 `BAD_REQUEST` (the `file` field is missing, or the file is corrupt or its dimensions are too large), 413 `PAYLOAD_TOO_LARGE` (more than 5 MB; when rejected by nginx the body is HTML and not JSON), 415 `UNSUPPORTED_MEDIA_TYPE` (not JPEG, PNG, or WebP), 401 `UNAUTHORIZED`. Rate limited to 5 requests per second per IP (nginx).

### GET /api/v1/users/me/avatar
Requires a token. Returns the photo of the **caller only**; there is no public URL and no route to view another user's photo. 200 `image/jpeg` with `ETag`, `Cache-Control: private, no-cache`, and `X-Content-Type-Options: nosniff`; send `If-None-Match` to get 304. 404 `NOT_FOUND` when there is no photo yet. Because the `Authorization` header is needed, the web client fetches the photo and turns it into an object URL (it does not use `<img src>` directly).

### DELETE /api/v1/users/me/avatar
Requires a token. Deletes the profile photo. Idempotent: 204 with no body, also when there is no photo.

### GET /api/v1/users/me/tickets
Requires a token. The user's tickets across all queues.

200:
```json
{
  "data": {
    "action": "list_my_tickets",
    "tickets": [
      {
        "id": 121, "queue_id": 122, "ticket_number": 2, "ticket_prefix": "A", "sort_key": 131072, "user_id": 3,
        "status": "waiting", "skip_count": 0, "last_notified_at": null,
        "created_at": "2026-10-17T16:29:57+07:00", "completed_at": null, "cancelled_at": null,
        "queue_name": "Counter A", "queue_status": "active", "current_number": 1
      }
    ]
  }
}
```
`current_number` = the `ticket_number` being served in that queue (the `currently_served` ticket), or null.
One row per queue: when the user has an active ticket (`currently_served` or `waiting`) in that queue, the active ticket is shown; otherwise the most recent ticket.
`ticket_prefix` = the number prefix of the queue (not a column of `tickets`). The display label = `ticket_prefix` + `ticket_number`, for example `A2`.

### GET /api/v1/users/me/history?limit=&cursor=
Requires a token. The ticket history of the logged-in user (customer), one row per ticket, newest first. There is no user parameter: only tickets belonging to the token are returned. Paging is the same as the owner's history (`cursor` is opaque, `limit`; see the queue history section).

200:
```json
{ "data": { "action": "list_my_history",
  "tickets": [ { "ticket_id": 56, "queue_id": 54, "queue_name": "Counter", "ticket_prefix": "A", "ticket_number": 3,
    "outcome": "completed", "queue_closed": true, "skip_count": 0,
    "joined_at": "...", "called_at": "...", "completed_at": "...", "cancelled_at": null, "counter_name": null } ],
  "next_cursor": null } }
```
Deliberately without owner notes and without other users' data.

## Queues (owner)

Every endpoint in this section requires a token. Except for `GET /queues/:id` (also for ticket holders) and `GET /queues/:id/status`,
only the queue's owner may use them.

### POST /api/v1/queues
Creates a queue. Requires an active owner subscription.

Request:
```json
{ "queue_name": "Counter A", "queue_description": "General service", "ticket_prefix": "A" }
```
`queue_name` is required. `queue_description` may be empty. `ticket_prefix` is optional: 1 to 3 letters or digits (lowercase is converted to uppercase); empty = `A`. Anything else is 400 `BAD_REQUEST`. A number is displayed as `<prefix>-<3-digit number>`, for example `VIP-007`.

201:
```json
{ "data": { "action": "create_queue", "queue": { ...QueueResponse } } }
```
A new queue automatically gets one default counter and one `queue_settings` row.

Errors: 400 `BAD_REQUEST`, 403 `EMAIL_NOT_VERIFIED` (checked before the subscription), 402 `SUBSCRIPTION_INACTIVE`, 500.

### GET /api/v1/queues
The owner's queues. When the subscription is not active, only queues that are still active are returned (there is no 402).

200: `{ "data": { "action": "list_queues", "queues": [ ...QueueResponse ] } }`

### GET /api/v1/queues/:queue_id
Queue details. The owner gets the full `QueueResponse`; a ticket holder in that queue gets a `PublicQueueResponse` (without `owner_id`). Other users: 404. (The old `GET /queues/:id/info` endpoint no longer exists; this endpoint replaces it.)

200: `{ "data": { "action": "get_queue", "queue": {...} } }`
Errors: 400, 404 `NOT_FOUND`.

### PUT /api/v1/queues/:queue_id
Changes the name and description. Requires an active subscription and owner rights.

Request: `{ "queue_name": "Counter B", "queue_description": "...", "ticket_prefix": "B" }` (`ticket_prefix` is optional; empty = unchanged; the same rules as creating a queue. Changing it changes the displayed label of every number in that queue, including numbers already issued.)
200: `{ "data": { "action": "update_queue", ... } }`
Errors: 400, 402 `SUBSCRIPTION_INACTIVE`, 404.

### POST /api/v1/queues/:queue_id/close
Closes the queue. It cannot be reopened. Customers cannot join after it is closed. Numbers that are still `currently_served` become `completed` (`completed_at` is set) in the same transaction, so no ticket is left hanging. `closed_at` is set to the closing time; queues closed before this fix were given an estimate by `db/scripts/apply_schema.sql` (the last ticket activity, or `created_at` when there were no tickets).

200: `{ "data": { "action": "close", ... } }`
Errors: 400, 404.

Side effect: when the owner's `users.email_notifications` is on, a database trigger writes one `queue-closed` outbox row (at most one per queue) and `mail-service` sends the "queue closed" email to the owner. Numbers being served that become `completed` still trigger the "finished being served" email for their holders, according to each person's own switch. A send failure does not affect the response.

### POST /api/v1/queues/:queue_id/next
Calls the first `waiting` ticket and makes it `currently_served` (the number being served) at a counter. The number that counter was serving before automatically becomes `completed` (`completed_at` is set) in the same transaction. Owner only.

Optional body (also for `skip` and `recall`): `{ "counter_id": 0, "expected_ticket_id": 121 }`. `counter_id` 0 or empty = the default counter; any other counter must belong to this queue. `expected_ticket_id` = the id of the number the device believes that counter is serving (0 = empty counter). When it differs from the real state, the response is 409 `COUNTER_STATE_CHANGED` with no data changed (this prevents two devices from advancing the same counter at once). Without this field (older clients) no check is made. An empty or malformed body is treated as the default counter.

200: the ticket object directly inside `data`, with `status` = `"currently_served"`.
```json
{ "data": { "id": 121, "queue_id": 122, "ticket_number": 2, "status": "currently_served",
            "called_counter_id": 11, "called_counter_name": null, "called_at": "...", ... } }
```
Errors: 404 `NOT_FOUND` (not the owner), 409 `QUEUE_EMPTY` (nobody is waiting), 409 `COUNTER_STATE_CHANGED`.

### GET /api/v1/queues/:queue_id/settings/skip
The queue's Skip settings. Owner only. Response: `{ "data": { "fallback": "remove|back|after_n", "n": null|int, "max_returns": int } }`. The default is `remove`, `n` null, `max_returns` 1.

### PUT /api/v1/queues/:queue_id/settings/skip
Body `{ "fallback": "remove|back|after_n", "n": int, "max_returns": int }`. Only the owner with an active subscription, for a queue that is not closed. `n` is required and ≥ 1 for `after_n`, and ignored for other modes. `max_returns` ≥ 1: how many times one ticket may be returned before it becomes permanently `skipped`.
- `remove`: the number becomes `skipped` immediately.
- `back`: returns to `waiting` at the very end of the queue.
- `after_n`: returns to `waiting` after N tickets that are still waiting (counted after the next ticket is called); if fewer than N are waiting, it goes to the very end. When the `sort_key` gap is exhausted, waiting tickets are renumbered automatically.
Errors: 400 `BAD_REQUEST` (unknown mode, N or `max_returns` below 1), 402 `SUBSCRIPTION_INACTIVE`, 403 (not the owner), 409 `QUEUE_CLOSED`, 404 `NOT_FOUND`.

### POST /api/v1/queues/:queue_id/recall
The owner calls again the number a counter is serving (the body is the same as `next`). It sends a push only to that number's owner (the status does not change). Available 3 seconds after the number was called (Next/Skip), and only once per call. 409 errors: `NOTHING_TO_RECALL`, `RECALL_TOO_SOON`, `ALREADY_RECALLED`, `COUNTER_STATE_CHANGED`. Not the owner of the queue: 404.

### GET /api/v1/queues/:queue_id/tickets/:ticket_id/note
The owner's note for one ticket number. Only the owner of the queue. Used when the Append Note popup opens, so an existing note is pre-filled.

200: `{ "data": { "ticket_id": 121, "note": "...", "created_at": "...", "updated_at": "..." } }`. A number without a note: `note` = null and the dates are null (not 404).
Errors: 404 `NOT_FOUND` (not the owner, or the ticket does not belong to this queue).

### PUT /api/v1/queues/:queue_id/tickets/:ticket_id/note
Creates the note if it does not exist, or overwrites it if it does (at most one note per number). Body `{ "note": "..." }`. Leading and trailing spaces are removed.

201: a new note was created. 200: the existing note was overwritten. The response is the same as for GET.
Errors: 400 `BAD_REQUEST` (the note is empty after trimming, or longer than 1000 characters; the old note is unchanged), 404 `NOT_FOUND` (not the owner, or the ticket does not belong to this queue).

Notes are never included in ticket responses and are never visible to customers.

### POST /api/v1/queues/:queue_id/insert
The owner inserts a registered account at a specific position in the waiting order (US-10 / 4.12). Only the owner of the queue.

Body: `{ "identifier": "email or username", "position": 2 }`. `identifier` is trimmed and required. `position` is an integer of at least 1, counted over `waiting` tickets (1 = next). A position beyond the length of the queue is placed at the very end.

201: the new ticket object (`status` = `"waiting"`), with the next `ticket_number` as in a normal join. The number currently being served is not disturbed.

Errors:
- 400 `BAD_REQUEST`: `identifier` is empty, `position` is less than 1, or the body is invalid.
- 404 `NOT_FOUND`: not the owner, the queue does not exist, or the account is not found.
- 409 `AMBIGUOUS_ACCOUNT`: more than one account uses that username. Use the email instead.
- 409 `ALREADY_JOINED`: the account already has an active ticket in this queue.
- 409 `QUEUE_CLOSED`.
- 503 `RETRY_EXHAUSTED`: failed after several attempts, usually because the queue is very busy.

An account whose earlier ticket has finished may be inserted again, with a new number.

### POST /api/v1/queues/:queue_id/cancel
The customer cancels their own number (4.9). Customer login, no body. The caller's active ticket (`waiting` or `currently_served`) is cancelled; if that number was being served, the first waiting ticket moves up to `currently_served`. The number is not deleted: its status becomes `cancelled`, and the user may join again with a new number.

200: the ticket object after cancellation (`status` = `"cancelled"`, `cancelled_at` is set). With no active ticket, or if it was already cancelled: 200 with the last ticket state (a no-op, so a double click is safe).
Errors: 404 `NOT_FOUND` (the queue does not exist, or the user has never had a ticket in that queue), 409 `QUEUE_CLOSED`.

### POST /api/v1/queues/:queue_id/tickets/:ticket_id/remove
The owner removes a customer's number from the queue (4.10). Only the owner of the queue. It behaves like a skip without fallback: the number leaves the waiting order and is not returned to `waiting`; `skip_count` does not increase. When the number being served is removed, the first waiting ticket moves up to `currently_served`. The final status is `deleted`, and the number does not appear in the `skipped` list.

200: the ticket object after removal (`status` = `"deleted"`). A ticket that is no longer active (`skipped`, `cancelled`, `deleted`, `completed`): 200 with the current ticket state (a no-op).
Errors: 404 `NOT_FOUND` (not the owner, or the ticket belongs to another queue), 409 `QUEUE_CLOSED`, 400 `BAD_REQUEST` (invalid id).

A push notification to the customer for removal does not exist yet; it will be added in a later task.

### Web Push (login required)
- `GET /api/v1/push/public-key` → `{ public_key }` (the public VAPID key).
- `POST /api/v1/push/subscribe` with body `{ endpoint, keys: { p256dh, auth } }`; the endpoint must be `https`, otherwise 400. The same endpoint is moved to the currently logged-in user.
- `POST /api/v1/push/unsubscribe` with body `{ endpoint }`.
The `/push` routes only exist when `VAPID_PUBLIC_KEY` is set. Pushes are sent by `notify-service` (LISTEN `queue_events`): a number is called → the owner gets "Your turn" and the first waiting ticket gets "1 number left"; recall → only the owner; skip → the owner of the skipped number ("skipped" or "skipped and returned to the queue").

### Android app push / FCM (login required)
- `POST /api/v1/push/fcm/register` with body `{ token: string, platform: "android" }` → `204` with no body. `token` is the device's FCM token (`getDevicePushTokenAsync` in expo-notifications). The same token is moved to the currently logged-in user (the same behavior as the Web Push endpoint). `400 BAD_REQUEST` when the token is empty or `platform` is not `android`.
- `POST /api/v1/push/fcm/unregister` with body `{ token: string }` → `204` with no body. It only deletes a token belonging to the logged-in user; tokens of other accounts are not touched. `400 BAD_REQUEST` when the token is empty.
The `/push/fcm` routes only exist when `FCM_PROJECT_ID` is set in the backend. The notification content and triggers are the same as for Web Push; `notify-service` sends through FCM HTTP v1 when `FCM_CREDENTIALS_FILE` is configured, and deletes tokens that FCM answers with `UNREGISTERED`. Tapping the notification opens `data.url` (for example `/ticket/12`).

### POST /api/v1/queues/:queue_id/skip
Skips the number the counter is serving (a `currently_served` ticket; the body is the same as `next`) and then immediately calls the first `waiting` ticket to the same counter, in one transaction. Owner only. `skip_count` +1 and `completed_at` is cleared. What happens to the skipped ticket follows the queue's Skip settings (see `settings/skip`): `remove`, or reaching the `max_returns` limit, makes it permanently `skipped` (`called_at` stays); `back`/`after_n` returns it to `waiting` at a new position (`called_at` and `called_counter_id` are cleared).

200: the newly called ticket object (the same as Next), `status` = `"currently_served"`.
Errors: 404 `NOT_FOUND` (not the owner), 409 `NOTHING_TO_SKIP` (no number is being served yet), 409 `QUEUE_EMPTY` (nobody is waiting; no change, the number stays served), 409 `COUNTER_STATE_CHANGED`.

### GET /api/v1/queues/:queue_id/tickets
All tickets of the queue, ordered by `sort_key`. Only the owner. At most 500 rows.

Optional query: `status` (`waiting`, `currently_served`, `completed`, `cancelled`, `skipped`, `deleted`), which may be repeated or comma-separated. `deleted` = a number removed by the owner (4.10).

200:
```json
{ "data": { "action": "list_tickets", "queue_id": 122, "tickets": [ ...OwnerTicketResponse ] } }
```
`OwnerTicketResponse` = `TicketResponse` plus `note` and `note_updated_at` (the owner's note; null when there is none). This is the only endpoint that includes notes.
Errors: 400, 404.

### GET /api/v1/queues/:queue_id/qr
The queue's QR status for the owner. A static token is created lazily the first time it is requested.

200:
```json
{
  "data": {
    "action": "queue_qr",
    "queue_id": 122,
    "type": "static",
    "join_url": "https://qms.example.com/join?t=K7M2QX",
    "expires_at": null
  }
}
```
`type` = `static` | `dynamic`. `expires_at` is only set for dynamic codes (Unix seconds).

Errors: 404 `NOT_FOUND` (not the owner).

### PUT /api/v1/queues/:queue_id/qr/type
Changes the QR mode. Body `{ "type": "static" | "dynamic" }`. The response is the same as `GET /qr`.
The static token is kept when switching to dynamic (an old printed QR code becomes valid again when switching back to static).

Errors: 400 `QR_TYPE_INVALID`, 404 `NOT_FOUND`.

### POST /api/v1/queues/:queue_id/qr/replace
Replaces the static token with a new code. The old token is rejected at join time immediately. Static mode only.

200: the same response as `GET /qr` (`type` = `static`).
Errors: 404 `NOT_FOUND`, 409 `QR_REPLACE_NOT_ALLOWED` (dynamic mode).

## Counters (owner)

Every queue has one default counter (created automatically, `is_default` true) and at most 10 counters including the default. Every endpoint requires a token and is only for the queue's owner (not the owner: 404 `NOT_FOUND`). A name is 1 to 30 characters after trimming, unique per queue.

### GET /api/v1/queues/:queue_id/counters
200:
```json
{ "data": { "action": "list_counters", "queue_id": 122, "counters": [
  { "id": 11, "name": "Counter 1", "is_default": true, "serving": { "ticket_id": 121, "ticket_number": 7 } },
  { "id": 12, "name": "Fast Counter", "is_default": false, "serving": null } ] } }
```
The default counter comes first, then the rest in creation order. `serving` is null when the counter is not serving anyone.

### POST /api/v1/queues/:queue_id/counters
Body `{ "name": "Fast Counter" }`. An empty name gives an automatic name `Counter N` (the smallest N not yet used, starting from 2). 201: `{ "data": { "id": 12, "name": "...", "is_default": false, "serving": null } }`.
Errors: 400 `BAD_REQUEST` (name longer than 30 characters), 409 `DUPLICATE_COUNTER_NAME`, 409 `COUNTER_LIMIT`, 409 `QUEUE_CLOSED`.

### PUT /api/v1/queues/:queue_id/counters/:counter_id
Renames a counter (the default counter may be renamed). Body `{ "name": "..." }`. 200: the counter object (without the `serving` info).
Errors: 400, 404, 409 `DUPLICATE_COUNTER_NAME`, 409 `QUEUE_CLOSED`.

### DELETE /api/v1/queues/:queue_id/counters/:counter_id
Deletes a counter in one transaction; the number that counter was serving returns to position 1 of the waiting order (`waiting`). 200: `{ "data": { "action": "delete_counter", "queue_id": 122, "counter_id": 12 } }`.
Errors: 404, 409 `DEFAULT_COUNTER`, 409 `QUEUE_CLOSED`.

Every change emits an SSE event `counter_updated` (see the Real-time section).

## Queue history (owner)

History only contains queues that are **already closed**; active queues never appear here (they stay on the dashboard and in `GET /queues`). Data is read directly from the live tables. Every endpoint requires login, and ownership is checked in the query: a user who is not the owner gets an empty list, and 404 for details (the same as "queue does not exist", so the existence of other people's queues does not leak).

Paging uses an opaque cursor (`cursor`) and `limit`. `next_cursor` is `null` on the last page. A malformed cursor: 400 `BAD_REQUEST`. A `limit` that is not a number or is negative: 400; 0 or empty uses the default; values above the maximum are capped at the maximum.

### GET /api/v1/history/queues?limit=&cursor=
The user's closed queues, newest first (ordered by `closed_at` descending, then `id` descending). The default `limit` is 20, the maximum is 100.

200:
```json
{ "data": { "action": "list_history_queues",
  "queues": [ { "id": 54, "queue_name": "Counter", "ticket_prefix": "A",
    "opened_at": "2026-11-03T01:00:00Z", "closed_at": "2026-11-03T09:00:00Z",
    "total": 6, "completed": 2, "skipped": 1, "cancelled": 1, "deleted": 1, "unserved": 1 } ],
  "next_cursor": "1793700935123456.54" } }
```
The per-outcome counts add up to `total`. `unserved` = numbers that were still waiting when the queue was closed (a number being served at closing becomes `completed`).

### GET /api/v1/history/queues/:queue_id?limit=&cursor=
A summary of one closed queue and one page of its numbers, in ascending number order. The default `limit` is 50, the maximum is 200. Not the owner, still active, or not found: 404 `NOT_FOUND`.

200:
```json
{ "data": { "action": "history_queue",
  "queue": { "id": 54, "queue_name": "Counter", "...": "same as in the list" },
  "tickets": [ { "ticket_id": 56, "ticket_number": 3, "outcome": "completed", "skip_count": 0,
    "joined_at": "...", "called_at": "...", "completed_at": "...", "cancelled_at": null,
    "customer_name": "budi", "customer_email": "budi@mail.com",
    "counter_name": null, "note": "asked for a receipt" } ],
  "next_cursor": null } }
```
`outcome`: `completed`, `skipped`, `cancelled`, `deleted`, or `unserved`. `counter_name` is only filled when the queue has more than one counter. `note` is the owner's note (null when there is none) and is never visible to customers. Fields with null values are still present in the JSON.

### GET /api/v1/history/queues/:queue_id/export?format=csv|xlsx
Downloads all numbers of one closed queue. Only the owner; not the owner, still active, or not found: 404 `NOT_FOUND`. `format` is required and must be `csv` or `xlsx`; anything else: 400 `BAD_REQUEST`.

200: a file (not JSON) with `Content-Disposition: attachment`, `Cache-Control: no-store`, and `X-Content-Type-Options: nosniff`. The body is streamed; the download headers are sent when the first byte is written, so a failure before that still returns a normal JSON 500.

## Queues (customer)

The public information about a queue for a ticket holder comes from `GET /queues/:queue_id` (see above; `PublicQueueResponse`).

### GET /api/v1/join/preview?t=<code>
The queue's public information from a join code (login required). The backend looks up the code in `queue_settings` (`qr_current_code` for static codes); a code that has been replaced or has expired is rejected.

200: `{ "data": { "action": "join_preview", "queue": PublicQueueResponse } }`
Errors: 400 `BAD_REQUEST` (empty code), 404 `QR_INVALID`, 410 `QR_EXPIRED`.

### POST /api/v1/join
The customer joins through a join code. Body `{ "t": "K7M2QX" }`.

- The join code is looked up again on the server (never trust a `queue_id` sent by the client).
- If the queue has no ticket for this user yet: a new ticket is created with `status` = `waiting`.
- If the user already has an **active** ticket (`waiting` or `currently_served`): **the existing ticket is returned** with 200 (not an error, and not a second ticket). This check comes before the email verification check, so scanning again does not turn into an error.
- Queue closed: 409 `QUEUE_CLOSED`.
- No active ticket (the old ticket is `completed`, `cancelled`, `skipped`, or `deleted`): joining creates a new ticket with a new number (rejoin). The old ticket is not changed.

200: the ticket object directly inside `data`.
```json
{ "data": { "id": 121, "queue_id": 122, "ticket_number": 2, "sort_key": 131072, "user_id": 3, "status": "waiting", ... } }
```
Errors: 400 `BAD_REQUEST` (empty code), 403 `EMAIL_NOT_VERIFIED` (new numbers only), 404 `QR_INVALID` / `NOT_FOUND`, 409 `QUEUE_CLOSED`,
410 `QR_EXPIRED`, 503 `RETRY_EXHAUSTED`.

The old endpoint `POST /api/v1/queues/:queue_id/join` has been **removed** (joining is only possible through a join code).

### GET /api/v1/queues/:queue_id/status
The queue status for the owner or a ticket holder. Other users get 404.

200:
```json
{
  "data": {
    "action": "queue_status",
    "queue": PublicQueueResponse,
    "my_ticket": TicketResponse | null,
    "current_number": 1 | null,
    "current_counter_name": "Fast Counter" | null,
    "waiting_ahead": 3 | null,
    "previous_number": 0 | null,
    "next_number": 2 | null,
    "serving": [ { "counter_id": 11, "counter_name": null, "ticket_number": 1 } ]
  }
}
```
- `current_number`: the number being served (`currently_served` ticket); `current_counter_name` = the name of its counter.
- `serving`: one element per counter that is serving a number (`counter_name` is null for the default counter).
- `previous_number`: the previous number.
- `next_number`: the number of the next `waiting` ticket.
- `waiting_ahead`: the number of `waiting` tickets in front of the requester's own ticket. Null when the requester does not hold a `waiting` ticket.

## Public display (feature 4.5)

The public display shows one card per counter, the next numbers, and the skipped numbers on a waiting-room monitor, without login. The only credential is a **random token in the URL** (`/display/<token>`, 32 base64url characters from 24 bytes), not the `queue_id`, so queues cannot be guessed. Configuration is only for the queue's owner and does not require an active subscription.

### GET /api/v1/queues/:queue_id/display
Requires a token (owner). The display configuration. 404 when the display has not been enabled or the caller is not the owner.

200:
```json
{ "data": { "queue_id": 5, "token": "Z8x7LgV0pgl1DEZ7EIuOIytZ11ndM5th", "upcoming_count": 5, "skipped_count": 0,
            "created_at": "...", "rotated_at": null } }
```
The client builds the display URL from the web origin + `/display/` + `token`.

### PUT /api/v1/queues/:queue_id/display
Requires a token (owner). Enables the display (a new token is created) or changes its counts (the token stays the same).

Body (all optional): `{ "upcoming_count": 5, "skipped_count": 0 }`. A value that is not sent keeps the current value when the display already exists, otherwise defaults to 5 and 0. `0` is a valid value and hides that section on the screen.

200: the configuration object as above. Errors: 400 `BAD_REQUEST` (outside 0 to 10, or a malformed body), 404 (not the owner).

### POST /api/v1/queues/:queue_id/display/rotate
Requires a token (owner). Replaces the token: the old link immediately returns 404, including for screens that are open (they receive a signal and show "Display link is no longer valid"). 200: the configuration with the new token. 404 when not enabled or not the owner.

### DELETE /api/v1/queues/:queue_id/display
Requires a token (owner). Turns the display off. Idempotent. 204.

### GET /api/v1/display/:token
**No login token.** The snapshot for the screen. It is cached on the server for 1 second per screen and invalidated on every queue event, so a read after a signal is always fresh.

200:
```json
{ "data": { "queue_name": "Counter A", "queue_desc": "General service", "status": "active", "multi_counter": true,
            "counters": [ { "id": 11, "name": "Counter 1", "serving": "A-007" },
                          { "id": 12, "name": "Fast Counter", "serving": null } ],
            "upcoming": ["A-008", "A-009"], "skipped": [], "generated_at": "..." } }
```
- `counters`: **all** counters of the queue, the default counter first and then the rest in creation order, including those that are empty (`serving` null). The client draws one card per element.
- `multi_counter` false (only the default counter): the client does not show the counter name.
- Numbers are already in display-ready text (`A-007`, the `ticket_prefix`, three digits). `upcoming` = the first `upcoming_count` `waiting` tickets; `skipped` = the most recent `skipped_count` `skipped` tickets.
- `status` `closed`: the queue is closed (still 200, not 404).
- **This response is public.** It never contains ticket ids, user ids, `owner_id`, customer names, `sort_key`, or owner notes. A contract test locks the list of keys.

The same 404 `NOT_FOUND` is returned for a malformed token, an unknown token, a replaced token, or a display that has been turned off. The header `Cache-Control: no-store` is set.

### GET /api/v1/display/:token/event
**No login token.** SSE for the screen. Unlike `/queues/:id/event`, the screen joins a separate room that receives **only** a signal without data, `{"type":"display_changed","queue_id":5,"ts":"..."}`, which means "fetch the snapshot again". Ticket payloads (which contain user ids) are never sent here. One action may emit more than one signal (Skip emits two); the client merges them. No signal is sent for `queue_qr_updated`.

Heartbeat `: ping` every 25 seconds, like a normal stream. Errors: 404 (token not valid), 429 `TOO_MANY_DISPLAYS` (more than 20 simultaneous screen connections for one queue), 503.

nginx: the rate-limit zone `display_zone` (20 requests per second per IP, burst 40) and an access log that masks the token (`/api/v1/display/***`).

## Owner subscription (BorderPay)

Served by the backend. Payments go through payment-service and BorderPay.

### POST /api/v1/owners/subscription/checkout
Requires a token. Body `{ "method": "qris", "return_url": "https://..." }` (only QRIS is supported; `return_url` is optional, the page to go to after paying, and must be https). Creates a one-month payment order through payment-service and BorderPay. The price is set by the server from the `SUBSCRIPTION_PRICE_IDR` environment variable (default 15000). The subscription is **not** extended here: `owners.subscription_expires_at` is extended by 30 days by the BorderPay webhook `payment.paid` after the payment is confirmed. A prospective owner becomes an owner automatically with their first confirmed payment.

200 response:
```json
{ "data": { "reference_id": "SUB-7-1794539540-a1b2c3d4e5f6", "pay_url": "https://<payment-provider>/pay/...", "expires_at": "2026-11-13T04:12:20Z", "amount": 15000 } }
```
Errors: 400 `INVALID_PAYMENT_METHOD`, 403 `EMAIL_NOT_VERIFIED`, 502 `PAYMENT_REJECTED`, 503 `PAYMENT_UNAVAILABLE`.
Also: 401 `UNAUTHORIZED` (no token or an invalid token), 500 `INTERNAL_ERROR` (database error; nothing changed). Through nginx, this path goes to the backend (`location /api/`).

## Payment webhook (BorderPay)

### POST /api/v1/webhooks/borderpay

Public, no JWT. Called by BorderPay when a QRIS transaction is paid. The `x-borderpay-token` header must equal `BORDERPAY_WEBHOOK_TOKEN` (an empty token means all requests are rejected). Only payloads with `data.status` = `paid` change data; other statuses get 200 with no effect. The order is matched by `reference_id`. Idempotent: a re-sent delivery for the same order does not apply the payment twice.

| Status | Meaning |
|---|---|
| 200 | Applied (`{"status":"ok"}`), already applied, status is not `paid`, or `reference_id` is unknown (ignored) |
| 400 | Body too large or not JSON, or `reference_id` is empty |
| 401 | `x-borderpay-token` does not match `BORDERPAY_WEBHOOK_TOKEN` |
| 409 | The webhook amount does not match the order; the payment is not applied |
| 500 | Saving to the database failed; BorderPay resends |

Through nginx, this path goes to payment-service and is not rate limited.

## Objects

### QueueResponse
```json
{ "id": 122, "owner_id": 1, "queue_name": "Counter A", "queue_desc": "General service", "ticket_prefix": "A",
  "status": "active", "created_at": "...", "closed_at": null }
```
`status`: `active` or `closed`. `queue_desc` may be null.

### PublicQueueResponse
The same as QueueResponse without `owner_id`.

### TicketResponse
```json
{ "id": 121, "queue_id": 122, "ticket_number": 2, "sort_key": 131072, "user_id": 3,
  "status": "waiting", "skip_count": 0, "last_notified_at": null,
  "created_at": "...", "completed_at": null, "cancelled_at": null,
  "called_counter_id": null, "called_counter_name": null, "called_at": null }
```
`called_counter_id` and `called_at` are set when the number is called (cleared when it is returned to `waiting`). `called_counter_name` on `TicketResponse` is currently always null; the counter name is in `TicketEvent` (only for non-default counters) and in `current_counter_name` and `serving`.
`status`: `waiting`, `currently_served`, `completed`, `cancelled`, `skipped`, `deleted`.
`ticket_number` is the number the customer sees. `sort_key` is an internal ordering key.

## Real-time (SSE)

### GET /api/v1/queues/:queue_id/event
Server-Sent Events stream. Requires a token in the `Authorization` header (so the client uses a `fetch`
stream, not `EventSource`). Only the owner or a ticket holder. Otherwise 404.

Response headers: `Content-Type: text/event-stream`, `X-Accel-Buffering: no`.

Initial frame:
```
: connected

```
About every 25 seconds the backend sends a heartbeat:
```
: ping

```

Each event looks like this:
```
data: {"type":"ticket_next","queue_id":122,"payload":{...},"ts":"2026-10-17T09:29:59.659Z"}

```

| `type` | Sent when | `payload` |
|---|---|---|
| `ticket_joined` | A customer joins successfully (new ticket) | TicketEvent |
| `ticket_next` | The owner presses Next | TicketEvent with `status` = `currently_served` |
| `ticket_skipped` | The owner presses Skip | TicketEvent with the ticket's final status (`skipped`, or `waiting` when returned); followed by `ticket_next` for the number that is called |
| `ticket_cancelled` | A customer cancels their number | TicketEvent with `status` = `cancelled`; followed by `ticket_next` if the next number moves up |
| `ticket_removed` | The owner removes a customer's number | TicketEvent with `status` = `deleted`; followed by `ticket_next` if the next number moves up |
| `queue_updated` | The queue name or description is changed | `{ "queue_id": 122 }` |
| `queue_closed` | The queue is closed | `{ "queue_id": 122 }` |
| `queue_qr_updated` | The QR mode is changed or the static token is replaced | `{ "queue_id": 122 }` |
| `counter_updated` | A counter is created, renamed, or deleted | `{ "queue_id": 122, "counter_id": 12, "action": "created" \| "renamed" \| "deleted" }` |

Note: `queue_updated`, `queue_closed`, `queue_qr_updated`, and `counter_updated` use plain object payloads, not TicketEvent. The client only needs the `queue_id` to fetch the status again.

### TicketEvent
The same as TicketResponse, but without `user_id`, `sort_key`, and `last_notified_at`; `called_counter_name` is filled for non-default counters. The reason: the recipients of an event can be other customers in the same queue, so internal identity and ordering are not sent.

After receiving an event, and after the connection recovers from a drop, the client must fetch the ticket list (or the status) again.

## Compatibility notes

- Joining again returns 200 with the same ticket. Older clients that still handle 409 `ALREADY_JOINED` remain safe, because that path is no longer called.
- `POST /join` and `POST /next` return the ticket object directly (without `action`). Other endpoints use `action`.
- The `GET /api/v1/queues/:id/info` endpoint has been removed; ticket holders use `GET /api/v1/queues/:id`.
- The id-based join (`POST /api/v1/queues/:id/join`) has been removed. Clients must use the token from the QR code
  (`GET /join/preview?t=...` followed by `POST /join {t}`). A static token that has been replaced is rejected with 404 `QR_INVALID`.