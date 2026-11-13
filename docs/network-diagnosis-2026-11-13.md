# Live WG / Hy2 diagnosis — 2026-11-13

This is a private review addendum. The owner opened an authenticated Bitvise terminal/SFTP session and authorized investigation. Two distinct existing VPS endpoints were kept separate. Private links, keys, account files, raw packet captures, addresses and raw client logs are excluded from review evidence. Selected profiles remain in owner/SYSTEM-only local Windows storage.

## Measured results

The owner-saved first VPS feed actually served24 native Android items and22 conservative Windows items. REALITY, XHTTP and Trojan ordinary verified HTTPS requests succeeded. Stock WireGuard and unmasked Hy2 failed twice each. A separately source-built newer Windows Xray26.11.9 did not repair those two failures. This is not a subscription-count or profile-name fault.

On the second, owner-opened VPS, existing Xray26.10.31/63ca2e0 processes use the expected config files. Hy2 auth, listener port, unexpired certificate and SNI match the selected profile. Both WireGuard public-key relationships match. Host UDP listeners and accept rules are present; UFW is inactive and host INPUT defaults to ACCEPT. Provider firewall state is not independently established.

A50-second metadata-only capture observed both64-byte and1200-byte diagnostic UDP markers at both protocol ports. One148-byte WireGuard initiation reached the VPS and one92-byte response left it; no encrypted data packet was seen. Actual Hy2 QUIC client packets were not seen, although the diagnostic markers arrived. The Windows engine logged three WireGuard initiation sends and zero received handshake responses. The currently installed0.1.2-preview.5 Core and native engine reproduced the WireGuard failure; this was not inferred solely from old Beta code.

Existing AmneziaWG, using that installed preview's unmodified Core, helper and Xray, passed HTTPS204 on the second attempt in each of two independent cold starts. Both first requests failed after about6 seconds. Therefore it has measured working traffic but an unresolved cold-start/retry limitation; this does not establish reliable immediate connection or phone/Mac acceptance.

An independent transient authenticated diagnostic core compared plain Hy2 on31846 and Salamander on31847. It used existing verified TLS certificate references and temporary fixture auth, no current subscriber credentials. The service had RuntimeMaxSec=90 and control-group cleanup. Plain mode failed2/2 with network inactivity; Salamander passed2/2 verified HTTPS204 in814ms and405ms. Terminal observations confirmed both diagnostic ports released and the original service PID/config hash preserved. Existing permanent services, firewall, identities, ports and feeds were not changed.

These observations support an inference of protocol-shaped traffic filtering/loss somewhere in the path. They do not identify a particular ISP, cloud provider, DPI implementation or host networking component. Different test ports remain a limitation of the plain/Salamander comparison.

## Prepared panel change

The private panel build now supports --hy2-salamander for core-plan/core-install and an explicit core-hy2-obfs NAME on|off lifecycle command for existing account-managed Hy2. A core-specific mask password is derived with a separate HMAC domain from the existing private seed. It is not a replacement for each user's authentication credential. Ordinary updates do not toggle this option.

Before switching an existing core the command refuses pending/edited state, validates the desired config using the pinned engine and writes/rechecks a private recovery copy. Failed validation or startup preserves/restores the prior files and active state. Disabled/inactive cores are not started. It changes no usage ledger, account identity, subscription token or other core. Clients must refresh the same saved subscription URL after a successful switch; their cached old plain profile cannot connect to a masked listener.

On the owned full VM,24 Android /22 Windows item counts and URL tokens remained; exactly one Hy2 profile per feed changed, with authentication/endpoint preserved. Repeat enable was a no-op. A real missing-certificate rejection happened before mutation, and an actual forced startup failure rolled back to an active masked core. Switching off restored the exact previous core files and links. Unit/race coverage also tests credential stability, wrong-protocol/path/pending-state rejection and rollback.

This provides a measured repair candidate for Hy2 without replacing a protocol family. Standard WG remains a recorded failing WAN path. AWG remains a separate profile; no silent WG-to-AWG substitution or inflated profile count. Android/Mac source readers contain Salamander support, but physical phone/Mac network acceptance remains open. The old public Windows22 compatibility feed stays conservative; a private newer preview's capabilities do not automatically negotiate support for older clients.

## Use and recovery after deployment review

    rovunet core-hy2-obfs YOUR_EXISTING_HY2_NAME on
    rovunet core-status YOUR_EXISTING_HY2_NAME

Update the same existing subscription inside each app. The command prints a private recovery directory; its contents contain credentials and must remain private. To intentionally return to plain mode:

    rovunet core-hy2-obfs YOUR_EXISTING_HY2_NAME off

This command is in the private review build, not public v1.21.1. No private build was deployed to a customer VPS. Independent offline custody, runtime/native ABI and full recovery/client gates still block publication. Cross-service concurrency and crash-during-toggle reconciliation are not proved by these single-operation rollback tests.
