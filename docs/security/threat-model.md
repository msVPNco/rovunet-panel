# Threat model and boundaries

Review scope is the panel repository at the recorded baseline and this private branch. The panel is a privileged server administrator. It is not a hostile-root isolation boundary or a guarantee against traffic analysis and censorship.

| Boundary / asset | Threat | Implemented control | Remaining limitation |
|---|---|---|---|
| Download to root execution | Replaced/missing/wrong-architecture binary, mutable latest, partial write | Independent manifest authority; exact tag/digest/size/ELF; stage then rename; no unsigned fallback | Production authority unprovisioned; legacy upstream source/advisory gaps |
| Imported config/backup to filesystem | Unsafe token/path, oversized/decompression input, symlink target | Token syntax validated before path construction; archive validation and bounds; atomic private regular-file writes | Root/operator import is privileged; a general hostile archive/CLI audit is not claimed |
| Config/timer concurrency | Lost account changes or resurrected URL | Locked compare-before-save, complete reload and subscription serialization | Cross-service mutations are not one atomic transaction |
| Remote DNS to router resources | UDP goroutine/pending/queue growth, slow TCP, resolver sharing | Fixed worker queue, global/per-backend/socket/pending/TCP-response budgets, aggregate UDP rate limits, timeouts | OS/socket memory and all transport engines require separate load tests; UDP spoofing/reflection not eliminated |
| Public SOCKS / SSH | Anonymous access, old credentials, admin account interference | Public SOCKS strict auth, session tracking/reload; owned SSH group and nonzero UID checks; local forwarding only | Full sshd/PAM/live-forwarding validation remains open; SOCKS itself is plaintext |
| Subscription URL | Capability leakage, traversal or stale fetch after rotation | Random 128-bit token, strict token validation, explicit rotation, private generation/no-store route, old aliases retired | Existing imported profiles keep server access until their credentials are revoked |
| Quota and expiry | Uncounted bypass, false zero, surviving stream | Shared eligibility policy, unavailable/partial accounting, history and conservative exclusions | Some engines remove only new-session credentials; hard quota for the new four is not available |
| Backup at rest | Exposure or undetected modification | age encryption/authentication by default; owner-only key files and atomic output | Internal recovery/live configuration remain plaintext; lost key means unrecoverable encrypted backup |
| Privileged services | Excessive capabilities or filesystem access | Per-service requirements inventory; only proven DNS NoNewPrivileges restriction added | Most root services are not fully confined; AWG/WARP need network/TUN privileges |

The DNS HMAC discovery response uses public configuration material. It is protocol discovery/compatibility, not user authentication or proof of device identity. No new cryptography has been invented. Resolver/source IP, TCP connection count and process count must not be called simultaneous devices.

Use valid DNS/TLS/SSH identities and protocol-specific options. SNI/ALPN routing and decoy behavior do not prove stronger encryption or success on every network. Direct Soar has shared public-key access and cannot revoke a single account; account-limited policy remains restrictive.

A practical follow-up review should attack each parser/service boundary with controlled fixtures, independently reproduce signed release builds and exercise recovery/revocation on a disposable full VM and supported devices. No disruptive test against a public resolver/service is authorized by this document.
