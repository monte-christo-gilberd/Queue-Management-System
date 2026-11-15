# Product Overview

> This is the product-level companion to `backend.md` (covers API + database) and `frontend.md`. Update the status and change log as your project evolves.

## Document Info

| Field            | Value                                              |
| ---------------- | -------------------------------------------------- |
| Project name     | _Queue Management System_                          |
| Status           | In Progress                                        |
| Owner            | _Monte Christo Gilberd_                            |
| Target live date | _Week 14 of capstone_                               |
| Summary          | A quick to set up and used queue management system |

## Change Log

| Date       | Change                                        |
| ---------- | --------------------------------------------- |
| 2026-07-23 | Initial draft                                 |
| 2026-08-01 | Reduced MVP scope after consultation feedback |
| 2026-08-10 | Finished Scope                                |
| 2026-08-13 | Added User Stories and edited scope           |
| 2026-08-15 | Reformatted user stories to standard Given/When/Then format, added Functional Requirements, Non-Functional Requirements, and Known Limitations & Future Improvements using Generate AI  |
| 2026-09-09 | Clarified admin authority delegation as per-queue (shared by all admins on a queue) rather than per-admin, to match the database schema; added data model note for Google Authentication (4.17); changed Add Notes (4.16) to Add/Edit Notes — one note per queue number, edited in place instead of appended |
| 2026-09-20 | Customer join (4.2, 4.4, US-14) now requires login (replaced no-login/anonymous design); added Owner Subscription & Payment (4.18, US-21, Scope row 18) to formalize the monetization model from ideation; added CSV/Excel export and dedicated-archive data model note to Read History (4.15); added owner's saved-admin-roster note to Admin Appointment (4.6) |
| 2026-10-02 | Backend for Create/View/Manage a Queue (4.1) fully rebuilt and verified (`go build`/`go test` passing, 16 new unit tests) — closes the gap left open since the Week 11 from-scratch rebuild. **Reversed the Week 9/10 architecture decision that rejected WebSocket in favor of "REST + polling"** (NFR #4): frontend direction was set to React Native (Expo), and real-time queue/ticket updates (4.3, 4.4, 4.5) are now pushed over WebSocket instead of polled. Subscription gating (4.18) was narrowed to apply only to actions that create or modify data (create queue, edit queue, edit admin authorities) — viewing, listing, and closing a queue no longer require an active subscription. `owners.subscription_expires_at` changed from nullable to `NOT NULL`, reflecting a clarified business rule: an `owners` row is only ever created after a first successful payment, so "never subscribed" is no longer a state the data model needs to represent |
| 2026-10-31 | **Tier 2 delivered:** Skip (4.7) with fallback setting (4.14: `remove`/`back`/`after_n`, max returns), owner recall (3 s after Next, once per call) and Web Push notification through a separate internal `notify-service` (4.8, push only, no outbox yet). Customers enable notifications per browser on their ticket screen (HTTPS only; Expo Go unsupported); one browser serves one account at a time. Email notify, custom ticket prefix and counter CRUD remain deferred. |
| 2026-10-30 | Overhaul after the Week 15 rebuild. **Removed the admin system** (US-3, US-18, FR 4.6): only the owner manages a queue; tables `queue_authz`, `queue_admins`, `owner_saved_admins` dropped. **Counter redesigned** (US-4, US-19, FR 4.11): every queue gets one hidden default counter; counter CRUD and the counter page deferred to Tier 2. **Join is QR-only and signed** (US-5, FR 4.2): tokens from a separate `qr-service` (static or dynamic, 120 s TTL); the customer joins only by scanning in the in-app camera, and a repeated join returns the same ticket (200). The QR image is drawn on the frontend. **Real-time stays SSE** (FR 4.4/4.5), with a resync every 45 s and a 3 s fallback poll when the stream is held back by a proxy. Skip (FR 4.7) with fallback (FR 4.14) and push notify (FR 4.8) were added after the rebuild; email notify is deferred. Delivery moved to Docker Compose behind nginx. See `plan-rombak-utama.md` and `Architecture_Summary.md`. |
| 2026-10-31 | Removed permanent deletion (history always remains), the history cap, the QR daily usage limit and registration_limit |
| 2026-10-31 | Custom ticket prefix delivered (1-3 letters/digits per queue, default `A`, set on create/edit, shown in UI and push notifications) |
| 2026-11-04 | Profile (plan item 6): `GET /users/me` now carries `subscription_expires_at` (owner only), new `PUT /users/me` edits the username (trimmed, 1 to 50 characters), and the profile screen works for customers and owners. Password change moved out of the profile form: a "Minta Ganti Password" button is shown but does nothing until reset by email (plan item 9), so the password-revocation work (`password_changed_at`, token `iat`, 8-character minimum) moves to item 9 as well. |
| 2026-11-04 | Profile photo: `PUT/GET/DELETE /users/me/avatar`, stored in Postgres (`user_avatars`, bytea, one 512x512 JPEG of at most 150 KB per user), private to its owner; the server validates by content, crops, resizes and re-encodes (EXIF/GPS removed); nginx allows 6 MB bodies and 5 requests per second per IP on this route only. |
| 2026-11-04 | Mailpit and mail-service (plan item 7): development mail is caught by Mailpit; email is sent by a separate `mail-service` (internal API with a service token, Indonesian templates, retries, TLS modes). `notify-service` is Web Push only and keeps working when the mail service is down (verified with real push events). Decision D4 revised accordingly. |
| 2026-11-04 | Email notifications (plan item 8): four emails to the ticket owner (joined, cancelled, removed by the owner, finished serving) through an outbox written by a trigger in the same transaction as the ticket change; the worker lives in `mail-service` (role `mail_rw`, no access to passwords or tickets), retries with backoff, is rate limited, and never includes owner notes. New switch `users.email_notifications` with `PUT /users/me/preferences` and a profile switch. Delivery is at-least-once. |
| 2026-11-03 | **Delivered since 31 Oct:** counter management (4.11: create, rename and delete counters, one number per counter, one account on several devices, a tab per counter), customer cancel (4.9), notes CRUD (4.16), insert at a position (4.12), dynamic QR (4.13), a customer can also join by pasting the join link into the join popup, error toasts, a ticket popup on the customer card, and the push "your turn" message names the counter. Join stays in-app only: opening `/join` outside the app creates nothing. **Plan to the end** (`queue-management-system/docs/plan-sampai-akhir-03-11-2026.md`): owner removes a number (status `deleted`, no fallback), owner history and CSV/Excel export, customer history, notes shown in the queue, user profile, Mailpit, email notifications (joined, cancelled, removed, completed), password reset by email, requeue push, a confirmation popup for Next and Skip (no undo, by design); then observability, k6 load tests (staged toward 1M concurrent users) and frontend performance audits. |
| 2026-11-04 | Public display (4.5) done, outside the 14 plan items: login-free `/display/<token>` with one card per counter that follows counters being added, renamed or deleted; random URL token; the screen listens on its own SSE room that carries only a "refetch" signal, never ticket payloads; snapshot cached for 1 second and invalidated before each event; owner settings card ("Layar Publik") on the queue detail. Verified with Go tests (race detector), Postgres integration tests, frontend unit tests and Playwright `ts-32`. |
| 2026-11-04 | Password reset (plan item 9, 4.20) done: forgot and reset by email, 30-minute one-time token stored as a hash, sessions revoked by `password_changed_at` against the token `iat`, minimum password of 8 characters at registration and reset. Replaces the earlier "in progress" status. Verified with Go tests, Playwright `ts-31` and a manual run. |
| 2026-11-04 | Requeue push (plan item 10): trigger `tickets_notify` emits `requeued` (with `reason` and `counter`) when a number returns to `waiting`; `skipped` is now only for permanent skips; the push states the new position. |
| 2026-11-04 | **Synced with the code.** Done and verified in the repo: owner history and CSV/Excel export (4.15), notes shown in the ticket list and history (4.16), customer history per ticket (4.22), full profile (4.23). Email notifications (4.19) are sent by `mail-service`, not `notify-service` (Scope row 19 corrected). Password reset (4.20) moved to *in progress*: backend (`/users/forgot-password`, `/users/reset-password`, `user_action_tokens`, `password_changed_at`, session revocation via `iat`), mail template, `/reset-password` and `forgot-password` pages and nginx rules are written but still uncommitted, with no end-to-end run. Still not built: public display (4.5), Google login (4.17), payment (4.18), observability and load tests (NFR 11-13). Completion estimate: about 82% of the Scope table (see "Project Completion"). |
| 2026-11-06 | **Synced with the code.** Owner subscription and payment (4.18, row 18) is done: a separate `payment-service` (Stripe Checkout, idempotent webhook, dev simulator) extends the subscription by one month, and the first payment turns a regular user into an owner; the Subscription screen is wired to it. Also built and now recorded: the stack is five Go services plus Postgres, nginx and the frontend; counters, cancel, remove, insert, notes, history and export, public display, profile photo, password reset and email notifications (all listed above); graceful backend shutdown. **Built, not committed:** email verification (new row 26, 4.24): `POST /users/verify-email`, `POST /users/me/verify-email/resend`, guard on create-queue and join (`EMAIL_NOT_VERIFIED`), the `/verify-email` screen and a profile banner with resend (finished later the same day). Still not built: Google login (4.17), observability and load tests (row 25). |
| 2026-11-03 | Document cleanup: corrected statements that contradicted each other or the code (4.10 now uses status `deleted`; NFR #4 says SSE, not WebSocket; 4.18 `subscription_expires_at` is `NOT NULL`; 4.14 requeue rules match the `remove`/`back`/`after_n` setting; 4.2 and 4.3 match the in-app join and the no-undo decision) and updated Scope statuses to match the code. |

## Background & Problem

| #   | Problem                                                                                                                | Solution                                                                                                                                                                                 |
| --- | ---------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | User having to queue in line without knowing their number or current number                                            | Web-based queue management system that is easy to set up and run ⚠️ _needs confirmation: with the frontend direction now set to React Native (Expo, see Change Log 2026-10-02), is "web-based" still accurate for all users (owner, customer), or does this become a native-app experience for some/all of them? The customer join flow (4.2, US-14) specifically assumes "phone browser opens the join page" via QR scan — confirm whether that still holds or whether Expo changes it too_ |
| 2   | Small Bussiness Owner forgetting which customer to serve first because chaotic queuing                                 | Small Bussiness Owner and Front Staff (Owner side) could easily set up the system using their own pc as the server or manually use a server as their main server and use qr to add queue |
| 3   | Front Staff handling customer cannot create a queue management system without integrated system prepared by IT support | User side can use their phone camera to join the queue and view their number in the queue                                                                                                |

## Scope

| #   | Feature                                       | Description                                                                                                  | Priority | Status  | User Stories      |
| --- | ---------------------------------------------- | -------------------------------------------------------------------------------------------------------------- | -------- | ------- | ------------------ |
| 1   | Create, View, and Manage a Queue              | Owner can create, view, edit, and delete a queue, and view all customers currently registered in it     | P0       | Done | US-1, US-2, US-6   |
| 2   | Join Queue via QR Code (static)               | Customer logs in (manual or Google) and joins a queue by scanning a static QR code generated by the owner    | P0       | Done | US-5 (static), US-14 |
| 3   | Continue Queue (Next)                         | Owner can continue the queue to the next number, on a single default counter                             | P0       | Done | US-8 (next)        |
| 4   | Customer View: Queue Status & Position        | Customer can view their position in the queue and the number currently being served                           | P1       | Done | US-15              |
| 5   | Display Queue for Public                      | Owner can display current queue on a screen for public to view (one card per counter, live)                                          | P1       | Done | US-7               |
| 6   | Admin Appointment & Authority Delegation      | **REMOVED 2026-10-30.**  Owner can appoint admins and configure a shared set of authorities for the queue they're on                   | P1       | Backlog | US-3, US-18        |
| 7   | Skip Queue                                    | **DONE (31 Oct).** Owner skips the number being served; the system calls the next waiting number in the same step. 409 when nothing is served or nobody is waiting. | P2       | Done | US-8 (skip)        |
| 8   | Notify Next                                   | **DONE (31 Oct, push only).** A separate `notify-service` sends Web Push: the owner of a called number gets "your turn", the first waiting customer gets "1 number left", and a skipped customer gets "skipped" (or "returned to the queue"). Owner can recall the number being served (3 s after Next/Skip, once per call). Email is deferred. | P2       | Done (push); email planned | US-9               |
| 9   | Queue Cancelling for Customer                 | **DONE (Nov 2026).** Customer can cancel their own queue number (kept in the record as `cancelled`). | P2       | Done | US-16              |
| 10  | Removing a Customer from Queue                | Owner can remove a specific customer number from the queue. The number gets status `deleted` (kept in the record, no fallback). | P2       | Done | US-11              |
| 11  | Counter Management                            | **DONE (Nov 2026).** Owner creates, renames and deletes counters (max 10); each counter serves one number; one account can be used on several devices, each acting for one counter. | P2       | Done | US-4               |
| 12  | Inserting a Certain Number in Queue           | **DONE (Nov 2026).** Owner inserts a customer (by email or username) at a specific position in the queue. | P3       | Done | US-10              |
| 13  | Dynamic QR Code                              | **DONE.** QR code can be static or dynamic (a code that rotates, valid for 120 s). | P3       | Done | US-5 (dynamic)     |
| 14  | Skipped Customer Fallback / Requeue           | **DONE (31 Oct).** Per-queue setting for a skipped customer: remove, return to the back, or return after N positions, with a maximum number of returns. Partly covers the original multi-attempt rule (see 4.14). | P3       | Done | US-17              |
| 15  | Read History & Export                         | **DONE (Nov 2026).** Owner can view all closed queues (paged), open one to see every number with its outcome and note, and export a queue's record as CSV or Excel (`/history/queues`, `/history/queues/:id`, `/history/queues/:id/export`). | P4       | Done | US-13              |
| 16  | Add / Edit Notes                              | **DONE (Nov 2026).** Owner can attach one note (editable in place) to a queue number; the note is shown in the owner's ticket list and in history. Never shown to customers. | P4       | Done | US-12              |
| 17  | Google Authentication                         | User can log in using their Google Account                                                                     | P4       | Backlog | US-20              |
| 18  | Owner Subscription & Payment                  | **DONE (Nov 2026).** Owner must have an active paid subscription to create queues. A separate `payment-service` extends it by 1 month after a Stripe payment (no other plans, no recurring billing); the first payment also makes a regular user an owner | P1       | Done | US-21              |
| 19  | Email Notifications                           | **DONE (4 Nov).** Customer gets an email when they join, cancel, are removed by the owner, or finish being served; sent by `mail-service` through an outbox (not `notify-service`) | P2       | Done | US-22              |
| 20  | Password Reset & Change                       | **DONE (4 Nov).** The profile button requests a reset link by email; the user sets a new password on `/reset-password`. Verified with Go tests, Playwright `ts-31` and a manual run against the real stack | P2       | Done | US-23              |
| 21  | Requeue Notification                          | **DONE (4 Nov).** Push notification when a number is returned to the queue (skip fallback or a closed counter), with its new position | P3       | Done | US-17 (extension)  |
| 22  | Confirm Next / Skip                           | A confirmation popup before Next and Skip. There is no undo, by design | P2       | Done | US-8 (extension)   |
| 23  | Customer History                              | **DONE (Nov 2026).** Customer can view all their past tickets and outcomes, one row per ticket, newest first, paged (`GET /users/me/history`) | P4       | Done | US-24              |
| 24  | User Profile                                  | **DONE (4 Nov).** User can view their profile, edit the username, manage a private profile photo and the email switch; the password button requests a reset link (item 20) | P4       | Done | US-25              |
| 25  | Observability & Performance                   | Metrics, logs, dashboards and alerts; k6 load tests toward a staged 1M concurrent-user target; Lighthouse and DevTools audits (after the features) | P4       | Planned | NFR 11-13          |
| 26  | Email Verification                            | **Done (not committed).** A new account gets a verification link by email; an unverified account cannot create a queue or join one (403 `EMAIL_NOT_VERIFIED`). Frontend screen and profile banner with resend; e2e `ts-33` | P2       | Done        | US-27              |

Keep this to at most 3 P0 (must-have) features per the ideation guideline — everything else is P1/backlog.

> **Note:** Row 4 ("Customer View: Queue Status & Position") stays at P1 by decision — the 3 current P0 rows are scoped to get a working prototype running end-to-end first (owner creates a queue, customer joins via QR, owner advances it). Rows 6 and 11 (Admin Appointment and Counter Management) are explicitly deferred past P0 as well: P0 assumes a single owner/operator running one default counter with no delegated admins. Multi-admin and multi-counter support is a later concern (P1/P2), not a P0 blocker.

## User Stories

One or more stories per feature above — this is the bridge between "what" (Scope) and "how" (Functional Requirements). Write these before wireframing; they're what the wireframe screens should satisfy.

```
As a [user role], I want to [do some task] so that [benefit/reason].

Acceptance Criteria:
- Given [context], when [action], then [expected result].
```

### US-1 — Creating a Queue

As an owner, I want to create a queue so that I don't have to manually manage my queue.

**Acceptance Criteria:**
- Given an owner account, when the "new queue" button is clicked, then a new empty queue is created.

### US-2 — Editing a Queue

As an owner, **I want to** manage a queue **so that** I can use it as needed in my business.

**Acceptance Criteria:**
- Given a selected queue, when the edit queue button is clicked, then the queue's name, description, and details can be modified.
- Given a selected queue, when the remove queue button is clicked, then the queue is removed.

### US-3 — Appointing and Managing Administrators

> **Removed 2026-10-30.** The admin system no longer exists; only the owner manages a queue.

### US-4 — Creating and Managing a Counter in The Queue

> **Delivered 2026-11.** Every queue has one default counter (hidden while it is the only one). The owner can create, rename and delete counters.

As an owner, **I want to** create counters **so that** the queue can accept multiple numbers at the same time.

**Acceptance Criteria:**
- Given a selected queue, when the owner creates a counter, then the counter is added to the queue.
- Given a selected counter, when the owner removes it, then the counter is deleted and the number it was serving returns to the front of the queue.

### US-5 — Setting Up The QR Code for The Queue

As an owner, **I want to** set up a QR code **so that** customers only need to scan it to join my queue.

**Acceptance Criteria:**
- Given a selected queue, when the owner sets up a QR code, then customers can scan it to register for the queue.
- Given a static QR code is chosen, when it is generated, then it can be printed on paper for repeated use.
- Given a dynamic QR code is chosen, when it is generated, then each customer receives a different code, requiring a display screen.

### US-6 — Viewing The Queue

As an owner, **I want to** see all customers registered in the queue.

**Acceptance Criteria:**
- Given a selected queue, when the owner opens the queue view, then all registered customers are displayed.

### US-7 — Displaying The Queue

As an owner, **I want to** display the queue on a publicly available screen **so that** customers can view its current situation.

**Acceptance Criteria:**
- Given a selected queue, when the owner sets up a display, then the queue's basic description is shown on it.
- Given an active display, when a number is called, then the current called number is shown.
- Given an active display, when the owner has configured a count of upcoming numbers, then that many next numbers are shown.
- Given an active display, when the owner has configured a count of skipped numbers, then that many skipped numbers are shown.

### US-8 — Continuing The Queue

As an owner, **I want to** continue the queue to the next number.

**Acceptance Criteria:**
- Given a customer has finished being served, when the owner clicks the "next" button, then the current number display changes to the next number.
- Given the next number does not respond, when the owner clicks the "skip" button, then that number is marked skipped and the queue continues.

### US-9 — Notifying The Customer

As an owner, **I want to** notify the next customer about the queue's progress **so that** they can be prepared to be called.

**Acceptance Criteria:**
- Given a customer becomes next in line, when their turn approaches, then they are automatically notified.
- Given the current number does not respond, when the owner presses the notify/bell button, then that customer is notified again.
- Given a notification is triggered, when it is sent, then it is delivered as a push notification.
- Given a number is called (Next or Skip), when the call is committed, then its owner is notified "your turn" and the first waiting customer is notified "1 number left".
- Given a customer's number is skipped, when the skip commits, then that customer is notified it was skipped, or returned to the queue when a fallback applies.
- Given a number is being served, when the owner presses "Recall" at least 3 seconds after calling it, then only that number's owner is notified again; recall works once per call.
- Given a customer joins a queue, when the join succeeds, then the browser asks for notification permission and subscribes automatically; if permission is denied or unsupported nothing happens. A button on the ticket screen remains as a manual fallback; logging out removes the subscription.
- Given the owner has pressed Next, when the queue detail is shown, then "Recall" replaces "Skip" and is disabled for 3 seconds; after Recall is pressed, "Skip" returns and is disabled for 3 seconds.

### US-10 — Inserting a Customer in The Queue

As an owner, **I want to** insert a customer at any position in the queue **so that** they can bypass the current order when needed.

**Acceptance Criteria:**
- Given a selected queue, when the owner inserts a customer at a chosen position, then that customer occupies that position in the queue.

### US-11 — Removing a Customer in The Queue

As an owner, **I want to** remove a customer from the queue.

**Acceptance Criteria:**
- Given a selected number in a queue, when the owner removes it, then that number is deleted from the queue.

### US-12 — Appending a Note to a Number in The Queue

As an owner, **I want to** append a note to a number in the queue **so that** I can quickly recall what I was doing with that queue number.

**Acceptance Criteria:**
- Given a selected number in a queue, when the owner adds a note, then the note is saved and attached to that number.

### US-13 — Viewing Queue History

As an owner, **I want to** access all previous queues **so that** I can review all customers I have served.

**Acceptance Criteria:**
- Given a selected past queue, when the owner opens it, then all customers registered in that queue are shown.
- Given a selected number in a past queue, when the owner opens it, then any notes appended to that number are shown.

Repeat one block per feature — keep acceptance criteria concrete enough to test against later, not vague ("works correctly").

### US-14 — Joining The Queue

As a customer, **I want to** log in and join the queue by scanning a QR code.

**Acceptance Criteria:**
- Given a displayed queue QR code, when the customer scans it with their phone camera, then they are prompted to log in (manual account or Google) before registering.
- Given the customer is logged in, when registration completes, then the customer is added to the end of the queue, or as the first number if the queue is empty.
- Given a customer who already has an account, when they scan a QR code, then they aren't asked to create a new account — their existing login works directly.

### US-15 — Viewing The Queue

As a customer, **I want to** view the queue I am registered in **so that** I can predict and prepare myself to be called.

**Acceptance Criteria:**
- Given a joined queue, when the customer opens the queue view, then all queue numbers up to their own number are shown.
- Given a joined queue, when the customer opens the queue view, then the current number being served is shown.

Repeat one block per feature — keep acceptance criteria concrete enough to test against later, not vague ("works correctly").

### US-16 — Cancelling The Queue

As a customer, **I want to** cancel my queue number **so that** I don't have to manually tell staff to cancel it.

**Acceptance Criteria:**
- Given a joined queue, when the customer cancels their number, then that number is removed from the queue.

### US-17 — Skipped Fallback Mechanism

As a customer, **I want to** automatically rejoin the queue if I am skipped **so that** I don't have to rescan the QR code.

**Acceptance Criteria:**
- Given a customer's number is skipped, when the skip occurs, then the customer is pushed back a configured number of positions in the queue and called again later.
- Given the owner opens the queue's skip setting, when they choose `remove`, `back` (to the end) or `after_n` (N positions behind) plus a maximum number of returns, then the choice is saved per queue and applied to every later skip.
- Given a customer was already returned the maximum number of times, when they are skipped again, then they become permanently skipped.

### US-18 — Delegated Authority

> **Removed 2026-10-30.** No admins.

### US-19 — Manning a Counter

> **Removed 2026-10-30.** Counters are not staffed by admins; counter CRUD is deferred (Tier 2).

### US-20 — Google Authentication

As a user, **I want to** log in using my Google account **so that** I don't have to manually configure my account from the start.

**Acceptance Criteria:**
- Given the login screen, when the user chooses "log in with Google", then a fully functional account is created and ready for use.

### US-21 — Owner Subscription & Payment

> **Status: done.** Implemented as a single one-month extension (no monthly/yearly/one-time choice, no recurring billing).

As an owner, **I want to** pay for a subscription **so that** I can create and run queues on the platform.

**Acceptance Criteria:**
- Given an owner account with no active subscription, when they try to create a queue, then they're blocked (402) and directed to pay first.
- Given a successful payment, when Stripe confirms it, then the owner's access period is extended by one month and they can create queues again.
- Given an owner's subscription has expired, when they try to create a new queue, then they're blocked the same way as an owner who never subscribed — existing queues they already created keep running until closed.
- Given a regular user (not yet an owner), when they make their first payment, then they become an owner.

### US-22 — Email Notifications

As a customer, **I want to** receive an email about my queue number **so that** I have a record and do not depend on push notifications alone.

**Acceptance Criteria:**
- Given a customer joins a queue, when the join succeeds, then they receive an email with their number. Joining again returns the same ticket and sends no second email.
- Given a customer cancels their number, when the cancel commits, then they receive a cancellation email.
- Given the owner removes a number, when the removal commits, then that customer receives an email saying the number was removed.
- Given a number was being served, when it is finished (Next), then its owner receives a thank-you email.
- Given the customer switched emails off in the profile, when any of these events happen, then no email is sent.

### US-23 — Resetting and Changing a Password

As a user, **I want to** reset a forgotten password by email, and change my password when logged in, **so that** I never lose access to my account.

**Acceptance Criteria:**
- Given the login screen, when the user requests a reset for an email, then the answer is the same whether or not the email is registered, and a registered email receives a one-time link valid for 30 minutes.
- Given a valid link, when the user opens the reset page and saves a new password, then the password changes, the link cannot be used again, and other sessions are signed out.
- Given an expired or used link, when the reset page is opened, then an error is shown with a way to request a new link.
- Given a logged-in user, when they press the change-password button in the profile, then a reset link is sent to their account email (planned with plan item 9; the button exists but does nothing yet).

### US-24 — Viewing My Ticket History

As a customer, **I want to** see my past tickets **so that** I can recall where and when I queued.

**Acceptance Criteria:**
- Given a customer with past tickets, when they open "Riwayat", then every finished, skipped, cancelled or removed ticket is listed with its queue, number, outcome and times, newest first.
- Given a customer, when they open the history, then they never see another user's tickets or the owner's notes.

### US-25 — Managing My Profile

As a user, **I want to** view and edit my profile **so that** my account details stay correct.

**Acceptance Criteria:**
- Given a logged-in user, when they open the profile, then their username and email are shown (and, for an owner, the subscription expiry).
- Given a logged-in user, when they change the username and save, then the new username is used everywhere.
- Given a logged-in user, when they turn email notifications off, then no notification emails are sent to them (4.19).

### US-26 — Confirming Next and Skip

As an owner, **I want to** confirm before Next and Skip **so that** a mistaken click does not move the queue.

**Acceptance Criteria:**
- Given a queue board, when the owner presses Next or Skip, then a confirmation popup names the number affected and nothing changes until the owner confirms.
- Given the popup is open, when the owner cancels, then the queue is unchanged.

### US-27 — Verifying My Email

> **Status: built (backend and frontend), not committed.**

As a user, **I want to** confirm my email address through a link **so that** the account is proven to be mine before it creates or joins queues.

**Acceptance Criteria:**
- Given a new registration, when the account is created, then a one-time verification link (valid 24 hours) is emailed.
- Given an unverified account, when it tries to create a queue or join one, then the request is refused with 403 `EMAIL_NOT_VERIFIED`.
- Given a valid link, when the user opens it, then the email is marked verified without logging in again.

Repeat one block per feature — keep acceptance criteria concrete enough to test against later, not vague ("works correctly").

## Functional Requirements

Repeat this block per feature. This is the most useful pattern to borrow: it forces you to think through the flow and the edge cases _before_ building, not after.

> **Note (2026-10-30):** the single-operator, single-counter assumption below is now the actual design, not just a prototype stage; admins were removed.

> **P0 scoping assumption:** the 3 P0 features assume a single owner/operator running the queue on one default counter, with no admins. Multi-counter logic (Scope, row 11) comes after the single-operator prototype works end-to-end.

> Below covers the 3 P0 features only (Scope, rows 1–3), so the prototype can be built end-to-end first. P1+ features (rows 4 onward) can get their own 4.x blocks once P0 is working.

### 4.1 — Create, View, and Manage a Queue

**Summary**
Lets the owner set up and maintain the queue(s) their business runs — create a new queue, view it along with everyone currently registered in it, edit its details, or delete it. This is the foundation every other P0 feature depends on (nothing else works without a queue to join or advance).

**Flow**

1. Owner opens the dashboard and clicks "New Queue".
2. Owner enters a name/description → clicks "Create" → system creates an empty queue and opens it.
3. Owner selects an existing queue → sees its details plus the live list of customers currently registered in it (US-6).
4. Owner clicks "Edit" → updates name/description/details → clicks "Save" → queue is updated.
5. Owner clicks "Delete" → confirms → the queue is closed and moved into history (4.15); it's no longer active or joinable, but its record and registrations remain viewable there. Queues and their history are never permanently deleted; history always remains.

**Edge Cases Handling**

| Edge Case                                                              | Expected Behaviour                                                                 |
| ------------------------------------------------------------------------ | -------------------------------------------------------------------------------------- |
| Owner submits "New Queue" with an empty name                            | Blocked with a validation message; queue is not created                              |
| Owner deletes a queue that still has customers waiting                  | Show a confirmation warning ("N customers will lose their spot"); on confirm, the queue closes and moves to history rather than being erased |
| Owner edits a queue's details while customers are actively joining      | Edits apply going forward only; customers already in the queue keep their position   |
| Same queue is open and edited in two browser tabs/devices at once       | Last save wins; no merge — acceptable for P0 single-operator scope                   |
| A customer's registration is cancelled (4.9) after the limit was reached | Their slot frees up — a cancelled registration doesn't count toward the limit, so a new customer can join |

### 4.2 — Join Queue via QR Code (static)

> **Updated 2026-10-30. The join link carries a signed token from `qr-service`. Joining happens only through the in-app camera scanner, which calls `POST /join`. Scanning again returns the existing ticket (200). QR images are drawn on the frontend from `join_url`.**

**Summary**
Lets a customer join a queue by scanning one static QR code the owner generates and displays or prints — the direct implementation of Problem #1 and #3 in Background & Problem. Customers must be logged in (manual account or Google, 4.17) to join; this replaced an earlier "no login required, anonymous by device" design once account-based identity was needed for reliable anti-double-join and for showing customers' real names in History (4.15).

**Flow**

1. Owner opens a queue and clicks "Generate QR Code" → system creates one static QR code that links to that queue's join page.
2. Owner prints or displays the QR code at their location.
3. Customer opens "Masuk ke Antrian" in the app and scans the code with the in-app camera, or pastes the join link into the form in the same popup. (Opening the join link in the address bar only shows an instruction page.)
4. If the customer isn't logged in yet, they're prompted to log in (manual account or via Google, 4.17) before continuing.
5. Customer confirms joining → system assigns them the next available number and appends them to the end of the queue (or as number 1 if the queue is empty, per US-14).
6. System shows the customer a confirmation screen with their assigned number.

**Edge Cases Handling**

| Edge Case                                                        | Expected Behaviour                                                                          |
| -------------------------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| Customer scans the QR code while the queue is closed/not accepting  | Show "queue is currently closed"; no number is assigned                                     |
| Customer scans the same QR code again after already joining         | Detected via their logged-in account and shows their existing number instead of assigning a new one |
| Customer has no internet connection when scanning                   | Join page fails to load / shows an offline error; no number is created until they retry     |
| Customer opens the join link in the browser address bar          | Only an instruction page is shown and no ticket is created; the customer must scan the QR in the app or paste the link into the join popup       |

### 4.3 — Continue Queue (Next)

**Summary**
Lets the owner advance the queue to the next number once they've finished with the current customer, on the single default counter assumed for P0.

**Flow**

1. Owner opens the active queue and sees the number currently being served.
2. Owner finishes serving that customer and clicks "Next".
3. System marks the current number as served (`completed`) and advances the "currently serving" pointer (`currently_served`) to the next number in line.
4. If there is no next number, the system indicates the queue is empty.

**Edge Cases Handling**

| Edge Case                                                  | Expected Behaviour                                                              |
| -------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| Owner clicks "Next" when the queue is empty                   | "Next" is disabled or shows "no customers waiting"; no state change             |
| Owner double-clicks "Next" quickly                             | Button is briefly disabled after a click (debounced) so only one number advances per click |
| Owner clicks "Next" by mistake before actually finishing a customer | No undo, by design. A confirmation popup is shown before "Next" and "Skip" (4.21), and a second device acting on the same counter is rejected with 409 `COUNTER_STATE_CHANGED`            |

### 4.4 — Customer View: Queue Status & Position

**Summary**
After joining (4.2), the customer can see their number, the number currently being served, and how many people are ahead of them — the direct answer to Problem #1 in Background & Problem.

**Flow**

1. Customer joins the queue (4.2) and lands on a personal status page tied to their logged-in account.
2. Status page shows their number, the number currently being served, and how many are ahead of them.
3. Page auto-refreshes (or polls) as the queue advances, without the customer needing to rejoin.

**Edge Cases Handling**

| Edge Case                                                          | Expected Behaviour                                                              |
| ---------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| Customer closes their browser and reopens the same link later          | Status page still shows their current, accurate position — no re-join needed    |
| Customer's number was cancelled by themselves (4.9)                    | Status page shows their number as cancelled — retained in the record/history, but no longer active in the queue |
| Customer's number was removed by the owner (4.10)                | Status page shows their number as removed by the organizer (status `deleted`): kept in the record, never shown in the skipped list, and no fallback applies |
| Customer logs in on a different device than the one they joined with   | Status page still shows their number correctly, since it's tied to their account, not the device |

### 4.5 — Display Queue for Public

> **Status (2026-11-04): done.** A public, login-free screen at `/display/<token>` shows one card per counter (same look as the customer's "Nomor Anda" card), plus the next N and the skipped numbers. The owner turns it on from the queue detail ("Layar Publik"), sets the counts, copies or opens the link, and can replace the link or turn it off. The QR code on the screen is not built.

**Summary**
Lets the owner show the queue's live status on a public screen (e.g. a monitor in the waiting area) so nearby customers can follow along without checking their phone.

**Flow**

1. Owner opens a queue and clicks "Aktifkan Layar", then sets how many upcoming and skipped numbers to show (0 to 10 each).
2. Owner opens the generated display URL on a screen. The URL holds a random token, so queues cannot be guessed by id.
3. Display shows the queue's name/description and **one card per counter** with the number it is serving (a counter with nothing to serve stays as a dimmed card), the next N numbers, and skipped numbers if configured. A queue with a single counter shows one large card without a counter name.
4. Display updates automatically as the owner clicks "Next" or "Skip".

**Edge Cases Handling**

| Edge Case                                                | Expected Behaviour                                                     |
| ------------------------------------------------------------ | ---------------------------------------------------------------------- |
| Display screen loses its network connection                 | Shows a "reconnecting" state rather than freezing on stale numbers     |
| Owner sets upcoming/skipped count to 0                       | That section is hidden entirely rather than shown empty                |
| Multiple displays are open for the same queue                | All stay in sync, reading from the same source of truth                |
| Owner adds, renames or deletes a counter                     | The cards change on the open screen without a reload                   |
| Owner replaces the link or turns the display off             | The old link stops working at once; an open screen says the link is no longer valid |
| Queue is closed                                              | The screen shows "Antrian ditutup"                                     |
| The link leaks                                               | It shows ticket numbers only, never names or notes; the owner can replace it |

### 4.6 — Admin Appointment & Authority Delegation

> **Removed 2026-10-30.** The admin system (`queue_authz`, `queue_admins`, `owner_saved_admins`) no longer exists. Only the owner manages a queue.

### 4.7 — Skip Queue

**Summary**
Lets the owner skip the currently called number if the customer hasn't responded within a configured time, so the queue keeps moving.

**Flow**

1. Owner calls a number (via Next, 4.3) and the customer doesn't show up within the configured wait time.
2. Owner clicks "Skip".
3. System, in one step, handles the skipped number according to the queue's fallback setting (4.14) and calls the next waiting number.
4. Skipped numbers feed into the public display (4.5) and the requeue fallback (4.14).

**Edge Cases Handling**

| Edge Case                                                          | Expected Behaviour                                                  |
| ---------------------------------------------------------------------- | ------------------------------------------------------------------- |
| Owner clicks "Skip" before the configured wait time has elapsed       | Allowed — the configured time is a suggestion, not a hard block     |
| No next number available after skipping                               | 409 `QUEUE_EMPTY`; the current number stays served                  |
| Nothing is being served                                               | 409 `NOTHING_TO_SKIP`                                               |
| A number is skipped after the customer already cancelled themselves   | No error — treated as a normal skip                                 |

### 4.8 — Notify Next

**Summary**
Automatically (and on demand) notifies the next customer(s) in line so they're ready when called.

**Flow**

1. When a number is called (Next or Skip), the database emits an event; `notify-service` sends "your turn" to that number's owner and "1 number left" to the first waiting customer.
2. If that customer doesn't respond, the owner presses "Recall": allowed 3 seconds after the call and once per call, and it only notifies the number being served (409 `NOTHING_TO_RECALL`, `RECALL_TOO_SOON`, `ALREADY_RECALLED`).
3. Customers subscribe per browser from their ticket screen; Web Push needs HTTPS and a supporting browser (not Expo Go).
4. When the queue has several counters, the "your turn" message names the counter that called the number (for example "Counter 2"); with a single counter the name stays hidden.
5. Email for joining, cancelling, removal by the owner and finishing service is done (4.19), and the requeue push with the new position is done (4.14).

**Edge Cases Handling**

| Edge Case                                                        | Expected Behaviour                                                              |
| ---------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| Customer hasn't granted notification permission on their device        | Notification silently fails; their status page (4.4) remains the fallback source of truth |
| Customer isn't "next" yet, just somewhere further back in line          | No notification sent to them yet                                                |
| Owner presses "Recall" repeatedly                                      | Only the first press after 3 seconds is accepted; later presses get `ALREADY_RECALLED` |
| Same browser used by another account                                   | The ticket screen re-registers the subscription to the logged-in account; logout releases it |
| `notify-service` is down when a number is called                       | That push is lost (no outbox yet); the status page remains the source of truth |
| Subscription rejected by the push service (404/410)                    | Deleted automatically                                                           |

### 4.9 — Queue Cancelling for Customer

**Summary**
Lets a customer cancel their own queue number directly, without asking staff to do it. Like a removal (4.10), the number isn't deleted — it's marked as cancelled and stays in the queue's record, visible later in history (4.15).

**Flow**

1. Customer opens their status page (4.4) and clicks "Cancel my number".
2. Customer confirms the cancellation.
3. System marks their number as cancelled and takes them out of the active waiting order; the queue continues normally for everyone behind them. Their entry remains in the record with a "cancelled" status rather than being erased.

**Edge Cases Handling**

| Edge Case                                                              | Expected Behaviour                                                     |
| ---------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| Customer cancels while they are the number currently being served           | Marked cancelled and the queue advances to the next number — same effect as a skip on the active order, but recorded as "cancelled" |
| Customer attempts to cancel twice (double click / two open tabs)             | Second attempt is a no-op, no error shown                              |
| Customer cancels right after being notified as "next" (4.8)                  | Owner dashboard reflects the removal from the active order in real time |

### 4.10 — Removing a Customer from Queue

> **Status (2026-11-03): done.** The owner removes a waiting or served number from the ticket list of the queue detail (after a confirmation popup) through `POST /queues/:id/tickets/:ticket_id/remove`. The customer sees "Dihapus" in real time. A push and an email for the removal follow with email notifications (4.19).

**Summary**
Lets the owner remove a specific customer's number from the active queue (e.g. a reported no-show, or a data-entry issue). The number isn't erased — it gets the status `deleted`: it stays in the queue's record and history as "removed by the organizer", leaves the waiting order like a skipped number, but is not shown in the skipped list and no fallback (4.14) is applied.

**Flow**

1. Owner opens the registered customer list (4.1) and selects a number.
2. Owner clicks "Remove".
3. After a confirmation popup, the system sets the number's status to `deleted` — it stays visible in the queue's record, is no longer waiting to be called, and is not subject to the fallback rules of 4.14.

**Edge Cases Handling**

| Edge Case                                                        | Expected Behaviour                                                    |
| ---------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| Owner removes the number currently being served                       | Marked `deleted` and the counter that was serving it is given the next waiting number |
| Owner removes a number the customer already cancelled themselves      | No-op — already cancelled, no error shown                              |
| Owner removes a number that's already marked skipped                  | No-op — stays skipped, no duplicate action                             |

### 4.11 — Counter Management

> **Delivered 2026-11. Every queue has one default counter (it cannot be deleted, can be renamed, and its name stays hidden while it is the only counter). The owner can create (maximum 10), rename and delete counters. Each counter serves one number at a time. One owner account can be used on several devices; each device acts for one counter (remembered per queue on the device) and can switch at any time. Admins stay removed.**

**Summary**
Lets the owner run several counters so several customers can be served at once.

**Flow**

1. Owner taps "+ Tambah counter" on the queue detail or dashboard → a popup suggests a name (Counter 2, Counter 3, …) → the owner confirms or edits it → a new tab appears and this device switches to it.
2. Owner presses Next on a counter; it pulls from the shared queue.
3. Owner can rename a counter (✎ or long press) and close a non-default counter; the number it was serving returns to the front of the waiting order.

**Edge Cases Handling**

| Edge Case                                                              | Expected Behaviour                                                              |
| ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Two counters click "Next" at nearly the same moment                         | Each counter is assigned a different number — no duplicate serving                |
| Owner removes a counter that's actively serving a number                    | The in-progress number is returned to the front of the remaining queue            |
| Two devices act on the same counter at nearly the same time                 | The second action is rejected with 409 `COUNTER_STATE_CHANGED` and the board reloads quietly |
| A device's counter is deleted from another device                           | That device falls back to the default counter                                      |

### 4.12 — Inserting a Certain Number in Queue

**Summary**
Lets the owner insert a customer at a specific position, letting them bypass the normal order when needed (e.g. a walk-in exception).

**Flow**

1. Owner opens the registered customer list and clicks "Insert".
2. Owner enters the customer's email or username (the customer must already have an account) and the target position. The insert is rejected when that account already holds an active ticket in this queue, or when the username matches more than one account.
3. System inserts the number there; everyone behind it shifts back by one position.

**Edge Cases Handling**

| Edge Case                                                              | Expected Behaviour                                                              |
| ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Insert at position 1 while a customer is currently being served              | Inserted number becomes "next", not "current" — doesn't interrupt the in-progress customer |
| Target position is beyond the current queue length                          | Inserted at the end instead; no error                                             |
| Two inserts at the same position happen concurrently                        | Processed in the order received; the second lands just after the first           |

### 4.13 — Dynamic QR Code

**Summary**
Extends the static QR flow (4.2) so the code can rotate per customer. There is no daily usage limit on QR codes.

**Flow**

1. Owner switches a queue's QR setting from "static" to "dynamic".
2. The public display (4.5) shows a fresh QR code after each customer joins.
3. Customer scans the currently displayed code; the rest of the join flow matches 4.2.

**Edge Cases Handling**

| Edge Case                                                              | Expected Behaviour                                                              |
| ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Customer scans a code that has already rotated (e.g. via an old screenshot) | Code is invalid — customer sees "code expired, please scan the current code"      |
| Owner switches from dynamic back to static mid-day                          | Already-joined customers are unaffected; new joins follow the new mode           |

### 4.14 — Skipped Customer Fallback / Requeue

**Summary**
Automatically gives a skipped customer (4.7) another chance by pushing them back a configured number of positions instead of dropping them from the queue outright.

**Flow**

1. A customer's number is skipped (4.7).
2. Owner has configured, per queue (`GET/PUT /queues/:id/settings/skip`): `remove` (permanently skipped), `back` (end of the queue) or `after_n` (N positions behind the first waiting number), plus a maximum number of returns (default 1).
3. If a fallback applies, the system re-inserts that number N positions behind the current "currently serving" number, where N is the configured offset.
4. The ticket returns to `waiting` with a new position; its status and position update on the status page (4.4). The owner of that number gets a push with the new position (event `requeued`, reason `skip`). When a counter is deleted while serving a number, that number returns to position 1 and its owner gets the same kind of push naming the counter (reason `counter_closed`). A permanent skip still sends the plain "skipped" push. Push only, no email.
5. When their turn comes around again, the normal Next/Skip flow applies — and if skipped again, the configured fallback rule (step 2) determines what happens next.

**Edge Cases Handling**

| Edge Case                                                              | Expected Behaviour                                                              |
| ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Customer is skipped again after being requeued                              | Follows the queue's setting: `remove` (permanently skipped), `back` or `after_n` (returned), limited by the configured maximum number of returns; after that the number is skipped permanently |
| Configured requeue offset (N) is larger than the remaining queue length      | Customer is placed at the end of the queue instead                                |
| Customer cancels themselves (4.9) while sitting in "skipped" state          | Marked as cancelled instead — doesn't wait for a fallback slot          |

### 4.15 — Read History

> **Status (2026-11-04): done.** The owner history list (closed queues only, paged), the detail screen (every number, outcome, note) and CSV/Excel export are built (`/history/queues`, `/history/queues/:queue_id`, `/history/queues/:queue_id/export`; unit tests for handler and export pass).

**Summary**
Lets the owner look back at past queues — e.g. to review who was served on a given day — and export that record as a CSV or Excel file to keep or share outside the app. History is never deleted and has no size limit: every closed queue stays there permanently.

**Flow**

1. Owner opens "History" from the dashboard.
2. System lists all closed queues for that owner, sorted by date.
3. Owner selects one → sees every customer who was registered in it, along with their outcome (served, skipped, cancelled, removed) and any note (4.16) attached to their number.
4. Owner clicks "Export" → chooses CSV or Excel → downloads a file containing that queue's full record (customer, outcome, timestamps, notes).

**Edge Cases Handling**

| Edge Case                                                        | Expected Behaviour                                                          |
| ----------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| Owner opens history for a queue that's still active                    | Excluded from history (or shown separately) until the queue is actually closed  |
| Owner deletes a queue entirely (4.1)                                    | Deleting a queue closes it and archives it into history (treated as "completed"); there is no permanent deletion, history always remains |
| Owner exports a queue with zero customers registered                   | Downloads a file with headers only, no rows — not an error                     |

**Data model note**
History is read from the live `queues`, `tickets`, `ticket_notes` and `counters` tables: a closed queue is never deleted, so a second copy would add nothing today. The archive table `queue_history` exists in the schema but is not used; it is kept for archiving when the volume grows. Nothing is ever deleted: no cap and no automatic removal. Export (CSV/Excel) reads the same data.

### 4.16 — Add / Edit Notes

**Summary**
Lets the owner attach a single note to a specific queue number, viewable later in history. Each queue number holds at most one note; saving again edits that same note in place.

**Flow**

1. Owner selects a number (active or past). If no note exists yet, an "Add Note" action is shown; if a note already exists, it's shown pre-filled with an "Edit Note" action.
2. Owner types and saves the note.
3. If no note existed for that number, a new note is created. If a note already existed, it is overwritten with the new text (and its last-updated timestamp is refreshed) rather than a new note being added alongside it.
4. The (single, current) note is attached to that number and visible wherever that entry is viewed, including in history (4.15). Notes are private to the owner: customers never see them (not in their status, history or emails). A note can be written from the queue detail and is shown in the owner's ticket list (`note`, `note_updated_at`) and in history (done).

**Edge Cases Handling**

| Edge Case                                                    | Expected Behaviour                                                        |
| ------------------------------------------------------------------ | ------------------------------------------------------------------------------ |
| Owner tries to save an empty note                                 | Blocked — no empty note is created; if a note already existed, it is left unchanged |
| Owner needs to update the note on the same number more than once  | Each save overwrites the previous note in place — there is only ever one note per number, not a growing list |
| A note is added to a number that is later removed/cancelled       | Note is retained and still viewable in history                                |

### 4.17 — Google Authentication

> **Status (2026-11-03): not built.** Only the database columns exist; the login and register screens show a placeholder.

**Summary**
Lets a user log in with their Google account instead of manually creating credentials, so an account is ready to use immediately.

**Flow**

1. User opens the login screen and clicks "Log in with Google".
2. User authenticates via Google's OAuth flow and grants permission.
3. System creates (first-time) or matches (returning) an account tied to that Google identity and logs the user in.

**Edge Cases Handling**

| Edge Case                                                              | Expected Behaviour                                                              |
| ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| User's Google account email changes later                                   | System matches on the Google account ID, not email, so login still works          |
| User denies permission during the Google OAuth prompt                       | Returned to the login screen with "login cancelled"; no account is created        |
| An account with the same email already exists another way                  | Auto-linked to the same account — the existing account gains Google as an additional login method |

**Data model note**
`users.google_id` stores the Google account ID and is what returning logins match on (per the edge case above). `users.password_hash` is nullable, since a Google-only account never sets a password — but every user must have at least one of the two (password or Google) to be able to log in at all.

### 4.18 — Owner Subscription & Payment

> **Status (2026-11-06): done.** The gate (402 `SUBSCRIPTION_INACTIVE` on create, edit and QR actions) and payment are built. `payment-service` (port 8093, behind nginx at `/api/v1/payments/` only) opens a Stripe Checkout session (`POST /api/v1/payments/checkout`, same JWT as the backend) and extends the subscription from the Stripe webhook (`POST /api/v1/payments/webhooks/stripe`, signature verified). Only one capability exists: extend by 1 month (no other plans, no recurring billing). Without Stripe keys the service stays up and checkout answers 503; in development `DEV_PAYMENT_SIMULATOR=true` makes the pay button succeed without Stripe (refused in production).

**Summary**
Gates the ability to create a queue behind an active paid subscription — the direct implementation of the monetization model from the original ideation, simplified to a one-month extension. The first payment of a regular user also turns them into an owner (Profile → "Jadi Owner" → Subscription).

**Flow**

1. Owner without an active subscription tries to create a queue → blocked (402) and pointed to the Subscription screen.
2. Owner presses "Bayar & Jadi Owner" (or "Perpanjang 1 Bulan"); the frontend gets a `checkout_url` and the browser goes to Stripe.
3. Stripe calls the webhook; in one transaction the service sets `owners.subscription_expires_at = GREATEST(now(), current expiry) + 1 month` (creating the `owners` row on a first payment) and records one `payments` row (`plan_type = 'monthly'`).
4. Owner can now create queues until the period ends. The browser redirect is only UX; the webhook is the source of truth.

**Edge Cases Handling**

| Edge Case                                                              | Expected Behaviour                                                              |
| ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Owner's subscription expires while they have active (not yet closed) queues | Existing queues keep running normally — the block only applies to *creating new* queues |
| Payment fails or is declined                                                | Owner's access period is unchanged; they're shown an error and can retry          |
| Owner pays again before their current period expires                       | The new period is added on top of (extends from) the current expiry, not from today |
| Stripe resends a webhook                                                    | Idempotent per event and per Checkout session: no double extension                |
| `payment-service` is down or Stripe is not configured                       | Checkout and webhook answer 503 `PAYMENT_UNAVAILABLE`; login, queues, tickets and SSE keep working; Stripe retries the webhook so the extension catches up |

**Data model note**
`owners.subscription_expires_at` holds the current expiry as a single timestamp (`NOT NULL`: an `owners` row only exists after a first payment); whether access is currently active is computed as `subscription_expires_at > now()` rather than stored as a separate flag, so there's no daily job needed to flag owners as expired. Each payment is recorded as its own row in `payments` (amount, plan type, status, resulting expiry) as an audit trail. The service uses its own database role `payment_rw` (no access to queues, tickets or `password_hash`) and two tables of its own (`payment_webhook_events`, `payment_session_credits`). The backend never calls payment-service; the isolation is guarded by `scripts/check-service-isolation.sh`.

### 4.19 — Email Notifications

> **Status: done (plan items 7 and 8, 2026-11-04).** Development uses Mailpit as the mail server. Email delivery is covered by Playwright TS-30 and Go unit and integration tests.

**Summary**
Sends an email to the customer for four events: joining, cancelling, removal by the owner, and finishing service. Mail is sent only by `mail-service`, a separate microservice (SMTP, templates and retries live there); `notify-service` stays Web Push only and does not depend on it, so a mail outage never stops push notifications. Each event is first stored in an outbox written in the same database transaction as the ticket change, so an email is retried instead of lost while `notify-service` is down.

**Flow**

1. A ticket changes (inserted, cancelled, deleted, or moved from `currently_served` to `completed`).
2. The change writes an outbox row; the mail worker picks it up, renders the Indonesian HTML and text template, and sends it by SMTP.
3. Failures are retried with a growing delay; sends are rate limited.
4. The customer can switch emails off in the profile (4.23).

**Edge Cases Handling**

| Edge Case                                                         | Expected Behaviour                                                              |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Customer scans the QR again after joining                             | Same ticket is returned and no second email is sent                              |
| Customer switched emails off                                          | No email is sent                                                                 |
| SMTP is down                                                          | The outbox row stays and is retried; the status page (4.4) remains the truth     |
| A queue with many served numbers is closed                            | Emails are sent at a limited rate, not all at once                               |
| The owner attached a note to the number                               | The note is never included in any email                                          |

### 4.20 — Password Reset & Change

> **Status (2026-11-04): done (plan item 9).** `POST /users/forgot-password` and `POST /users/reset-password`, table `user_action_tokens` and column `users.password_changed_at`, session revocation by token `iat` (`RequireAuthWithRevocation`), the `password-reset` mail template, the `forgot-password` and `reset-password` pages, the profile button, a minimum password of 8 characters at registration and reset, and nginx rules (rate limit, token kept out of the access log, `Referrer-Policy: no-referrer`). Verified: Go service, handler and middleware tests; Playwright `ts-31` (11 cases, with Mailpit); manual run against the real stack. There is no "change password with the current password" form; the profile button sends a reset link instead. Changing the email (which also needs verification) is not built.

**Summary**
A user who forgot the password asks for a reset link by email, then sets a new password on a dedicated page. A logged-in user changes the password the same way: the profile button emails the link to the account's address.

**Flow**

1. User opens "Forgot password" and enters an email. The answer is always the same neutral message.
2. If the email is registered, a one-time link valid for 30 minutes is emailed. Only a hash of the token is stored.
3. The link opens the "Reset password" page; the user enters and confirms a new password.
4. The password changes, the link is marked used, and other sessions are signed out. The user logs in with the new password.

**Edge Cases Handling**

| Edge Case                                                         | Expected Behaviour                                                              |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Email is not registered                                               | Same neutral answer; nothing is sent; response time does not reveal it           |
| Link is expired or already used                                       | Error with a way to request a new link                                           |
| User requests a reset again                                           | The earlier link stops working                                                   |
| Too many requests                                                     | 429 from the proxy and a per-account limit                                       |
| New password is shorter than 8 characters                             | Rejected before the link is touched, so the same link still works                |
| Another session was signed in when the password changed               | That session gets 401 on its next request and is signed out                      |

### 4.21 — Confirm Before Next / Skip

> **Status: done (2026-11-03).** One shared confirmation popup is used on the queue detail, the dashboard board and the live screen; the owner's "remove number" popup uses the same component. The confirm button runs the action once even when double-clicked.

**Summary**
There is no undo for Next (4.3, by design). To prevent mistakes, Next and Skip ask for confirmation first.

**Flow**

1. Owner presses Next or Skip on the queue detail, the dashboard board or the live screen.
2. A popup names the number affected (for example "A-003 will be marked as served").
3. Owner confirms (the action runs) or cancels (nothing changes).

**Edge Cases Handling**

| Edge Case                                                         | Expected Behaviour                                                              |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Owner double-clicks Next                                              | Only one popup opens; the confirm button is disabled while the request runs      |
| Two devices act on the same counter                                   | The later action is rejected with 409 `COUNTER_STATE_CHANGED`                    |
| The counter is empty                                                  | The popup text adapts ("call the first number")                                  |

### 4.22 — Customer History

> **Status (2026-11-04): done.** `GET /users/me/history` returns one row per ticket of the caller, newest first, with a cursor; the owner's note is never included.

**Summary**
Lets a customer review all their past tickets: queue, number, outcome and times.

**Flow**

1. Customer opens "Riwayat".
2. The list shows one row per ticket (not per queue), newest first, loaded page by page.
3. Customer opens a row to see the details (read only).

**Edge Cases Handling**

| Edge Case                                                         | Expected Behaviour                                                              |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Customer has no past tickets                                          | An empty state is shown                                                          |
| Another user's ticket id is requested                                 | 404; a customer only ever sees their own tickets                                 |
| The owner attached a note                                             | The note is not shown to the customer                                            |

### 4.23 — User Profile

> **Status: done (plan item 6, 2026-11-04).** View, edit username, subscription status, upload and delete a profile photo, and the email-notification switch. The "Minta Ganti Password" button is wired to the reset-link request (4.20).

**Summary**
Shows the account details and lets the user edit the username. Customers and owners use the same profile; the subscription card is shown to owners only. Changing the password goes through an emailed link, not a form on the profile: the profile has a "Minta Ganti Password" button that requests that link (4.20). Changing the email address is not built (it would need its own verification link). The user can upload a profile photo (JPEG, PNG or WebP up to 5 MB; the server crops it to a 512x512 JPEG and strips EXIF/GPS data) and delete it; the photo is private and only the account owner sees it.

**Flow**

1. User opens the profile (`GET /users/me`); username, email (read-only) and, for an owner, the subscription status and expiry are shown.
2. User edits the username and saves (`PUT /users/me`). The save button is disabled while the name is unchanged or invalid. The header name updates.
3. Errors appear as toasts; success shows "Perubahan tersimpan".
4. Photo: "Unggah Foto" / "Ganti Foto" opens the picker, the photo is uploaded (`PUT /users/me/avatar`) and shown; "Hapus Foto" asks for confirmation, then deletes it (`DELETE /users/me/avatar`) and the icon returns.
5. Email notifications: a switch "Notifikasi email" (`PUT /users/me/preferences`) turns ticket emails (4.19) on or off; it is on by default.
6. Password: the user presses "Minta Ganti Password"; the emailed link flow is described in 4.20.

**Edge Cases Handling**

| Edge Case                                                         | Expected Behaviour                                                              |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Username empty or only spaces, or longer than 50 characters           | Rejected (400); the button is disabled in the app                                |
| Username contains `<` or `>`                                          | Rejected (400), same rule as registration                                        |
| Two users with the same username                                      | Allowed (usernames are not unique); ambiguity warnings on insert still apply     |
| Request tries to change the email                                     | Ignored; the email never changes here                                            |
| Spaces around the username                                            | Trimmed before saving                                                            |
| File is not JPEG, PNG or WebP (checked by content, not by name), or is corrupt | Rejected (415 or 400) with a toast; the old photo is kept                  |
| Photo larger than 5 MB                                                | Rejected (413) with a toast; the web app also shrinks photos in the browser first |
| Uploading again                                                       | Replaces the previous photo (one per user)                                       |
| Deleting when there is no photo                                       | Succeeds without error (idempotent)                                              |
| Another user asks for this photo                                      | Not possible: only `GET /users/me/avatar` exists, always for the caller          |

### 4.24 — Email Verification

> **Status (2026-11-06): built (backend and frontend), not yet committed.** `POST /users/verify-email`, `POST /users/me/verify-email/resend`, column `users.email_verified_at`, token purpose `email_verify` in `user_action_tokens`, the `email-verify` mail template, a guard on create-queue and join, the public `/verify-email` screen, and the banner with a resend button on the profile. Verified end to end against the dev stack with Playwright `ts-33` (8 tests, Mailpit). Dev sends the link to Mailpit; production follows `MAIL_MODE` (`smtp`, the default, real inbox such as Gmail; or `mailpit`, captured and not delivered). There is no MX or domain allow-list check: ownership is proven by clicking the link.

**Summary**
A new account must prove its email through a one-time link before it can create or join a queue. Enabled by `EMAIL_VERIFICATION_REQUIRED` (default: on when `mail-service` is configured; turning it on without mail-service makes the backend refuse to start). When off, `GET /users/me` always reports `email_verified: true`.

**Flow**

1. Registration emails a link to `/verify-email?token=...` (valid 24 hours, at most 3 emails per account per hour; only the token hash is stored).
2. The user opens the link; the page calls `POST /users/verify-email` and the email is marked verified. No re-login is needed because the guard reads the database, not the JWT.
3. An unverified account that creates a queue or joins gets 403 `EMAIL_NOT_VERIFIED`; the profile shows a banner with a resend button (`POST /users/me/verify-email/resend`, at most 3 per hour). Re-joining with an existing ticket is not blocked. Accounts that existed before the feature, seed accounts and demo accounts count as verified; `make verify-user EMAIL=...` verifies one account by hand (for example in `MAIL_MODE=mailpit`).

**Edge Cases Handling**

| Edge Case                                                         | Expected Behaviour                                                              |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Link is invalid, expired or already used                              | 400 `INVALID_TOKEN` (one error for all three)                                    |
| Resend while already verified                                         | 202 with a neutral message; nothing is sent                                      |
| Too many resend requests                                              | 429                                                                              |

## Non-Functional Requirements

| #   | Category         | Detail                                                                                                    |
| --- | ------------------ | -------------------------------------------------------------------------------------------------------------- |
| 1   | Performance       | Pages (owner dashboard, customer join/status, public display) should load in under 2 seconds                  |
| 2   | Consistency Model | A queue's live state (current number, registrations, counters) requires **strong/absolute consistency** — every read (owner and customer views) must reflect the latest write immediately, with no stale or conflicting views. Data outside the active queue — user accounts and closed-queue history (4.15) — can be **eventually consistent**, where a short propagation delay before all views catch up is acceptable |
| 3   | Concurrency / Reliability | No two counters can serve the same number at once (4.11); no duplicate join can occur from a double-tap or repeated scan (4.2) |
| 4   | Real-time Updates | Customer status pages (4.4) and the public display (4.5) should reflect the queue's current state within a few seconds of a Next/Skip action, without the user needing to manually refresh. **Implementation note (2026-10-30):** delivered over Server-Sent Events (SSE), one stream per queue, with a resync every 45 s and a 3 s fallback poll when a proxy holds the stream back. This replaced the WebSocket plan of 2026-10-02, which had itself reversed the REST + polling decision of Week 9/10 |
| 5   | Availability      | The system should remain usable throughout the business's operating hours; brief downtime (e.g. during owner's own server restart, per Problem #2's self-hosted option) should not lose queue state |
| 6   | Notification Delivery | Push notifications (4.8) are best-effort — the system does not guarantee delivery (e.g. notifications disabled, or `notify-service` down since there is no outbox yet), which is why the status page (4.4) always remains the source of truth. Email (4.19, done) is stored in an outbox and retried, so it is not lost while `notify-service` is down |
| 7   | Security / Permissions | Owner-only actions (edit, Next, Skip, Recall, settings, close) must be enforced on the server side, not just hidden in the UI — a non-owner gets 404 even via a direct request |
| 8   | Data Retention    | Queue history is kept permanently, with no cap and no deletion (4.15) |
| 9   | Usability         | Owner setup (creating a queue, generating a QR code) should not require IT support or technical setup beyond using the web app, per Problem #3 |
| 10  | Device Compatibility | Customer-side join and status pages must work in a standard phone browser reached via QR scan, with no app install required |
| 11  | Observability     | Planned (plan item 12): metrics (Prometheus), structured logs (Loki) and dashboards (Grafana) per service, plus alerts for error rate, latency, SSE connection drops and failed notifications. Logs never contain tokens, passwords or notes |
| 12  | Scalability       | Target, reached in stages (plan item 13): 1 million concurrent users, measured on the scenario that is heaviest for the server: users concentrated in a few very large queues with joins, Next and Skip running continuously, so every event makes all watchers reload their status. Verified step by step from small to large (1k, 10k, 50k, 100k, 250k, 500k, 1M) with k6 and a dedicated connection holder; a lighter scenario (users spread over about 100 thousand queues) is measured at two steps for comparison. Each step must pass its gates before the next. The report states the highest step that passed and what blocks the next one. Known blockers: clients reload the status on every join, next and skip event (about N requests per event for N watchers), `GET /status` runs six or more queries, ticket numbers are allocated with `MAX+1` plus retry on conflict, one backend instance with an in-memory hub, and a single Postgres primary |
| 13  | Frontend performance | Planned (plan item 14): Lighthouse budgets on the production web build (LCP, CLS, INP, TBT), Chrome Performance and memory checks, React Profiler review, and bundle size tracking. Target from #1: pages load in under 2 seconds |

## Known Limitations & Future Improvements
 
**Known Limitations (P0 scope)**
 
- No undo for the "Next" action (4.3), by design. A confirmation popup before Next and Skip (4.21) reduces mistakes.
- Editing the same queue from two tabs/devices at once uses last-write-wins with no conflict warning (4.1) — acceptable for the single-operator P0 assumption, not for concurrent multi-admin editing.
- The system has a single owner/operator (admins were removed). Multi-counter operation (row 11) is delivered: one account can be used on several devices, each acting for one counter.
- QR joining supports static and dynamic codes; there is no daily usage limit.
- Push notifications (4.8) depend on the customer's device/browser permissions, need HTTPS, and aren't guaranteed to arrive — the status page (4.4) is the reliable source of truth. A browser serves one account at a time, and there is no outbox, so pushes during `notify-service` downtime are lost.
- Customers must have an account to join a queue (4.2) — this replaced an earlier no-login/anonymous design; there's no fallback path for a customer who can't or won't create an account.
- Owner subscription/payment (4.18, row 18) is built, but only as a one-month extension (no other plans, no recurring billing); real Stripe payments need keys and a registered webhook, otherwise checkout answers 503.
- A profile photo is visible only to its owner (not to owners of queues or other customers), and is stored in the database (see DESIGN.md for the point at which it should move to object storage). Changing the email address needs a verification link and is planned after password reset.
- Google login (4.17), email change, observability and load tests are not built. Email verification (4.24) is built (backend and frontend).
**Future Improvements**
 
Once the P0 prototype (create/manage a queue, join via static QR, continue via Next) works end-to-end, the plan is to build out the backlog in Scope's existing priority order:
- P1: Done: customer status view (4.4), public display (4.5) and owner subscription/payment (4.18). Admin delegation (4.6) was removed.
- P2: Done: customer self-cancel (4.9), owner-side removal (4.10), multi-counter (4.11), Skip (4.7), push notify (4.8), the Next/Skip confirmation (4.21) and email notify (4.19). Password reset (4.20) is done. Email verification (4.24) is built (backend and frontend).
- P3: Done: inserting a customer at a position (4.12), the skipped-customer fallback (4.14) and the requeue push notification (4.8/4.14).
- P4: Done: queue history and export (4.15), notes (4.16), customer history (4.22) and the profile (4.23). Google authentication (4.17) stays open.
- Quality phase, after the features: observability, staged load tests toward 1 million concurrent users, and frontend performance audits (see `queue-management-system/docs/plan-sampai-akhir-03-11-2026.md`).
With more time beyond this backlog, worth revisiting: a proper conflict-resolution strategy for concurrent queue edits (beyond last-write-wins), and whether a guest/anonymous join path should be reintroduced alongside the account-based one.
 
 ## Project Completion (as of 2026-11-06)

Measured against the Scope table above. Row 6 (admins) is removed and not counted, leaving 25 rows (including the new row 26). A row scores 1 when Done, a fraction when partial, 0 when not started. Partial scores are judgement calls, based on what exists in the code and whether it was verified.

| Group | Rows | Score |
| ----- | ---- | ----- |
| Done | 1, 2, 3, 4, 5, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 18, 19, 20, 21, 22, 23, 24, 26 (23 rows) | 23.00 |
| Not started | 17 (Google login), 25 (observability and load tests) | 0.00 |
| **Total** | | **23.00 / 25 = 92%** |

- Features only (excluding row 25, the quality phase): 23.00 / 24 = about 96%.
- Plan view (`plan-sampai-akhir-03-11-2026.md`, 14 items): items 1-11 done, items 12-14 not started: 11 / 14 = about 79%.
- Verification: `go test ./...` in the backend and the services, and frontend unit tests; Playwright specs (32) cover the main flows, with email tests needing Mailpit.
- To reach 100%: finish email verification (frontend and end-to-end), build Google login (4.17), then observability and load tests.

## Pixso Wireframe
 https://pixso.net/app/design/79Kq0W10KNeTlhGSejyhWg Invite you to join the Pixso Design file "Owner PC Interface"