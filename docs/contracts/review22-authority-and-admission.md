# Review22: authority loss, initialization and bounded admission

Private review delta from Review20. No public catalog/Latest, real server,
client source or production release signing identity changes. The 32 panel
obligations and four partial ACT03 actions remain.

## Custodian authority

The operator tool now requires an explicit independently retained witness path
at initialization. It creates an initializing record, key, signed genesis state
(sequence0, status initialized), independent signed head and durable completion
identity under its lock. Retrying interrupted initialization reuses the key.
A used key plus missing state is never treated as a new authority.

state.json is a versioned identity-bound signed document. Normal seal/claim/
release/replay validates completion identity, state version and exact integer
sequence, signature, public key and independent current head before signing
or writing a new permission. Missing/corrupt state or witness is an error. A
missing latest.envelope alone can be reconstructed with replay. An identical
claim returns the same envelope; a second target is refused.

Each transition durably writes a signed after-image intent, state and independent
head before returning permission/output. A process death with valid surviving
state/head resumes only that already-recorded transition forward. Lost/corrupt
state is never treated as output loss. repair requires the independently trusted
CURRENT head hash, preserves the key and claims, and refuses a pending commit
that needs forensic recovery or a witness older than surviving state.

Legacy custody requires migrate-legacy with an independently retained current
signed envelope and its separately trusted hash, plus a separate witness path.
It preserves the old key and exact last claim. Migration intent supports
interruption/retry. No directory deletion, new identity or sequence reset is a
migration. This is a format/tool API change, not a wire change to panel claims.

Trust boundary: restoring the whole custody directory to an older snapshot does
not roll back its separate witness. Restoring BOTH all trusted state and that
independent witness to an older point cannot be detected without another fresh
authority/hardware guarantee. Never bundle/roll back the witness with custody.
The lab uses separate directories on H, not an approved offline disaster kit.

## Revision authority lifecycle

A durable config binding records authority ID, logical server and state
(initializing, adopting or ready). A disabled renderer keeps this binding.
Legacy enabled state with no binding requires authenticated adoption; disable
records legacy history rather than making initialization fresh.

subscription-v2 initialize [CONFIG] is the explicit first-use operation. It
requires a valid persisted logical server ID, disabled v2 and no prior revision
evidence. It persists initializing BEFORE private files. Retry only finishes
that same identity/genesis; unexpected existing bytes are held, not overwritten.
The ready binding is committed after files are durable.

The immutable completion identity remains at CONFIG.status-v2.anchor.
The version2 registry, mutable anchor.json high-water and transaction intent are
inside CONFIG.status-v2, the renderer's existing private writable directory.
The registry binds authority/server/records and the private high-water hashes
the complete registry. This detects empty/truncated/inconsistent registries.
It also permits atomic file writes without granting the renderer write access
to the entire /etc/rovunet parent.

Prepare/refresh, serve/restart, enable and strict post-update ONLY validate
existing known authority. Missing registry, completion marker, high-water or all
of them never generates an empty map or new ID. Post-update validates before
unit/runtime mutation. disable/enable cannot erase initialization history.
Status/feed remains feed1/status2, exact int64 contract unchanged.

A revision update persists before/after intent before records and high-water.
A pending update serves unavailable, and explicit recover-commit completes
only a validated known transition with surviving authority. checkpoint exports
a private consistent revision checkpoint. repair-authority requires an
independently retained CURRENT manifest hash and matching authority/server;
it never guesses past revision from the DB. A surviving newer registry rejects
an older checkpoint. Operators must not pass a digest read from the same
untrusted/stale checkpoint as proof of freshness.

adopt-legacy accepts a signed independently pinned manifest binding old config,
records, anchor hashes, server and authority. It archives the exact old bytes
and signed manifest, persists adopting, converts without resetting any
server/account/period revision or confirmed amount, and commits ready.
Retry resumes the same adoption; a ready authority does not re-import its old
migration archive. Snapshot/recovery preserves the private adoption archive.

The installed renderer retains the prior root:root/supplementary rovunet
sandbox, Unix socket and four endpoints. Disabled /v2/sub/* now explicitly
returns404 with no-store, rather than forwarding private paths to the decoy.
An enabled broken authority returns503 while the renderer remains alive;
a stopped/failed renderer returns Caddy502 with no static fallback.
Systemd's restart burst limit is distinct from authority; after deliberately
repeated failing restarts the lab stops/resets failed unit state before repair.
No authority files are reset by that service action.

## Cooperative admission and workload scope

One local store has one cooperative writer lock before SQLite BEGIN.
Control operations announce shared control intent before waiting; new
reserve/settle probes defer while control is pending, including a second probe
after acquiring the writer lock. Bounds per process/store are16 data operations
and4 control operations, including active/waiting work. Full backlog fails
closed. SQLite still has one connection per Store and busy_timeout250ms.

Connection initialization and writes retain the3s deadline. Scalar Limit/Read
and explicit DEFERRED metadata read transactions have a1s bound and bounded
rollback cleanup. Read-only metadata no longer requests an IMMEDIATE writer
transaction through the DSN default. There is no blind body/COMMIT retry and
no relaxation of reservations, pending or quota limits.

This is a declared cooperative concurrency/priority policy, not a proof of
global FIFO fairness or guaranteed latency with arbitrary processes/raw SQLite
writers. Old writers must be stopped. A new opt-in diagnostic switch logs only
operation stage/error class or SQLite code, no account/endpoint/credential.

SQLite's WAL concurrency still permits busy outcomes; read transactions and
single-writer constraints are described in the official
[WAL documentation](https://www.sqlite.org/wal.html) and
[transaction documentation](https://www.sqlite.org/lang_transaction.html).
Measured lab workload/latencies and unsuccessful stretch tests are reported
separately; one successful workload is not an unlimited throughput guarantee.

## Upgrade and remaining gates

The reviewed lab upgrade still stops timer/oneshot/native/renderer and checks
no old installed-binary inode process remains. Optional explicitly reviewed
adoption manifest/public-key paths permit legacy v2 adoption before strict
post-update. Missing/stale authority leaves services held for forward repair,
not rollback to an older writer. This is not a public production updater.

The exact Caddy specimen remains SHA256
88099ffd284666727b1d3e8224320e9190e1171932f9c3a5e871dc2f67380fef.
The upstream [v1.6.8 release](https://github.com/anonvector/slipgate/releases/tag/v1.6.8)
describes Caddy2.11.7, forwardproxy e747854 and BBR/idle patches, but does not
authenticate the original local replacement build trees for those bytes.
Prior separate reconstructed candidates remain separate. Original
source/patch/license/advisory/ABI promotion is still open; no catalog change.

Full recovery of TLS private roots, every engine/firewall/DNS, independently
accepted offline custody and operator drill remain open beyond native finite
TCP. Xray/SSH/UDP hard quotas and cross-server global quota are not inferred.
Extra-address mapping/tombstones and fresh-period owner option remain;
primary-address/trust/pin reconciliation is separate.

Client coordination remains capability-based: Windows management case21
(preview.11/source f0638364d0fef0cf6608350e1cd3eb6999172754) is current, Mac17
and Android18 retained. No new Windows task or client source change is issued.
Exact parser/UI/refresh/trust artifact acceptance remains required for shared
activation. JSON integral1.0/1e0 remains accepted; panel serialization is exact.

