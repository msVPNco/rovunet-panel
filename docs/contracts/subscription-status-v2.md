# Subscription HTTP v2 and private status v2

This is the panel contract for the local review20 candidate. Production and final
client acceptance are separate gates. Existing /sub/ resources and strict feed
v1 remain supported during migration.

| Resource | Endpoint | JSON body version |
|---|---|---|
| Android/default feed | /v2/sub/<token>.json | 1 |
| Windows feed | /v2/sub/<token>.windows.json | 1 |
| Default private status | /v2/sub/<token>.status.json | 2 |
| Windows private status | /v2/sub/<token>.windows.status.json | 2 |

Only canonical lowercase 32-hex tokens are accepted. GET/HEAD are supported;
other methods return 405. Unknown aliases, encoded-path alternatives and query
strings return 404. The preview command is loopback HTTPS only. The optional installed renderer
uses a private Unix socket behind Caddy, defaults off, and exposes only these
four resources. See [review20 operations](review20-operations.md) for its unit,
permissions, lifecycle, exact runtime specimen and remaining production gates.

Successful responses use application/json and Cache-Control: no-store. Missing
tokens/resources return 404; broken revision authority or configuration races
return 503. Neither response proves active, unlimited, or expired.

## Why private body v2

Strict private v1 requires a numeric used_bytes for a finite account and cannot
represent confirmed and pending separately, explicit limit/expiry kinds or hold.
We therefore choose an independently versioned private body v2. No required keys
are added to v1. HTTP path v2 alone does not identify body version: feeds retain
exactly version,name,items with feed body version1. A client must opt in to status
v2 parsing. It must never reinterpret v2 as private status v1.

Private status2 fields are:
version=2; server_id/account_id/period_id; revision; account_status;
limit_kind/limit_bytes; expiry_kind/expires_at; confirmed_bytes/pending_bytes;
coverage; measurement; sampled_at; generated_at. Numeric byte counts are signed
int64 nonnegative. Null is unknown/not applicable, never an implicit zero.
limit_kind is finite/unlimited/unknown; finite has positive limit_bytes,
unlimited has null. expiry_kind is at/none/unknown; at has RFC3339 UTC expires_at.

account_status is active, expired, limit_reached, hold, or archived. Unknown DB,
binding, period or malformed policy produces hold. Confirmed exhaustion means
limit_reached; pending alone does not. Not knowing status cannot authorize a
server session. Status and provider headers do not grant admission.

measurement is accounted_lower_bound, none, or unknown. coverage is partial,
unavailable, or not_metered in this candidate. Even native accounted bytes are
not an end-to-end delivery receipt or global coverage of all engines. Missing DB
returns null confirmed/pending; it is not legacy fallback. Native activity alone
does not manufacture sampled_at. Any source sample remains a lower bound.

## Identity, revision and freshness

Revision is durable semantic state scoped to server/account/period. Changed
token, default/windows format, HTTP Date or generated_at do not reset identity
or increment it. Renew creates a new period scope. Changes to policy, confirmed,
pending, coverage or account status increment revision. Timestamp-only refreshes
do not. Missing/corrupt revision registry after initialization fails closed.
Current-period confirmed usage cannot regress; it produces an error if a stale
DB is attached. Revision files/anchor require coherent operator recovery.

HTTP Date is response transport time. generated_at identifies each resolver read;
sampled_at is the underlying sample time and may be null/old. Two sequential GETs
may observe different committed DB states. Clients compare bindings and revision;
do not add their counts or invent new source/item identities. Native and sampled
directions are read in a single DB read transaction with confirmed/pending.

Rovunet-Revision and Rovunet-Generated-At identify the response snapshot.
Subscription-Userinfo is optional and derives from the same resolver snapshot.
Its total/expire can be meaningful when usage is unknown. Missing upload/download
remain omitted. Opening balances are never split into fabricated directions.
The body is authoritative private status for this server binding; generic
provider headers have their own parse/cache/trust model.

## Client adaptation and fixtures

Mac, Windows and Android need exact endpoint mapping above, a strict independent
status2 parser, nullable confirmed/pending, explicit kinds, hold and freshness.
Android review01 already recognizes /v2/sub; no blanket prefix defect is claimed.
Keep final artifact acceptance separate for each app.

Rotation is a source URL change, not replacement of item IDs, source ID, manual
selection, pins or local overrides. Stable item IDs remain attached to the
account/server/transport/extra-address mapping. Validate server/account/period
before accepting status. Refuse conflicting cached identities or an unapproved
TLS trust change; metadata may not silently overwrite saved credentials or pins.

The generated status-v2-fixtures.json contains time-only, volume-only with
unknown usage, mixed, explicit unlimited, malformed expiry, invalid limit and
expired cases. Endpoint tests add missing, encoded path and conflict cases;
lifecycle tests cover pending, settlement, rotation, ledger error, exhaustion
and missing revision state. Existing multi-address stable-item fixtures remain
unchanged. Passing a panel fixture is not evidence that any final app binary
already implements this contract.

