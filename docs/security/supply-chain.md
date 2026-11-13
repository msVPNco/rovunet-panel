# Executable supply chain and production gate

Review dates: 2026-11-12/13. Panel baseline: 2601c1a9cce2fb718e18a051663b81c115f20daa. On November 13 the owner authorized creating and retaining the missing panel production signing/recovery identity in Windows. No paid service, GitHub secret, tag or release was created.

## Trust path

The installer itself must be obtained at an independently reviewed immutable commit and checked against a digest distributed through that independent channel. Piping mutable main into a root shell is not the trust procedure for this branch. A checksum downloaded beside an executable is not an independent trust anchor.

Both install.sh and internal/releaseauth/production-keys.json hold the same reviewed production Ed25519 public-key set. The key ID is SHA256 of the DER SubjectPublicKeyInfo, not a human label. A release manifest covers exact asset names, sizes, SHA256, OS/architecture, source commit, source URL and license. Its 64-byte detached signature authenticates the original JSON bytes. Repository/tag/authority, unknown fields, duplicate members, depth/size and duplicate asset constraints are checked. Test-purpose keys and test manifests are refused by production consumers.

The production root now contains the separately provisioned panel identity described in [production identity and recovery](production-identity.md). Its fingerprint is also retained in the owner's independent local Windows record. Generated test authorities remain refused. A portable recovery into a fresh Windows vault passed; copying the recovery kit to an actual offline/off-site medium remains outstanding. No environment-variable trust override exists.

The signer checks that its identity matches both compiled roots and that the exact installer is included. A separate verifier authenticates every file in the release directory and rejects unsigned extras. The Windows signing path keeps private bytes inside current-user DPAPI and passes only public manifest paths to the ACL-checked operator tool. The optional CI file-based signer still requires separately configured owner custody; no CI private secret has been provisioned. Full-qualification publication still requires all four acceptance gates verified. The owner separately authorized the bounded v1.21.2 functional update after the private report; its scope is in ../releases/v1.21.2-scope.json and does not mark those gates passed. Actions only stages a tested draft; exact final bytes and acceptance are verified and signed locally before the owner-authorized publication. No private key is sent to CI.

## Consumers

| Consumer | New bytes accepted from | Validation and continuity |
|---|---|---|
| Bootstrap panel | Exact tag resolved once; signed manifest | Bounded download, signature, exact size/hash, host ELF/load segment; old binary retained until same-filesystem rename |
| Panel updater | Canonical exact asset URL; signed manifest | Reject draft/prerelease on stable and numeric downgrade; durable binary/config/usage/unit recovery record; strict post-update required |
| Managed Xray/AWG/tool | Exact installed panel version tag | Same production verifier; no mutable runtime latest |
| DNSTT/VayDNS/Slipstream/Caddy | Embedded reviewed transport catalog, v1.6.8 | Exact byte size/hash and host ELF; offline aliases get the same checks; pin is part of the reviewed panel source |
| Legacy setup.sh installer | Installed reviewed panel's legacy-naive-runtime helper | No xcaddy@latest, moving plugin branch, unsigned Go tarball or global Go deletion |
| OS packages, WARP prerequisites | Administrator's OS package trust store | apt metadata/signatures are the distribution's responsibility; not a panel-created publisher identity |
| CI actions/tools/compiler | Commit-pinned actions, Go modules, pinned ShellCheck/Arm archive digest | Build provenance retained; action/tag updates need review |

Existing transport executables may be retained during ordinary operation for continuity. This is not retroactive attestation of their bytes or source. A reviewed runtime update stages before replacement and keeps the previous complete binary; it must not erase that backup on an idempotent retry.

The transport catalog is a pin, not proof of an upstream build's source. Unresolved local replacements, source-license mapping and binary advisory results are a production gate; see [inventory](runtime-inventory.md).

## Acceptance and remaining cases

Ephemeral authority tests prove that production rejects the test authority, tampered signatures/assets, missing metadata, wrong CPU, truncated downloads, 404/504, interruption and disk exhaustion without executing a candidate. The bootstrap lab executes a synthetic candidate only at its final installed path, and exercises a pending-install retry without replacing the prior recovery copy.

Ephemeral-authority tests run in private mount/network/PID namespaces. Separate owner-authorized Windows custody tests exercise the real provisioned identity. Neither set independently approves a release. The test API cannot change shipping roots. There is no unsigned fallback.

Reproducible builds across independent hosts, upstream exact source reconstruction, signing-key incident response and protected production environment configuration remain open. Signature validity does not prove that signed code is free of defects.

For v1.21.2, the landing guides fetch the exact versioned installer and check its reviewed SHA256 before execution. Initial root distribution remains in the broader qualification inventory; operators can compare the published public key fingerprint with an independently retained record.
