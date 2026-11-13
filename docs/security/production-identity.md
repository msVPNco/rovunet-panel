# Panel production release authority

Provisioned on 2026-11-13 under the panel owner's explicit instruction to create
the missing signing/recovery identities and retain them in Windows. This is a
new panel identity; it is separate from Android, Windows and macOS signers.

Public Ed25519 SPKI SHA-256:

`885d8aba22365ce42e20527bfa00e82aed3cba2f69501f146ed0496a8bc3dcbe`

Public SPKI (base64 DER):

`MCowBQYDK2VwAyEAMfWBf2jAnb20LJvMoOfN1R/4OP0tSjPec+GWU+QXdqs=`

The same production-only public root is compiled into the Go verifier and the
bootstrap installer. Downloaded roots, environment overrides and test-purpose
authorities cannot replace it. Compare this fingerprint with the owner's
separately retained Windows trust-root record before initial installation.
The repository and a checksum fetched alongside a binary do not constitute an
independent initial trust channel.

## Custody and recovery

The operator tool in `tools/signing-vault` uses current-user Windows DPAPI with
a panel-specific purpose to protect the signing key. Release signing passes
only public manifest paths to the tool; the private key is not exported to
Python, WSL, a CI secret, a command argument, a repository or a release archive.
Use `scripts/panel-signing-vault.ps1` to enforce owner/SYSTEM-only directory and
file ACLs and reject links before accessing the vault or recovery kit.

A distinct random age X25519 recovery identity decrypts an authenticated backup
of the production PKCS8 key. The original DPAPI blob is not needed to recover
into a new Windows vault. Recovery verifies the independently retained public
fingerprint before resealing. An actual recovery into a fresh private Windows
directory, followed by a signature challenge, passed on the provisioned key.
Automated Windows tests also reject wrong recovery identities, truncated age
backups, damaged DPAPI blobs, wrong DPAPI purposes, manifest tag/repository/key
mismatches and overwrite attempts.

The owner-specific storage locations are supplied in the private handoff, not
embedded into server code. The portable age identity and encrypted key backup
are protected by Windows ACLs on a separate local drive. **A local second drive
is not an offline or off-site backup.** No removable/off-site medium has been
observed or certified. Loss/compromise of the Windows account or both drives
remains outside the measured recovery result. DPAPI is not a hardware signer.

The wrapper refuses overwrites; `init` never rotates an existing identity.
Before future rotation, provision and separately record a successor root,
review both roots in the installer and panel, authenticate the transition with
the currently trusted key, retain old signed-release verification and recover
the successor in isolation. If the current key is compromised, authenticated
rotation alone is insufficient: redistribute a verified new root through the
owner's independent channel. Do not silently accept a new root from GitHub.

Public release gates remain independent of key provisioning. Local signing
does not authorize publication or claim runtime/client acceptance.

PortableRecovery is an explicit read-only input mode for removable/non-NTFS or foreign-owner recovery media. The destination vault still requires current-owner/SYSTEM private ACLs. The helper requires ExpectedKeyId for every check/recover/sign operation and checks it before decryption. The default is the separately retained production fingerprint. The wrapper never rewrites ACLs/ownership of an existing folder. Three fresh local recovery destinations passed, including portable input mode; no actual removable medium was observed. PowerShell7 is used without changing execution policy.
