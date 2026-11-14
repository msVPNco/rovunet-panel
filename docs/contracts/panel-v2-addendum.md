# Panel2 client integration addendum

The strict feed remains version1. Current status2 implementation is in
`internal/handlers/private_status_v2.go` and `subscription_metadata.go`.
Historical review20 operational records describe their original specimens;
they do not replace the current implementation or certify physical clients.

## Endpoints and access

The optional renderer defaults off. An operator enables it explicitly with
`subscription-v2 enable`; normal update does not opt existing servers in.
There is no automatic discovery, capability negotiation, request nonce or
separate Bearer header. A valid HTTPS origin and the existing private path
token are required. Tokens are32 lowercase hexadecimal characters.

Under `/v2/sub/`, `TOKEN.json` and `TOKEN.windows.json` retain feed body1;
`TOKEN.status.json` and `TOKEN.windows.status.json` return status body2.
The current renderer additionally supports `TOKEN.trust.json`. There is no
`TOKEN.windows.trust.json` alias. Existing `/sub/` resources remain supported.
The old four-alias description predates the trust addition.

GET/HEAD succeed with200; unknown resources404, unsupported methods405, broken
snapshot/authority503. A stopped renderer can yield502 behind Caddy. Neither
failure proves expiry or unlimited access. Queries and encoded alternatives
are rejected. Responses use JSON and `Cache-Control: no-store`.

## Exact status and freshness

Required status2 fields are version, server_id, account_id, period_id, revision,
account_status, limit_kind, limit_bytes, expiry_kind, expires_at, confirmed_bytes,
pending_bytes, coverage, measurement, sampled_at and generated_at.

`measurement` is a **string enum**, not an object: accounted_lower_bound, none,
or unknown. Independent coverage values are partial, unavailable, not_metered.
Unknown is null, not zero. Unlimited accounts can still have known non-null
confirmed/pending amounts when bound to a valid ledger; unlimited describes
the limit, not absence of measured consumption. Confirmed is a recorded lower
bound; pending is an uncertain reservation, not confirmed consumption. Pending
can reduce safe capacity without being relabeled consumed. Partial coverage
does not establish exact remaining delivery across all engines.

finite+none expiry is volume-only, unlimited+at is time-only, finite+at is both,
and unlimited+none is unlimited. The two kinds are independent.25 configured GB
means26,843,545,600bytes or25,600MiB using1024; bytes are authoritative.
Use exact checked int64 values, not a floating-point intermediate.

Revision is durable semantic state scoped to server/account/period. Token,
format or timestamp changes do not reset it. Renewal creates a new period.
Reject conflicting bindings, revision rollback and contradictory equal-revision
state. Do not sum snapshots or mix periods. HTTP Date is transport time,
generated_at is resolver time, sampled_at is actual sample time and can be
null/old. Rovunet-Revision and Rovunet-Generated-At identify the same response
snapshot. Subscription-Userinfo is optional; unknown directions stay omitted.
A retained app UX cache must be identified as stale/unknown after failed fetch;
new timestamps do not turn old consumption into a fresh sample.

## Trust is separate from sessions

Trust body1 has server/account/period bindings and items with profile_id,
transport, domain, certificate_pem, certificate_sha256 and not_after. The hash
is SHA256 of the complete certificate DER, not SPKI. Only a public certificate
is exported; private keys and mixed bundles are refused. Match stable item ID
and domain, not display name or row position.

The same HTTPS origin authenticates this companion. It is not independently
signed authorization to replace a saved identity. There is no trust nonce,
trust_revision or signed max_age. The HTTP status revision does not bind the
certificate digest; a newer Date or account revision cannot authorize a new
pin. Keep an existing valid pin on failed refresh. Explicit changed-identity
recovery is separate from ordinary fetch. Unknown first-use TOFU must not
silently replace required public-CA/hostname validation. Final app artifacts
must demonstrate their actual bootstrap and recovery policies.

There is **no panel Slipstream session API** for start/heartbeat/stop, device
IDs, session TTLs or a15-device cap. QUIC transport state and certificate
recovery do not create an account-management session. Account authentication
occurs in the SSH/SOCKS backend. Do not implement guessed REST endpoints or
heartbeats. A future device/session authority needs a separately reviewed
shared contract and real connection enforcement before activation.

Synthetic independent parser/UI examples are in
`fixtures/status2-parser.json`. These are not live credentials, cache-update
sequences, phone acceptance or proof of a session API.
