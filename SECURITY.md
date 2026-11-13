# Security policy

This is an actively developed server manager with privileged installation and network services. The private engineering branch is not approved for production publication. See [release gates](docs/release-gates.json) and the [engineering ledger](docs/ENGINEERING-LEDGER-2026-11-12.md).

## Reporting

Use GitHub's private vulnerability reporting if it is available for this repository. Otherwise, contact a maintainer to agree a private channel before sending a reproduction. Do not put real subscription URLs, credentials, private keys, backup contents or private server addresses in an issue. Public support at [RovunetVPN](https://t.me/RovunetVPN) is not a place to post those values.

Include the affected source commit and binary hash, OS/architecture, engine version/hash, a minimal synthetic reproduction, impact and the trust boundary crossed. Distinguish a source concern, a reproduced defect and a dependency advisory. Maintainers work voluntarily; no response-time guarantee or paid incident service is offered.

## Updates and scope

Only an explicitly reviewed release is a production update. The new verifier requires an independently provisioned Ed25519 authority and rejects missing signatures and test identities. No production authority has been created in this review; the empty trust root intentionally prevents installation/update from silently accepting unsigned bytes.

Older v1.21.1 artifacts remain historical public artifacts. Their co-published SHA256SUMS detect transfer corruption but do not establish an independent publisher identity. This review does not retroactively authenticate those releases.

No general penetration-test, security-audit or all-protocol-health certification is claimed. Real-device WireGuard/Hysteria 2, legacy runtime provenance/advisories, full VM recovery and several open-session limits remain acceptance gates.

See [threat model](docs/security/threat-model.md), [privacy](docs/security/privacy.md), [supply chain](docs/security/supply-chain.md), [recovery](docs/security/recovery.md) and [runtime inventory](docs/security/runtime-inventory.md).
