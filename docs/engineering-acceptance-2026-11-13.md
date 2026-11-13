# Private acceptance evidence — 2026-11-13

The owner authorized creating panel signing/recovery identities and storing them in Windows. This review changes no client repository, permanent live-VPS configuration, public GitHub tag/release, CI secret or existing subscriber identity. An owner-authorized bounded transient live-VPS diagnostic was cleaned up; see the network addendum. Public v1.21.1 remains unchanged. All24 prior and added tasks remain in the linked ledger; canceled device caps stay canceled.

## Production identity and recovery

The Ed25519 production root is provisioned: SPKI SHA256 **885d8aba22365ce42e20527bfa00e82aed3cba2f69501f146ed0496a8bc3dcbe**. Go verifier and bootstrap share this production-only root. A separate public record is retained in the owner's Windows desktop; key/recovery storage is disclosed only in the private handoff.

Current-user DPAPI protects PKCS8 in Windows. A separate X25519 age identity authenticates/decrypts the portable encrypted backup. Three fresh destination-vault recovery drills, wrong-expected-root rejection, Windows DPAPI/age/tamper/manifest boundary tests and an actual production-key signature verified independently by OpenSSL passed. The signing challenge is explicitly a local custody check, with no release/deployment. Portable recovery still requires the independently retained expected public key ID and a private destination. No execution-policy bypass, Windows trust-store import or hardware-signer claim.

The local second drive is not certified offline/off-site; physical separation, custody loss/compromise response and independently reviewed initial root delivery remain open. The portable recovery identity itself is private and must stay out of the source/review archive. No CI key was provisioned.

## Full guest VM, not service mocks

An official Ubuntu24.04 cloud image was verified against its signed checksum file, with cloud-image key fingerprint D2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81. The owned KVM/QEMU guest uses kernel6.8.0-147-generic, real systemd/sshd, private host-key pinning and synthetic fixturealice/fixturebob accounts. It has nine real protocol-core services. Thirteen base tunnel entries are synthetic prerequisites for export and are not a claim every public DNS/Naive transport was deployed in the guest.

- Actual CLI20-to24 completion preserved all previous20 link bytes, subscription tokens, cfg bytes, existing five core files/processes; installed only missing four; created one private backup. The second call kept all nine process IDs/state.
- Both normal guest reboot and abrupt owned-QEMU power cut recovered all nine services with config/credential hashes, server ID and observed usage state preserved (empty usage ledger in this VM scenario; nonzero accounting recovery is covered separately by real-engine/backup tests). Recovery timings are recorded per attempt in the packaged full-vm-reboot evidence, one complete normal/power-cut sequence per run; no live VPS or Windows reboot.
- The power-cut test reproduced stale persistent API sockets. Prepare now checks the exact owned path/private parent, socket type and actual liveness, then removes only the stale refused socket. A live listener/file/symlink is preserved.
- Actual encrypted nine-core/OS restore rejected a wrong key before mutation, restored credentials/IDs/tokens and preserved unrelated root-private key ownership. Recursive chown was the confirmed fault; EnsureDir now changes only the requested root-owned parent without dereferencing. Existing descendants are not recursively repaired.
- Actual two-account SSH password authentication/forwarding denied shell/PTY, retained the administrator's existing password policy, removed the target account/session and preserved the other account's session; measured operation/probe elapsed7.419s.
- A private-CA leaf reissue plus actual certificate-checker service restarted exactly hy2,managed-trojan,xh and preserved six other processes. This is not a public ACME renewal or timer-delay measurement.
- Actual WARP provider registration failed at register WARP before data acceptance. Cleanup was verified; provider payload is false, not zero-latency success. Full base DNS/WARP/public ACME restore and concurrent multi-service operations remain open.

## Exact released clients

Android alpha19 source build e8c93787fc4703cf4fea6c52ae056fae2724b105, source ZIP SHA2562ed77f73f71476b7e7947d39448415edaf27745b3449a639276b6d2ae3efb9a5. Unmodified SubscriptionPayload.kt/WireBase64.kt executed on JVM using Android org.json. The24 native-v1 items and seven malformed rejection cases passed. The non-native JSON-document stub rejects use; this is not an APK, engine-admission, phone or UI test.

Windows0.1.1 Beta source00f6fe8fbcf63fa2c0c36ad364983a11890faef1, source ZIP SHA256f546b19ed520a85cf37754e97764d72e3081be40d97b0ee3da8e931d9ff3fa03. Unmodified released Core C# compiled in an independent harness:22/22 conservative feed,23/24 full feed, seven malformed cases rejected. AWG is the unsupported full-feed item.

The exact pinned released Windows Xray archive (SHA2562e04a9995a6f2ad1b28c0bb25c708ecb2695e43a1c9c7889c4b6ac2b48cf7a6a) and generated effective configs passed five real Windows-to-guest payload cases: Trojan,VLESSenc,Hy2,WireGuard,SS2022, each140800 bytes with exact echo hash. Endpoint mappings and a private test CA were explicit; TLS cases verified CA/hostname with no allowInsecure; non-TLS protocols use their native authentication/encryption. Windows global routing, proxy, firewall and root trust stayed unchanged.

This proves the controlled released Core/native-engine path for those cases. It does not prove UI packaging, TUN/device-wide coverage, every protocol, physical phone/ARM, mobile carrier/Doze/battery, WAN loss, long sessions or public IPv6. Existing Android24/Windows22 feed semantics remain unchanged; successful Beta SS2022 does not negotiate every older client.

## Rebuilt runtime candidates and advisories

[Candidate provenance](security/runtime-candidates-2026-11-13.json) binds14 amd64/arm64 assets to nine exact source archives/patches, Go1.27.4/Rust1.100.0, compiler/package records and348 license/notice records. These are new candidate builds, not byte-identical reconstruction of anonymous legacy assets. The production engine pins/catalog remain unchanged.

Xray676eaf66, AWG b5928efb, official DNSTT0c5c52a5, VayDNS a0ff7011, Caddy72dd0fb0/forwardproxy e747854e and Slipstream7de506b2/picoquic d4a865c5/picotls5a4461d8 are recorded with complete patches. The Tor-managed DNSTT fork was rejected for incompatible CLI; official-module DNSTT adds only positional CLI compatibility, retaining official -udp form. Both Caddy BBR/read-idle patches are preserved.

Dependency remediation includes crypto0.59/net0.62/sys0.50, compress1.20.2 (GO-2026-6865), brotli1.2.7 (GO-2026-6888) and Tokio1.53.3 (RUSTSEC-2026-0386). The official Go DB snapshot modified2026-11-11T02:51:11Z contains437 referenced records for799 selected module paths, fetched/hashed using the working Windows network path. Five candidate full-source Go scans and the panel source scan report zero finding records. Main/local replacement module versions are not automatically attributable: Caddy's18 and Xray's1 root advisory version ranges were separately checked against the explicit pinned bases; no affected range matched. That is not an audit of local patches/forks.

RustSec database af7b3ec33460b8e2de8d422e16d628518891a519 reports zero Rust vulnerabilities/warnings after Tokio remediation. Go/Rust scanners do not cover native C/OpenSSL/picoquic/picotls/C AWG, license compatibility or unknown advisories.

Latest candidate Xray repeated distinct-account bidirectional payload, per-user/idempotent shared counters, new-session revoke, unaffected-account health and private TLS/core restore. Existing streams still survived in the three added protocols: no strict quota claim; timer SLO and overshoot remain null. New candidate AWG passed actual two-peer payload/selective removal/scoped NAT cleanup/restart (15.8s observed reconnect).

Candidate DNSTT panel-positional and official CLI, VayDNS native and DNSTT-compatible mode, Slipstream with private certificate, and Caddy TLS HTTP/1.1 CONNECT each passed three32768-byte loopback echo/hash attempts (18 attempts total); Caddy wrong password rejected. This is not public recursive DNS or Chromium Naive/H2/H3 acceptance.

Slipstream ABI requires GLIBC2.34 plus OpenSSL3 on amd64, and GLIBC2.38 on arm64; it cannot yet cover the existing Ubuntu20.04+/Debian10+ support range. ARM Slipstream --help executed under user emulation only. Older-distribution rebuild/execution, native advisory review and complete client transport acceptance block promotion.

## Remaining gates and rollback

The four gates remain open with the completed subcases recorded above. Do not mark publication ready or replace production runtime pins based on zero dependency findings. No private key or real customer feed belongs in the review archive.

For code-review rollback use a fresh checkout of baseline2601c1a9cce2fb718e18a051663b81c115f20daa; do not rotate/reinitialize the production identity. For any future deployment, retain current configuration and usage before restoring compatible signed binary/state, preserve accrued usage and only restart owned affected services. See [recovery](security/recovery.md). No live rollback was executed.

## Subsequent owner-authorized WAN diagnosis

See [WG/Hy2 network evidence and explicit Salamander lifecycle](network-diagnosis-2026-11-13.md). Unlike the controlled VM success, both ordinary protocols failed on two owner VPS paths. The installed Windows preview reproduced WG failure; existing AWG passed after a failed cold request; transient Salamander Hy2 passed2/2 while plain Hy2 failed2/2. Exact location of filtering/loss is unknown. The new panel toggle passed actual VM identity, URL/count, validation-failure and startup-rollback cases. No automatic production toggle or new protocol family was introduced. Phone/Mac/hardware acceptance is still open.
