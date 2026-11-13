# Engineering work ledger — 2026-11-12

Latest status reconciliation: 2026-11-13. See [full-VM, client and custody evidence](engineering-acceptance-2026-11-13.md). All24 original task IDs and acceptance criteria remain; publication gates stay open.

Baseline: `2601c1a9cce2fb718e18a051663b81c115f20daa`, clean `main`, latest GitHub API release `v1.21.1` (25 assets).

Working branch: `codex/security-reliability-20261112`. Initial review had no live server inspection; Nov13 owner-opened authenticated SSH/SFTP supplied the separately scoped network addendum. Source, mock, real-engine loopback and device evidence are separate.

User task document SHA256: `16185a337b4dcf06b2d277336afb684de78280c98bb1d93e9918efce0d52844f`. Historical prompt and conversation remain the source for the OLD rows. Existing explicit device-cap cancellation remains in force.

No new publication/deployment or permanent live host firewall/SSH/credential changes are included. The bounded transient Hy2 diagnostic was cleaned up. Production signing identity, dedicated reboot VM and matching client/device tests are explicit gates, not reasons to stop independent local repairs.

Estimates are person-days, not delivery promises; they overlap as indicated and exclude external waiting. P0 means a release gate in this task, not an asserted critical vulnerability.

## OLD-01 — P1

Origin: Previous prompt sections 2, 6-11.

Manage AWG, SS2022, Trojan TLS and VLESS Encryption with per-account private credentials, install/start/stop/remove, certificate renewal, restore and upgrade. Preserve existing tests and services.

- Status: implemented; real nine-core VM and five controlled Windows payloads passed; phone/native C gates open
- Evidence / confirmed cause: Managed Xray loopback; actual AWG namespace TCP/UDP/revoke; actual CLI 20-to-24 preserved old core/runtime/account state. Nov13: full Ubuntu VM nine services survived normal reboot and abrupt power cut with same credentials; Windows pinned native engine transferred140800 bytes per case for WG/Hy2/Trojan/VLESSenc/SS2022; new AWG two-peer revoke/reconnect passed. Subsequent WAN: existing AWG passes installed-preview HTTPS only after a failed cold request; do not generalize immediate connection or physical-device support.
- Dependencies: Real reboot and supported client/device.
- Acceptance: Private credentials per account; no cross-account export; correct listeners, firewall and idempotent rollback; timed eligibility; no unvalidated quota access.
- Next: Physical released phone, WAN/UDP/carrier and source/ABI promotion review.
- Rollback: Reviewed previous release plus compatible config/usage snapshot.
- Effort: 0.5-2 remaining validation. Person-days estimate; external waiting excluded, shared work not additive.

## OLD-02 — P0

Origin: Previous prompt sections 11-12.

One eligibility policy for provisioning, CLI, export-clients and subscriptions; sum upload/download once across paths; retain history, account/period IDs, renewal and unavailable/partial reporting.

- Status: implemented with restrictive policy; hard-limit/timer proof incomplete
- Evidence / confirmed cause: Go accounting/policy/history suites plus six real-engine protocol modes; new-session removal measured, established streams survived for the new three. Nov13 latest Xray repeats per-account data/counters/new-session revoke; existing streams still survive in all three added Xray protocols. Timer delay and quota overshoot remain null; excluded quota paths stay excluded.
- Dependencies: OLD-01; isolated real engine.
- Acceptance: Quota paths match server credentials and metering; reset/crash/read failure never fabricates zero; open-session limitations explicit.
- Next: Measure timer/overshoot and retained quota paths; do not enable excluded paths on the basis of a new engine tag.
- Rollback: Keep restrictive eligibility; restore usage with supported recovery tools.
- Effort: 1-3 remaining. Person-days estimate; external waiting excluded, shared work not additive.

## OLD-03 — P1

Origin: Previous prompt section 13.

Complete Soar direct/SSH before final setup summary; one structured complete subscription after successful provisioning; no failed, absent or account-ineligible path advertised ready.

- Status: implemented; full-VM actual systemd completion passed; public DNS/Soar acceptance incomplete
- Evidence / confirmed cause: Private namespace real panel/engines gives Android24 / Windows22; repeat no-op, existing20 links and credentials retained; systemd simulated. Nov13 actual panel plus nine real engines/systemd:20-to24 Android/22 Windows; existing20 links, five core files/PIDs and cfg unchanged; one backup; repeat all-nine no-op.
- Dependencies: Missing public DNS/TLS on fresh VPS.
- Acceptance: Eligible full fixture has Android 24 / Windows 22; complete setup does not pretend missing prerequisites succeeded.
- Next: Public DNS/TLS and final fresh-host/Soar TTY flow remain separately gated.
- Rollback: Existing backups and missing-only retry.
- Effort: 0.5-1.5 remaining. Person-days estimate; external waiting excluded, shared work not additive.

## OLD-04 — P0

Origin: Conversation: old subscription updates without resending URL.

Refresh existing Android .json and Windows .windows.json URLs automatically; retain schema v1, IDs, native fields, tokens, status aliases, account identity and locally relevant semantics.

- Status: implemented; same-URL full-VM export and matching released readers passed
- Evidence / confirmed cause: 20-to-24 retained token/URI/IDs; stale timer/token-rotation tests and actual path-containment CLI regression passed. Nov13 actual VM20-to24 retained URL tokens and all20 previous links byte-for-byte; exact released Android v1 reader accepted24, Windows accepted22 conservative/23 full. Explicit Hy2 mode toggle on the full VM retains saved URL tokens, item counts and auth; only the one Hy2 link per feed changes and off restores exact old links.
- Dependencies: Client fetch behavior owned by client.
- Acceptance: Ordinary upgrade/refresh keeps links and identity; transient feed error preserves last valid feed; explicit revocation retires old files.
- Next: Physical APK fetch/update and address/group identity migration remain distinct open cases.
- Rollback: Keep old feed until valid atomic replacement.
- Effort: 0.5-1 remaining. Person-days estimate; external waiting excluded, shared work not additive.

## OLD-05 — P1

Origin: Conversation: apply all proposed display names only.

Apply approved names to 24 profiles, including legacy xhttp/hysteria2/soar-direct aliases; change native field 2 only; preserve IDs/order/keys/URLs and custom names.

- Status: complete; approved display-name regression retained
- Evidence / confirmed cause: Existing mapping/name-only/native-field regression suites passed; no second rename.
- Dependencies: None.
- Acceptance: Name-only tests and legacy fixtures pass.
- Next: Retain approved mappings and custom names.
- Rollback: Revert only label mapping.
- Effort: 0. Person-days estimate; external waiting excluded, shared work not additive.

## OLD-06 — P0

Origin: Conversation: both fresh installation and update must add four missing cores.

Default full install/Quick Wizard/update produces full eligible setup, preserves users, secrets, existing core choices and old URLs; no four manual additions on each server.

- Status: implemented; automatic missing-four completion passed on actual full VM
- Evidence / confirmed cause: Actual20-to24 namespace test passed; old services/processes/runtime/config preserved; full VM/fresh server prerequisites remain. Nov13 removed only four owned lab cores to recreate20, then actual complete-setup installed missing four; old five unchanged and repeat idempotent. No live VPS changed.
- Dependencies: Real host prerequisites and user upgrade.
- Acceptance: 20-to-24 and repeat no-op; missing prerequisite visibly pending; no keys/users regenerated.
- Next: Fresh public DNS/TLS bootstrap and second production server acceptance remain unknown.
- Rollback: Setup backup plus reviewed restore; do not revert usage blindly.
- Effort: 0.5-2 remaining. Person-days estimate; external waiting excluded, shared work not additive.

## OLD-07 — P3

Origin: Conversation: publish 1.21.0, README and amd64; subsequent automatic-setup update.

Publish approved prior releases with both Linux architectures and updated four-language documentation.

- Status: complete for prior authorized public releases; no new publication
- Evidence / confirmed cause: Publicv1.21.0/v1.21.1 history retained, v1.21.1 25 assets; this mission prepares only private commits/build/review files.
- Dependencies: None.
- Acceptance: Published tag and asset evidence retained. This does not authorize a new release.
- Next: No new tag, push or release in this review.
- Rollback: Never replace bytes of a published tag.
- Effort: 0. Person-days estimate; external waiting excluded, shared work not additive.

## OLD-08 — P1

Origin: Previous prompt section 15.

Correct TCP/UDP ports, collision checks, real config validation, persistence and rollback; preserve SSH/unrelated firewall, stopped cores, certificates, WARP and backend privacy.

- Status: partial; actual reboot/power-cut/SSH/certificate checker passed; WARP blocked
- Evidence / confirmed cause: Stopped service preservation, atomic Caddy validation and owned SSH text tests passed; actual DNS systemd start/restart passed. Nov13 actual nine-core VM, private-CA leaf reissue and scoped three-core restart; real sshd no-shell forwarding/selective revoke/admin isolation; WARP stopped at provider registration with cleanup.
- Dependencies: Dedicated VM reboot, cloud firewall and valid DNS/cert.
- Acceptance: Each claimed service survives restart/reboot/cert renewal and restores keys/consumption without touching unrelated rules.
- Next: Reachable WARP registration/data path, public ACME, external firewall and full base-tunnel restore.
- Rollback: Validated private backup and unit/rule snapshots.
- Effort: 2-4 remaining. Person-days estimate; external waiting excluded, shared work not additive.

## OLD-09 — P1

Origin: Previous prompt sections 10, 16; repo docs status/quota-access.

Prove sustained multi-user behavior, Doze/battery/idle data, UDP, network recovery, real reboot, client import and actual data; separate user report, loopback, mock and hardware evidence.

- Status: partial; WAN failure diagnosed, Salamander Hy2 repair and rollback prepared; phone/Mac and AWG cold start open
- Evidence / confirmed cause: Bounded DNS5000+100 lab, actual systemd2x200 DNS requests, Xray loopback and AWG namespace data; no physical-device/Doze claim. Nov13 Windows native released Xray and generated configs passed five payload cases over private VM forwarding without host routing/proxy changes; Android native v1 envelope reader passed on JVM only. Owner-authorized live WAN evidence: standard WG/Hy2 fail on two VPS paths, keys/cert match, small/large raw UDP reaches the second VPS; WG response leaves server but is not received by client. Installed Windows preview reproduces WG; transient Salamander Hy2 passes2/2, plain fails2/2. The panel now offers explicit opt-in mask switching with actual VM validation/startup rollback proof; no production switch.
- Dependencies: Dedicated VM, representative devices/networks and matching client owners.
- Acceptance: Record attempts/failures, durations, p50/p95, reconnect, other-user impact; no fabricated pass for skipped cases.
- Next: Physical Android/Mac Salamander and AWG cold-start acceptance, carrier/long-session behavior; no silent protocol substitution.
- Rollback: Isolated experimental environment only.
- Effort: 2-4 plus external waiting. Person-days estimate; external waiting excluded, shared work not additive.

## OLD-10 — P1

Origin: Previous prompt section 14; docs/ROVUNET-STATUS.md.

Keep strict schema/native links, correct applied SNI/port/address/cert and special characters, IPv6, stable IDs/grouping, no server private seed export, status contract complete/partial/unavailable; verify stable identity across address changes.

- Status: partial; strict v1 preserved; native grouping/address-order migration open
- Evidence / confirmed cause: Versioned out-of-band matrix, pinned client-source review, strict generated-feed tests and oversized-feed retention; existing status IDs kept.
- Dependencies: Client source readers and explicit fixture ownership.
- Acceptance: No unsupported effective setting silently dropped; v1 readers remain compatible; changing address does not remint server/account IDs.
- Next: Run matching released readers; plan compatible native grouping and reorder-independent address IDs.
- Rollback: Keep existing schema v1 and platform filters.
- Effort: 1-3 remaining. Person-days estimate; external waiting excluded, shared work not additive.

## CANCEL-01 — cancelled

Origin: Conversation: device cap 15; later explicitly cancelled.

Per-user concurrent device limit default 15. User explicitly said to leave it. Do not reactivate or relabel IP/socket/session as a device.

- Status: cancelled by user; unchanged
- Evidence / confirmed cause: No device/IP/socket cap added.
- Dependencies: New explicit request required.
- Acceptance: No device enforcement claimed or enabled.
- Next: Only a new explicit request can reactivate it.
- Rollback: N/A
- Effort: 0. Person-days estimate; external waiting excluded, shared work not additive.

## NEW-01 — P0

Origin: New prompt 3.1 and appendix 0-3.

Inventory complete executable supply chain; pin installer and runtimes; authenticated manifest with independent trust anchor; verify all executable bytes, architecture, digest, license and provenance. Reject missing/invalid/tampered/incomplete artifacts. Test key must not work in production.

- Status: partial; real local production identity/custody works; publication P0 remains open
- Evidence / confirmed cause: Ed25519 verifier, empty production root, signed exact-assets bootstrap/updater, pinned transport catalog, legacy setup helper; 14-case ephemeral-key bootstrap and release-authority tests passed. Runtime source/license and advisory gaps are explicit. Nov13 actual panel root provisioned on owner instruction; DPAPI signing + age backup + three fresh local recoveries and independent OpenSSL signature verification passed. Fourteen candidate runtimes have source/patch/notice records; legacy/native/ABI review unfinished.
- Dependencies: Production signing identity/trust-root distribution; upstream license/source investigation.
- Acceptance: Tamper/wrong arch/wrong signature/missing verification/truncation never execute; same panel version never follows mutable upstream latest.
- Next: Actual offline/off-site copy and independently reviewed root delivery; finish runtime promotion review.
- Rollback: Old released install remains available; new path must not weaken to unsigned fallback.
- Effort: 2-5 remaining plus authority/source waiting. Person-days estimate; external waiting excluded, shared work not additive.

## NEW-02 — P0

Origin: New prompt 3.2.

Transactional bootstrap/update/recovery: download+verify before replacing or stopping; no global killall; atomic files, backup, interruption/disk/network/post-update failure; preserve users, credentials, URLs, usage, custom settings and stopped services. Migration changes only owned path fields.

- Status: partial; actual VM power-cut and core recovery passed; full cross-service transaction unproven
- Evidence / confirmed cause: Atomic durable replace/journals, retained recovery copy/snapshot, strict pending update, stopped-service preservation, owned-path migration, stale-writer rejection; 18 preview recovery and rollback suites retained. Actual CLI now proves invalid legacy-layout restore/check/keygen/version do not migrate, and authenticated confirmed legacy restore migrates successfully. Nov13 real KVM guest recovered all nine engines after normal reboot and abrupt QEMU power cut, preserving config/keys/tokens. Stale API socket and recursive chown bugs reproduced and fixed. Encrypted nine-core restore, actual SSH and cert checker passed. New Hy2 switch validates before mutation, keeps a private recovery copy and passed actual missing-cert and forced-start failure rollback; full multi-operation concurrency/crash reconciliation remains open.
- Dependencies: NEW-01; ownership snapshots; isolated fault injection.
- Acceptance: 404/504/truncation/no space/interrupt/repeat/post failure leave usable binary or explicit compatible recovery; no false success; existing recovery tests retained.
- Next: Base DNS/WARP/public ACME restore and failure/concurrency across all services; no atomic-all-services claim.
- Rollback: Retain binary plus validated config/usage/unit snapshot; no automatic stale usage rollback.
- Effort: 1-3 remaining VM cases. Person-days estimate; external waiting excluded, shared work not additive.

## NEW-03 — P0

Origin: New prompt 3.3; appendix 1.

Versioned out-of-band capability and fixture contract per client version/architecture/engine/options/network/account; distinguish import, config, runtime, connection, payload, metering and revoke. Preserve strict schema v1; do not change other owners files.

- Status: partial; matching released reader and five native Windows payload cases passed
- Evidence / confirmed cause: No extra v1/native fields; pinned Android reader source; capabilities-v1.json distinguishes client/ABI/options/account/evidence, Windows SS2022 import vs payload and AWG absence. Nov13 exact Windows Beta source00f6fe8 and source ZIP checksum; native reader22/22 and23/24; matching checksum-pinned Windows Xray passed WG,Hy2,Trojan,VLESSenc,SS2022. Exact alpha19 Kotlin v1 reader24 + seven malformed rejects on JVM; no phone. Current installed Windows0.1.2-preview.5 was checked independently of public Beta: native AWG path passes with cold retry; ordinary WG fails; Salamander Hy2 passes WAN. No blanket downgrade of new client capabilities to the old public matrix.
- Dependencies: Client-owner hardware confirmation for AWG dialect/Windows SS2022.
- Acceptance: Actual feeds accepted by matching strict readers; unsupported with reasons; counts from actual records; no schema field additions without compatibility proof.
- Next: Physical APK/Mac data path, protocol-aware current-client negotiation and remaining transport/Doze cases.
- Rollback: Retain existing v1 and conservative platform filters.
- Effort: 1-3 remaining shared with OLD-10. Person-days estimate; external waiting excluded, shared work not additive.

## NEW-04 — P1

Origin: New prompt 4.1.

Validate direct Soar shared public-key trust vs SSH per-account revoke; preserve timed/quota restrictions; SSH admin isolation/forwarding/validated config; strict public SOCKS empty users rejects auth, credentials change/delete closes only affected streams.

- Status: partial; actual restricted sshd and selective session removal passed
- Evidence / confirmed cause: Existing strict SOCKS/stream credential tests retained. AddSSHUser/removal/session kill refuse unmanaged/admin/UID0 accounts; owned SSH blocks preserve foreign rules; explicit password change targets only owned sessions. Nov13 real guest OS users/password authentication + two SSH forwarding sessions: no PTY/shell, existing admin password policy preserved, target removed while other account survived; elapsed7.419s. Shared Soar direct restrictions retained.
- Dependencies: Real SSH/Soar environment for end-to-end gates.
- Acceptance: Wrong/no credentials, password change, deletion/last user, limiting account and old saved configs tested; direct and SSH results separate; admin/other users retained.
- Next: Public Soar/SSH/SOCKS end-to-end, timer removal and provider/network cases.
- Rollback: Preserve direct configs unless operator explicitly rotates shared key.
- Effort: 1-2 remaining. Person-days estimate; external waiting excluded, shared work not additive.

## NEW-05 — P1

Origin: New prompt 4.2.

Protocol-specific counting/session termination matrix; real-engine open stream tests and measurable quota expiry delay/overshoot; retain conservative eligibility; no collateral shared-core restart without disclosure.

- Status: partial; latest-candidate data/counter/revoke revalidated; strict quota remains unproved
- Evidence / confirmed cause: Actual six SS2022/Trojan/VLESSenc modes pass; counter/new-session/other-account behavior measured; established streams remain alive; timer SLO/overshoot null, conservative exclusions unchanged. Nov13 real candidate Xray three protocols: bidirectional payload, distinct counters, shared idempotent total, selective new-session block, other account healthy and private core/TLS restore passed. Established streams survive; scheduler SLO/overshoot null.
- Dependencies: Controlled rate lab and engine availability.
- Acceptance: Expiry/quota/renew/restart/restore/read failure/concurrent accounts preserve totals; measured delay and other-account impact; report soft limit honestly.
- Next: Scheduled timer and quantitative overshoot on retained quota paths; design before admitting unsupported hard-quota protocols.
- Rollback: No enablement of quota for unsupported protocols.
- Effort: 2-4 remaining. Person-days estimate; external waiting excluded, shared work not additive.

## NEW-06 — P1

Origin: New prompt 4.3 subscription.

Token lifecycle/explicit rotation independent of password, removal/status aliases/no listing/no-store; regular renewal/update keeps URL; inventory private logs and retention with redaction.

- Status: implemented locally; operational retention/deployment review remains
- Evidence / confirmed cause: Independent rotate_link, strict active/archived tokens, stale-writer and retired-file regressions; historical path escape reproduced then prevented; diagnostics no longer auto-dump journal. Privacy/opt-in retention documented.
- Dependencies: Operator choice only for explicit rotation.
- Acceptance: Old rotated URL removed; stable refresh/renew; URLs/tokens/passwords absent in ordinary diagnostics/errors/logs.
- Next: Host log-policy and real HTTPS no-store/listing behavior on full VM; no forced token rotation.
- Rollback: Existing token preserved until explicit rotation commits.
- Effort: 0.5-1 remaining. Person-days estimate; external waiting excluded, shared work not additive.

## NEW-07 — P1

Origin: New prompt 4.3 backup.

Authenticated encrypted backup export using maintained library; safe key handling/recovery; default/private atomic writes; explicit plaintext choice; old formats/decompression/path/permission validation retained.

- Status: partial; encrypted actual nine-core restore and protected signing-key recovery passed
- Evidence / confirmed cause: age1.3.2, private key handling, atomic output; actual CLI wrong-key/tamper/truncate rejected before mutation, authenticated IDs/tokens/usage restored; partial restore exits nonzero. Full bundle tests retain keys/core/history. Actual CLI now proves invalid legacy-layout restore/check/keygen/version do not migrate, and authenticated confirmed legacy restore migrates successfully. Nov13 age wrong key rejected before mutation; actual guest OS/core restore preserved account IDs/tokens/passwords and root-private unrelated recovery-key owner. Local DPAPI production signer recovered three times into fresh vaults using portable age backup.
- Dependencies: Maintained library and operator key storage.
- Acceptance: Wrong key/tamper fail before mutation; interrupted export keeps old file; roundtrip retains all keys/IDs/usage; secrets absent from argv/logs.
- Next: Offline/off-site signing kit and full base DNS/WARP/usage-forward reconciliation acceptance.
- Rollback: Legacy plaintext still decodes; encrypted key retained separately offline.
- Effort: 0.5-1 remaining. Person-days estimate; external waiting excluded, shared work not additive.

## NEW-08 — P1

Origin: New prompt 5 DNS.

Bound UDP workers/queue/pending and measure query/response ratio, CPU/RAM/concurrency; retain TCP caps/timeouts; no per-resolver device assumption; local controlled flood only.

- Status: implemented resource bounds; controlled loopback measurements passed
- Evidence / confirmed cause: Bounded UDP workers/queue/global+backend pending/sockets/TCP-response memory/rates;5000 bounded burst then100/0 healthy same resolver; accepted idle TCP closes on shutdown.
- Dependencies: Isolated loopback/namespace lab.
- Acceptance: Resource ceiling holds under bounded overload and healthy shared-resolver traffic recovers; publish scenario/attempts/latency and unknowns.
- Next: Real DNS-tunnel/HMAC/MTU/amplification and sustained host resources; synthetic echo ratio not generalized.
- Rollback: Revert router code and restart only router; no host firewall changes.
- Effort: 1-2 remaining. Person-days estimate; external waiting excluded, shared work not additive.

## NEW-09 — P1

Origin: New prompt 5 systemd.

Service-specific least privilege review (users/writepaths/caps/TUN/low ports/DNS/certs/WARP); add only tested restrictions; prove restart/reboot and data.

- Status: partial; narrow DNS confinement and real full-VM reboot passed
- Evidence / confirmed cause: Per-service requirements table; actual NoNewPrivs=1 and200 UDP/TCP requests before/after owned systemd restart. No blanket AWG/TUN/certificate/WARP restrictions. Prior actual DNS NoNewPrivs/restart evidence retained. Nov13 root ownership of configuration parents and nonrecursive EnsureDir preserve unrelated private keys; no claim all old child ACLs or every service capability is migrated.
- Dependencies: Dedicated systemd VM for reboot/renew/WARP proofs.
- Acceptance: Restrictions must not break AWG/TUN/NET_ADMIN or cert reload; systemd score alone insufficient.
- Next: Review each TUN/WARP/native backend confinement and pre-existing child ownership separately.
- Rollback: Restore prior owned unit; daemon-reload and restart only affected service.
- Effort: 2-4 remaining. Person-days estimate; external waiting excluded, shared work not additive.

## NEW-10 — P1

Origin: New prompt 5 maintenance.

Pinned runtimes/provenance/licenses/advisory monitoring, SECURITY.md, threat model/privacy inventory/private report policy and realistic response; complete CI shellcheck/shfmt/integration gaps.

- Status: partial; source-built candidates, notices and Go/Rust scans pass within scope; promotion open
- Evidence / confirmed cause: SECURITY/threat/privacy/recovery/provenance docs; pinned actions/tools; all shell scripts linted/formatted; official332-record mirror and panel source0 findings after crypto update; runtime findings explicitly not clean. Nov13 fourteen amd64/arm64 candidate assets, nine base source archives/patches,348 notice records; official DNSTT CLI compatibility patch; crypto/net/sys/compress/brotli/Tokio updates. Five attributable Go source scans and panel scan zero findings; Caddy/Xray root OSV ranges separately reviewed; RustSec zero findings. Native C/older-Linux ABI unknown.
- Dependencies: Confirmed source/licensing and public advisory evidence.
- Acceptance: No speculative CVE claims; maintain both legacy/dedicated engines; CI checks relevant changed behavior, scripts formatted/linted.
- Next: C/OpenSSL/picoquic/picotls review, compatible Slipstream builds, full client transport acceptance and GitHub CI after authorization.
- Rollback: Revert CI/docs only; engine pins do not silently change.
- Effort: 1-3 plus upstream reconstruction. Person-days estimate; external waiting excluded, shared work not additive.

## NEW-11 — P2

Origin: New prompt 6; appendix 2.

Stabilize present VLESS variants/SS2022/Hy2/WG with actual client capability; evaluate missing VMess/other variants by need; then AnyTLS/TUIC, later Mieru/AWG variants. No URI-only support claims.

- Status: gated experiment; no new public protocol family enabled
- Evidence / confirmed cause: Android alpha19 now has constrained AnyTLS/TUIC/Mieru adapter per pinned source/provenance; panel does not deploy them. WG/Hy2 own-panel failures not declared fixed. Capability/acceptance prerequisites recorded. A bounded real WAN experiment justified existing Hy2 Salamander support; no replacement protocol family enabled and no WG profile silently replaced.
- Dependencies: NEW-03, OLD-09, client-owner maintained engine/testing.
- Acceptance: Each new experiment has auth/quota/revoke/service/log/backup and compatible client plus controlled payload test before public support.
- Next: Stabilize existing clients/options/data first; maintained exact-source experiment only for a justified need.
- Rollback: Feature remains off until acceptance and release approval.
- Effort: 3-8 per justified family; not a schedule. Person-days estimate; external waiting excluded, shared work not additive.

## NEW-12 — P2

Origin: New prompt 6; appendix 3.

443 multiplexing correct/invalid SNI/ALPN, renewal/reload and dependency failures; IPv6/multi-IP stable IDs/routes/eligibility/no duplicate profiles; no blanket random SNI/ports or forced token rotation.

- Status: partial; lab certificate lifecycle and private endpoint mapping passed; public address migration open
- Evidence / confirmed cause: Existing front/parser/address/eligibility tests pass; extra-address build errors now retain the prior valid feed; >1000 items rejected. Native grouping and positional address IDs remain known gaps. Nov13 private-CA leaf replacement restarted exactly hy2,managed-trojan,xh; other six processes preserved. Windows payload used explicit test endpoints/CA, with system trust and default routes unchanged.
- Dependencies: Valid DNS/cert and IPv6 VM/network for external proofs.
- Acceptance: Local tests cover invalid routing and stable dedup; hardware/network outcomes explicitly separate.
- Next: Real SNI/ALPN/IPv6/multi-address reordering, public ACME and WARP routing.
- Rollback: Preserve existing SNI/port/routes; no automatic reconfiguration.
- Effort: 2-4 remaining. Person-days estimate; external waiting excluded, shared work not additive.

## NEW-13 — P3

Origin: New prompt P3 and 8; appendix 4.

Keep README layout/language order, accurate limitations/channel/docs; badges/topics/templates after functional work; deliver per-item old/new status, evidence, hashes/commits, rollback, estimates and at most three next priorities.

- Status: partial; review.2 source/build/evidence/custody handoff prepared; public release held
- Evidence / confirmed cause: All prior/new rows retained with status/evidence/dependencies/acceptance/rollback/remaining effort; four-language notes, security docs, local source/build/report bundle. No release/deploy/account-key rotation. Nov13 bilingual evidence report, updated four-language README notices, all24 backlog rows retained, new source/runtime archives and protected Windows key-recovery guide. Review artifacts do not authorize deployment or publication. Redacted WAN evidence, explicit rollback instructions and installed-preview source/engine identities are included in the final local review.
- Dependencies: All independent work and validation results.
- Acceptance: No item silently dropped; no complete security audit/global protocol-health claim; no unapproved release/deploy.
- Next: Finish open engineering/physical/provider gates; optional presentation extras remain unstarted.
- Rollback: Local commits/patch are reversible; no published mutations.
- Effort: 0-0.5 packaging/review. Person-days estimate; external waiting excluded, shared work not additive.

## Additional confirmed findings linked to the task rows

| ID / priority | Reproduction / cause | Resolution / evidence | Remaining gate / rollback |
|---|---|---|---|
| FOUND-01 / P0 | Historical v1.21.1 accepted a ../ token in config/backup and wrote outside the subscription subdirectory in a disposable lab | Strict128-bit hex active/archived token validation; actual old/new CLI differential test | Specific path-containment defect fixed; no hostile-root/full importer audit claim; revert only with a reviewed equivalent boundary |
| FOUND-02 / P0 | Stale config save and omitted JSON fields could restore retired tokens/users | File lock+compare-before-save, complete reload and serialized refresh; lost-update/revocation tests | Specific file-state races fixed; multi-service/account operation transaction remains OPEN under NEW-02 |
| FOUND-03 / P0 | Legacy setup fetched moving Go/xcaddy/plugin code and deleted global toolchain before verification | Fixed-path reviewed runtime helper, pinned digest/ELF; removed mutable toolchain build and unrelated cleanup | Same upstream provenance/advisory gate as NEW-01; management menu retained |
| FOUND-04 / P1 | Restore printed complete despite failed user/usage/service steps | Warning-aware nonzero pending restore; actual encrypted CLI counter-storage fault test | Full VM recovery still OPEN; retained pre-restore snapshot and forward repair |
| FOUND-05 / P1 | SSH upsert could target an existing non-panel account; password change left existing sessions | Owned group/nonzero UID guards and targeted explicit-session termination; mocked OS-command behavior tests | Real sshd/PAM forwarding and failure reconciliation OPEN; no routine-update session kill |
| FOUND-06 / P0 | Legacy automatic CLI pre-run migrated state before backup authentication | Defer restore migration until authentication and confirmation, then reload; backup-check/keygen/version bypass migration; actual old/new namespace CLI reproduction | Specific ordering defect fixed; complete VM migration/recovery remains OPEN under NEW-02 |

No open P0 is closed by limiting a claim: production trust, runtime provenance/advisory triage, full VM recovery and matching released client acceptance remain in docs/release-gates.json. A successful compiler or loopback test does not close them.

See [subsequent owner-authorized WG/Hy2 WAN checks](network-diagnosis-2026-11-13.md); permanent production state was preserved.

## Owner-authorized functional release follow-up — 2026-11-13

After receiving the private review and open gates, the owner requested GitHub publication and updated landing READMEs for normal install/update and same-URL app refresh. v1.21.2 adds the current-Windows24 feed and one-time Hy2 Salamander migration with a persistent explicit-off choice. Older Beta compatible import is checked separately. The four broader gates remain open and candidate runtime promotion/quota expansion remain excluded; see releases/v1.21.2-scope.json. Historical counts/evidence above retain their original dated scope.

The final integration handoff is capability-based, independent of moving client version numbers. The owner requests the current functional release now and asks that app foundations for the next version be recorded separately. Mac0.1.6 shared Core originally rejected the Android native AWG envelope; both feeds now use lossless AWG/SS2022 standard URI encodings while retaining the other 22 native items and native CLI output. Older readers safely retain supported items. Full physical-client/runtime qualification remains open.
