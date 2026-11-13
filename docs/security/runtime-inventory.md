# Runtime inventory and advisory evidence

Observation: 2026-11-12. Exact hashes/platform/size/source pins are in [runtime-inventory.json](runtime-inventory.json) and the compiled [transport catalog](../../internal/binary/transport-catalog.json). These are observations of specific bytes, not a general assurance for a protocol.

| Component | Reviewed pin / license evidence | Provenance or maintenance gate |
|---|---|---|
| Panel | This private branch, Go1.27.4; AGPL-3.0 | Final source commit/build hashes in the review bundle; owner-authorized Windows production root provisioned; offline/distribution acceptance remains open |
| Xray, legacy and dedicated | 63ca2e0c1d31b89f67700c33842431f541fedd25; MPL-2.0 | Pinned API/config/loopback behavior only; upstream deprecations and dependency findings need triage |
| AmneziaWG userspace | b5928efb6ca19f0153958460c3d141f04abc5c2e; MIT | Exact dialect/client/options matter; actual namespace data test is not device or hard-quota acceptance |
| AmneziaWG tools | ee0f0a9aa34ff0a0da4b3433b9512781cfe02843; GPL-2.0 | C/cross-compiler provenance retained; Go vulnerability scanner does not assess C code |
| Caddy/Naive | SlipGate v1.6.8 release commit200d8151d8933ac50c80f7dc9ff8516b799c91b3; both Linux CPU hashes pinned | Build metadata says Caddy v2.11.7, but local ../caddy-bbr and ../forwardproxy replacements are devel; exact patched source/build and combined license material unresolved |
| DNSTT | Same upstream release; both Linux CPU hashes pinned | Binary uses Go1.25.6, main module devel, no vcs.commit. Exact source/license/rebuild mapping unresolved |
| VayDNS | Same upstream release; both Linux CPU hashes pinned | Go1.25.6, devel main; KCP replaced by net2share revision416ba9d3856d. Exact source/license/rebuild mapping unresolved |
| Slipstream | Same upstream release; both Linux CPU hashes pinned, ELF checked | Non-Go executable; no reconstructed exact source/build/advisory mapping yet |
| Soar | go.mod v0.2.4, bundled in panel | Shared-key direct mode is not per-account authentication; dependency scanner and behavior gates remain separate |
| OS SSH/WireGuard/tcpdump | Distribution packages on deployment host | Distribution version/advisories and administrator policy need host-specific review |

Upstream release notes identify Caddy v2.11.7 and forwardproxy e747854 with BBR/idle patches, while the other transport binaries are said to be unchanged since v1.6.4. Those statements and artifact hashes do not reconstruct exact build sources or authenticate them. No claim is made that a license for the upstream panel covers unknown bundled binary modifications.

## Advisory scan

govulncheck v1.9.0 was built from its official module (source e672cbe8fbc8c76e28fcb6db0efd9ce354e0dac5). WSL requests to the official service returned403. The Windows path fetched the official index and all332 advisory records referenced by the208 selected panel/runtime module paths, validated their IDs and recorded their SHA256. Scans used that local snapshot, last-modified2026-11-11T02:51:11Z. This is not a claim that a blocked network scan passed.

| Target | Scan evidence | Unique advisory IDs / interpretation |
|---|---|---|
| Panel source after dependency update | Full source call graph, linux/amd64, Go1.27.4 | 0 findings; not a complete security audit |
| DNSTT / VayDNS amd64 | Conservative binary symbol presence | 69 each; runtime reachability/impact not established |
| Caddy/Naive amd64 | Conservative binary symbol presence | 19; devel/local replacements further limit version attribution |
| Pinned Xray amd64 | Binary package presence | 17; no full source-call-graph triage was performed for this engine |
| Pinned AWG Go amd64 | Binary package presence | 37; no full source-call-graph triage was performed for this engine |
| Slipstream / C AWG tool | Not assessed by Go scanner | Unknown; not zero |

JSON scan mode can exit0 while emitting findings. The table is derived from finding records, not exit status. Package/symbol presence is not proof of an exposed vulnerable path or exploit; it is also not a clean result. ARM64 static byte/architecture validation was performed for the eight legacy assets; ARM64 execution and C-family advisory analysis were not.

The panel's x/crypto was moved to v0.59.0 (source6c966a7517bfc4fa0f268afb1fe663013ed932a5), with x/sys v0.50.0 and x/term v0.48.0. Three earlier module findings were in packages not reached by the panel: unmaintained OpenPGP and two SSH deadlocks; the updated source scan reports none. See official [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932), [GO-2026-6354](https://pkg.go.dev/vuln/GO-2026-6354), [GO-2026-6355](https://pkg.go.dev/vuln/GO-2026-6355) and [Go vulnerability tooling](https://go.dev/doc/security/vuln/).

Production publication remains blocked until exact source/license reconstruction, runtime-specific advisory triage/remediation, repeat data/revoke tests and the independent signing/recovery gates are reviewed. Do not silently update an upstream pin merely because a newer tag exists.

## November13 candidate reconstruction

The table above remains an observation of the old installed pins. A separate [candidate source/ABI manifest](runtime-candidates-2026-11-13.json) and [acceptance report](../engineering-acceptance-2026-11-13.md) now cover14 source-built assets, source patches and348 notice records, attributable Go source scans plus explicit main/local-root version review and RustSec. They do not retroactively authenticate anonymous old bytes. The default catalog did not change.

Official DNSTT adds the existing panel positional CLI as a compatibility-only patch. Both Caddy BBR/read-idle patches remain. compress1.20.2, brotli1.2.7 and Tokio1.53.3 remove observed candidate dependency findings. All relevant local replacement/main version-attribution and native C coverage limitations are explicit in the new report.

Actual new candidate Xray/AWG data/revoke and bounded transport payload tests passed, including controlled native Windows WG/Hy2. Established Xray streams still survive per-user removal. Slipstream's minimum GLIBC2.34/2.38 does not satisfy all currently documented Linux versions. These are review candidates with an open promotion gate, not new installed/released pins.
