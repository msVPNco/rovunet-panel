# Review20 private laboratory continuation

This is an unsigned review candidate, not a production update. Review19 accepted
subgates are retained. Four ACT03 actions and the 32 original obligations remain
partial. No runtime pin, release signing identity, real account or client source
is changed.

## Exact JSON integers

Run tools/generate-status-schema.py to regenerate status-v2.schema.json.
Python integers and Decimal-aware JSON Schema validation carry the exact
9223372036854775807 maximum, including revision. ParsePrivateStatusV2 uses raw
JSON tokens and arbitrary precision rational parsing before conversion to int64;
no float64 intermediary is permitted. Integral JSON forms 1.0 and 1e0 remain
integers, while fractions, negative byte counts, strings, booleans and overflow
are rejected. Revision starts at one. The parser rejects numeric inputs longer
than 128 characters and extreme exponents to bound work.

## Optional installed v2 renderer

subscription-v2 enable is an explicit operator action. The config flag defaults
false; ordinary post-update does not opt existing installations into v2.
The unit is rovunet-subscription-v2.service; its sole listener is
/run/rovunet-subscription-v2/http.sock, root:rovunet 0660.
Caddy alone routes /v2/sub/* to that Unix socket. Four supported private aliases
are TOKEN.json, TOKEN.windows.json, TOKEN.status.json and
TOKEN.windows.status.json. Feeds remain wire version1; status is version2.
The installed resolver reads fresh config and ledger on each request and
serializes the private monotonic revision state. It does not use static status
as a substitute for an unavailable ledger.

The renderer runs as root:root with supplementary rovunet, empty capabilities,
PrivateNetwork, PrivateDevices, ProtectSystem=strict, ProtectHome and AF_UNIX
only. Root is currently required by the private DB ownership checks and reading
the existing credential derivation files. Socket directory is 0711 so Caddy may
traverse but not enumerate. Runtime dir, configured DB directories and private
revision state are the only writable areas. DB reads may create SQLite WAL/SHM;
this is why those directories are writable. No generic admin API is exposed.

subscription-v2 refresh regenerates/restarts the unit and its Caddy routes;
disable removes the route before stopping the renderer. A missing revision
authority returns503. A stopped backend returns Caddy502 with no-store and no
static feed/status fallback. Missing/invalid capability is404, unsupported
methods405, successful GET/HEAD200. HEAD has no body. Status headers revision
and generated_at come from the same snapshot as the response body. HTTPS
authority in the VM is an explicitly trusted fixture root, not public trust.

The tested Caddy bytes are SHA256
88099ffd284666727b1d3e8224320e9190e1171932f9c3a5e871dc2f67380fef,
Go1.26.8, Caddy v2.11.7 => ../caddy-bbr@(devel), forwardproxy => ../forwardproxy.
Captured build-info and module list identify this specimen only. Exact original
replacement source/patch reconstruction remains unresolved; source-built
candidates from the older engineering review are different artifacts.
This specimen is not cleared for a public canary or promoted catalog.

## Signed same-store carry

carry-period takes a signed manifest and an independently pinned public key file.
CarryPlan binds original config hash, server/user/account/old+new periods,
DB directory/store, unchanged finite limit, confirmed/pending and expiry.
The owner-side signer is outside the installation; the panel receives no private
key. A private before-image, pending journal and durable receipt bind the request.
Only the affected account's gate/live fence closes its streams before the old
ledger period is closed and the config selects the new one. Unknown pending or
a changed balance is held. Opening40 with limit1000 means remaining960; the
limit must not be reduced to960 and then charged40 again.

After a crash, the ordinary publisher resumes the journal forward. Receipt
replay never opens another budget. Conflicting signed requests are refused.
A failed balance check after preparation keeps a visible pending hold for
operator resolution; there is no force-reset/implicit rollback escape hatch.
Five SIGKILL phase tests cover prepared, ledger-closed, config-selected,
published and receipt. They are process-kill evidence, not hardware power loss.

## Sealed checkpoint and lost installation

recovery-checkpoint seal NEW-DIRECTORY OPERATION seals admission first and
exports a coherent private snapshot: config, SQLite snapshots, public aliases,
rotation receipts, carry archive, revision registry/anchor and native VLESS
derivation secrets. A failed export stays sealed. Repeating the same seal
verifies the archive instead of taking a different balance under the same ID.

tools/recovery-custodian.py runs outside both installation and VM. It owns a
separate Ed25519 identity, monotonic durable state and one-target claims:
init, seal, claim, release. Its files must be private and independently backed
up by the owner. claim is durably committed before emitting its signed output;
the same operation replays the same envelope, a second physical target cannot
claim it, and the old source cannot subsequently be released. The release
signing root is a separate established identity and is never passed to this tool.

On a fresh physical destination, new-target creates a nonce; restore-target
checks the externally signed latest checkpoint hash and nonce, verifies every
manifest member, restores privately and records a commit receipt before opening
admission. Logical server/account/period and credential roots survive; the
physical instance changes. activate-empty only populates an empty canonical
installation; it refuses another existing instance. V2 remains off until
explicitly enabled on that target. Old claim replay must preserve consumption.
Missing DB/receipt/revision authority is an error; never delete anchors or
reinitialize a DB to make recovery succeed.

Trust boundary: the custodian's durable state and key must not be rolled back or
copied into independent authorities. A root operator able to copy target nonce,
keys and all trusted state can clone an installation; this does not implement
hardware anti-rollback or global quota. The H: custody used for the VM experiment
is separate from the guests but is not an accepted offline disaster recovery kit.
Restoring every runtime, Caddy TLS root/private keys and firewall/DNS state is
outside this native finite TCP recovery lane and remains a production gate.

## Upgrade and caller guard

Public install/update/legacy restore refuse coordinated configurations before
downloads or mutation. tools/upgrade-coordinated-lab.sh is limited to an owned
lab marker and independently reviewed SHA256. It stops the old expiry timer,
oneshot, native units and renderer, checks MainPID=0 and refuses any remaining
process using the installed binary inode before atomically replacing it.
The strict post-update path recovers forward and regenerates native SQL-bound
units without downloading arbitrary runtime upgrades. A failure leaves services
stopped for repair; the old writer/database is never automatically restored.

The native installed SQL unit requires all its users bound to one local store
and a loopback Caddy front. Mixed legacy/SQL or multiple stores require separate
reviewed listeners; they are not silently merged. Xray/SSH/UDP hard quotas and
cross-server shared quota remain unaccepted. Reserve waits can fail closed under
contention. Only connection initialization and BEGIN retry within three seconds;
transaction bodies and ambiguous COMMIT outcomes are not replayed. High-load
control-plane fairness is still open, including the recorded VM contention run.

## Extra address identity

extra_address_ids persists a mapping from a logical canonical address to the
old ipN suffix. First migration uses the existing order. Reorder preserves IDs,
removed entries keep tombstones, and a genuinely new extra endpoint gets a new
suffix instead of inheriting the removed endpoint's ID. Identical display names
do not identify endpoints. Ambiguous historical canonical aliases and corrupt
ID collisions stop rendering for explicit reconcile; they are not silently
merged. Tests compare a real emitted pre-migration feed to the reordered feed.
The client must still perform endpoint/trust fingerprint reconciliation and
confirm its pin/override behavior. Primary address promotion/trust rotation is
not implied by this extra-address mapping.

## Remaining acceptance

This delta does not rerun or enlarge the accepted Review19 73 tests/12 groups,
does not count the guarded VM helper's ordinary SKIP as a pass, and does not
turn fixture boolean expectations into app parser results. Client adapters and
matching TLS/reconnect/reboot/quota behavior on macOS, Windows and Android must
still return current capability-based evidence. Runtime ABI/advisory/license
promotion, production custody and coherent operational recovery outside the
reviewed native lane remain blocked. No release is authorized.

