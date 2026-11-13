# Managed protocols — v1.21.2

Normal installation and update complete account-managed AmneziaWG, Shadowsocks 2022,
Trojan TLS and VLESS Encryption. Existing test services are never adopted.
The accepted private preview established the staged server/name-update path;
protocol limits and the remaining validation boundaries are documented below.

## Account capabilities

| Protocol | Unlimited | Time only | Traffic limited | Released Android | Released Windows |
| --- | --- | --- | --- | --- | --- |
| AmneziaWG | Yes | Peer removal | Excluded | Compatible native file wrapper | 0.1.2-preview.5 supports; older Betas skip |
| SS2022 | Yes | Denies new sessions after removal; existing streams survive | Excluded | Compatible SS2022 PSK chain | 0.1.1 Beta/current support; 0.1.0 Beta skips |
| Trojan TLS | Yes | Denies new sessions after removal; existing streams survive | Excluded | Compatible | Compatible |
| VLESS Encryption | Yes | Denies new sessions after removal; existing streams survive | Excluded | Compatible pinned ML-KEM format | Compatible pinned ML-KEM format |

The Xray runtime test proves that `HandlerService` removal leaves an already
established stream passing data. Counting capability is therefore separate
from quota eligibility. `protocols.Allowed` governs provisioning and all
exports; a quota account gets none of these four credentials. Existing
VLESS/REALITY/XHTTP quota paths retain their policy. Their legacy restart
mechanism may disconnect other users of that same core.

Expiry is checked by the existing five-minute timer and again before a
managed service starts. API failures remain pending for retry. A link being
removed is not proof that an existing session was closed.

An exact simultaneous-device cap is not implemented. The requested future
default is 15. Shared account links carry no independent physical-device
identity; IP or connection counts cannot substitute for device counts.
Standalone `--devices` provisions independent credentials; it is not a
simultaneous-device limit for panel accounts. AnyTLS/TUIC/Mieru remain future
work because the current Android release does not support them.

## Pinned runtimes and client contracts

- Xray 26.10.31, clean source commit
  `63ca2e0c1d31b89f67700c33842431f541fedd25`, MPL-2.0.
- amneziawg-go commit `b5928efb6ca19f0153958460c3d141f04abc5c2e`, MIT.
- amneziawg-tools commit `ee0f0a9aa34ff0a0da4b3433b9512781cfe02843`, GPL-2.0.
- Android alpha18.1/versionCode 39 source:
  `e6359aae2e2e19164ad7e9ec87267f09a0c1152d`.
- Windows 0.1.0 Beta source:
  `562a3d71670ceb7de909146bf68116d5cc614496`.

`scripts/build-managed-awg.sh` builds immutable sources, verifies modules,
and records runtime checksums, licenses and compiler flags. The Go engine's
static `--version` is older than its pinned source; use its verified build
commit and checksum to identify this build. A newly installed Xray protocol
requires clean pinned Go build metadata. Existing unrelated engines are kept.

The strict v1 feed retains exactly `version/name/items` and `id/link`. Android
uses the existing URL; Windows gets `<token>.windows.json` with a matching
status alias. Private deployment exports separate `android-subscription-private.txt`
and `windows-subscription-private.txt` files that identify the intended client.
The combined file is retained for compatibility.
Android must use its own `.json` URL: its released status resolver does not recognize `.windows.json`. In v1.21.2 both feeds use standard AWG/SS2022 URIs with identical keys/endpoints/options, letting older Windows/macOS readers skip unsupported entries safely. The other 22 native profiles retain their metadata; native CLI exports are unchanged. Android alpha18.1 and alpha20 accept the lossless URIs. macOS0.1.6 Beta keeps22 supported items from its existing .json URL; this is shared-reader execution on Windows, not physical Mac acceptance. Full output is 24 items for an eligible unlimited account; Windows 0.1.1 Beta imports 23 and 0.1.0 Beta imports 22. The installed 0.1.2-preview.5 supports all 24. This is reader compatibility, not a physical-device connection guarantee.
Status keeps the released schema and account/period IDs. The
released Android parser requires numeric `used_bytes` for a metered account,
even when counting is unavailable. That number is a persisted lower bound;
`partial`/`unavailable` and the last complete sample must remain visible.

## Accounting and revocation

Absolute Xray counters are not reset. Each private `.source` ledger atomically
stores its last reading, process epoch and cumulative per-account totals.
Upload and download are counted once; interface/NAT counters are not added.
An engine epoch change or counter decrease preserves known totals and marks
an unsampled interval permanently incomplete. A later successful read does
not repair unknown traffic. A crash before persistence can still lose the
final unsampled interval when the engine itself disappears.

New Xray user updates use a HandlerService API on a Unix socket under a root
0700 directory. Existing TCP statistics APIs remain statistics-only. Applied
snapshots prevent rejected additions/password changes from being exported.
Retries query actual engine users, handling a lost successful API reply.
AmneziaWG uses `syncconf` and stable per-user peer addresses without restarting
other peers. AWG peer traffic accounting is not implemented in this preview.

## Operations and rollback

### Updating a released panel or the accepted preview

Released versions use `sudo rovunet update`. Normal updates retain subscription
tokens, account/period IDs and the native profile IDs. Protocol counts depend
on installed services and account eligibility. Normal update adds missing managed cores and migrates old Hy2 to Salamander once; refresh the same saved subscription afterward. An explicit `core-hy2-obfs NAME off` choice survives future updates.

The accepted `1.20.1-managed-preview.3 (2ecfe96)` amd64 build intentionally blocks
the public updater. Its release asset
`rovunet-preview-to-v1.21.2-linux-amd64.tar.gz` contains a reviewed panel-only
bridge to v1.21.2. Run `sudo rovunet update` after promotion to reconcile pending setup/Hy2. Extract it into a private directory and run
`python3 upgrade-managed-preview.py apply`, followed by `status`. It verifies
the exact preview binary, creates a validated backup, retains existing core
keys/URLs, and checks all native fields before accepting the replacement.
Network services are not restarted; only two scheduler timers are paused and
restored. Do not run concurrent account/core changes. Optional `revert` restores
preview.3 without removing any core. Keep the original private preview backup.
The fresh-server `install.sh` is not the migration command for this preview.

Other private preview helpers below describe their original audited baselines;
they are historical staging tools, not general public updaters.

### Automatic installation and upgrade completion

Since v1.21.1, the standard installer and Quick Wizard use the same complete
preset. They configure both DNS/Soar backends and nine cores, yielding 24
Android and, since v1.21.2, 24 Windows subscription profiles for the supported single-address, unmetered
account setup. Use `rovunet install --custom` or `rovunet wizard --custom`
for a deliberately smaller set. The direct public SOCKS5 listener is not
needed by the full preset.

`rovunet update` runs automatic completion in the newly installed binary.
The existing 20-profile setup needs no manual four-core installation. Missing
base tunnels enter the same installer during an interactive update, asking
for their domain/DNS settings; noninteractive updates report the required
interactive step. Existing tunnels/users are kept. A private, validated
backup under `/etc/rovunet/setup-backups` precedes creating missing cores.
Retries preserve previously created keys and do not duplicate protocols.
The three new Xray protocols use `/usr/local/bin/rovunet-xray-managed`, verified
against the pinned source; the legacy Xray executable is preserved. Stored
`dedicatedXray` options retain this choice across lifecycle operations and
restore. Existing manually installed/new-preview cores keep their old runtime.

The final output reports actual per-user counts. Missing/invalid certificates,
occupied ports, unavailable TUN or disabled/missing tunnels produce a pending
setup message; incomplete setup is never announced as a completed 24-profile
installation. Resolve the stated prerequisite and run `rovunet complete-setup`.
Provider/cloud firewall rules and DNS records are outside panel control.
Traffic-limited or expired accounts never receive otherwise excluded protocols.

### Stable subscription URLs

The private `.3` display-name update uses the approved labels in
`internal/clientcfg/display_names.json`. Only native profile field 2 changes;
item IDs/order, collection identity, endpoints, embedded core files, passwords,
keys and subscription URLs retain their previous values. Tunnel/core tags and
on-disk state are never renamed. Custom IDs keep their configured display
names. Android still receives 24 profiles and Windows 22 for the audited
unmetered fixture. A normal subscription Update retrieves the new names.

The `.2` to `.3` package additionally compares every previously active feed
with its private snapshot before it accepts the upgrade. A change beyond the
approved name field aborts the transaction and restores the previous panel
and feeds while the configuration/core snapshots still match. This is an
in-place panel upgrade; the engines and their services are not reinstalled.

Normal panel/core updates rewrite the existing account's feed at the same
URL. Existing tokens, account IDs, period IDs and profile IDs are retained;
installing a protocol does not require sending every user another link.
New credentials are exported only after their engine update is confirmed.
The regression fixture models thirteen tunnels and five existing cores:
Android grows from 20 to 24 profiles, Windows from 20 to 22, with the old
URLs and twenty original profile records unchanged after reload and refresh.

Tunnel start/stop/edit/remove and completed restore now refresh subscriptions
immediately. Stop/start persists the tunnel's enabled state. Writers are
serialized within and across processes; an unexpected generation error keeps
the affected account's last valid feed and reports the failure. Intentional
account removal/token revocation still deletes the retired files.

Client fetching is independent of server file generation. Released Android
alpha18.1 can enable automatic updates for each private subscription. Its
WorkManager job checks about every twelve hours on an unmetered network with
sufficient battery and skips while Rovunet is connected. Manual Update also
requires disconnection. The server cannot force a saved Android client to
fetch immediately or enable that client preference. Retain the correct
platform URL; changing Android to the Windows feed intentionally reduces the
available protocols. An explicit token revocation/password reset or a change
of hosting domain is a separate operation from a normal panel update.

The private panel upgrade package uses `upgrade-managed-preview.py`
instead of reinstalling the managed cores. Each package accepts only its
audited baseline binary hash, saves a validated private backup, briefly pauses the expiry and
certificate timers and waits for their jobs. It replaces the panel atomically
and runs `subscriptions-refresh`, which bypasses legacy migration and only
publishes static feeds. Configuration bytes, core-file hashes and network
service PIDs/invocation IDs must match their pre-upgrade snapshots. Timers
return to their previous active/enabled states; no network service is started,
stopped or restarted. Do not run administrative account/core changes at the
same time as this short transaction.

An unsuccessful upgrade restores the previous panel. Feed snapshots are
restored only while the account configuration and core files still match;
concurrent legitimate account/key changes are never rolled back. A private
journal allows a repeated `apply`/`revert` to recover an interrupted attempt,
including timer restoration. `revert` in this upgrade package restores only
the previous panel binary and preserves the current cores, accounts and feeds;
it does not run the original preview removal command. The `.3` package targets
the audited `.2` panel and its revert restores `.2`. `status` is read-only
and reports aggregate counts without URLs or credentials. Production
installation and actual host reboot still require separate verification.

`core-start`, `core-stop`, `core-port`, `core-status`, `core-remove`, and the
existing user commands manage account-backed cores. Administrative stop is
persistent. Start preparation excludes expired, spent and ineligible accounts.
Port changes preserve keys and roll back the target core on failure. Firewall
ownership preserves preexisting/shared rules; no global flush or UFW enable
is performed. AmneziaWG scopes forwarding/NAT rules to its own interface and
subnet. Host-provider firewalls remain separate.

Backups include private external TLS files, applied state, stable keys and
source-ledger history. Restore carries valid renewal-source paths when
available. A certificate timer restarts only cores whose certificate changed;
that restart can disconnect their users. ACME renewal on a real server remains
an operational check. A restored certificate without an available ACME
source needs renewal configured on the destination host.

`scripts/managed-preview-server.sh` is specific to the audited v1.20.1 Linux
x86_64 server shape. It takes host/SNI as arguments, creates a private backup,
and installs four distinct names/ports without restarting existing protocol
services. Its rollback pauses accounting/certificate timers and bridges
absolute ledgers for the old reset-based collector. It preserves the current
epoch for that collector to count once, and archives the original ledgers.
An interrupted rollback keeps those timers disabled across boot; rerun the
same rollback command before account operations. Do not run concurrent panel
administration or manual engine restarts during deployment/rollback.

The private build blocks the public updater to avoid downgrading into a
binary that lacks its service commands. Use a reviewed package or rollback.

## Evidence and remaining checks

- Unit/behavior tests cover shared export policy, Soar output ordering,
  failed provisioning, account identity, counter failures, port ownership,
  backup paths and repeat installation preserving credentials.
- `TestManagedProtocolsRuntime`, with a verified Xray binary, exercises all
  three actual engines, two accounts, bidirectional payloads, user counters,
  expiry/new-session denial, password rotation, quota exclusion, API retries,
  private TLS/core backup restore, process restart and preserved ledger totals.
- `scripts/test-managed-awg.py` requires root and uses disposable network
  namespaces. It runs the production userspace driver, two peers, forwarding
  and NAT, then tests peer removal, unaffected peer traffic, cleanup and restart.
- Rollback accounting has an isolated test for live-epoch subtraction,
  stopped-core totals, idempotency and recovery of an interrupted bridge.

These checks are not phone import, public-Internet throughput, host reboot,
long-run/Doze, high concurrency, battery or independent SS2022 UDP evidence.
The owner exercised staged server installation/rollback, subscription refresh
and the approved display names. Those reports do not establish whole-host
reboot, long-run traffic or end-to-end ACME renewal. Release CI validates the
exact published source and runtime assets. The four README translations cover
v1.21.1; app source is unchanged by this panel release.
