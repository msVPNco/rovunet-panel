# Installation, update and recovery

The new update transaction retains an old binary, a private state journal and a validated recovery snapshot before installing a verified candidate. Its states distinguish prepared, installed, post-update-pending and complete. A failed post-update is reported as pending and can resume through the reviewed updater; the old binary is not deleted at the first successful rename.

Bootstrap uses a private temporary directory in /usr/local/bin, verifies before chmod/rename and records pending configuration. It does not kill running panel processes or remove the old binary before downloading. A verified executable is launched only by its permanent path. The bootstrap receipt is cleared only after install/complete-setup succeeds.

Config, subscription and unit files use private same-directory staging, fsync and atomic rename, retaining an intentional service owner. Symlink/non-regular destinations are rejected. Configuration writes are cross-process locked and reject stale loaded state. This prevents lost config/token changes; it is not an all-services database transaction. A newer configuration is reloaded before publishing subscriptions, so an old timer cannot recreate a retired link.

Restore is excluded from the automatic CLI pre-run migration. It authenticates
the backup, reads the existing legacy configuration without mutation, obtains
restore confirmation, then migrates and reloads current state before snapshot
and restore. backup-check, backup-keygen and version queries do not migrate.
The actual CLI namespace test covers invalid legacy-layout restore/validation,
read-only commands and successful authenticated/confirmed legacy restoration.

Service regeneration preserves an administrator's stopped/disabled choice. An active service is restarted after successful unit replacement, without an up-front stop/delete. Caddy candidate configuration must validate before replacing the current file; validation failure does not silently drop the subscription route.

## Recovery material

Internal update/setup snapshots remain **plaintext** in private root-only directories. They contain recoverable secrets and must be protected like the live configuration. They preserve configuration, core credentials/state, usage history, owned units and activation information; SSH configuration and firewall state are captured read-only when available. A missing optional capture is recorded, not fabricated.

An operator-exported backup is now encrypted by default using maintained age authentication. Hidden passphrase entry or a separately stored age identity is required for recovery. Existing plaintext bundle versions still decode; plaintext export requires an explicit choice. The encrypted backup is not a substitute for retaining its recovery key.

## Rollback procedure

Do not copy an old binary over newer config/usage blindly or reset counters to a stale snapshot. While the new binary remains valid, first retry the pending update to finish forward migration. Keep both binary hashes, the state receipt and recovery directory for diagnosis. Inspect the journal and backup with the reviewed matching binary before changing anything.

For a deliberate rollback, stop only the affected owned services in a maintenance window, retain the current config and usage too, verify the old binary and its matching recoverable schema, and restore compatible owned files/units. Use the matching validated backup/usage-reconciliation tools, daemon-reload, then restart only services that were previously active. Reconcile usage accrued after the snapshot; a binary-only rollback does not restore that usage automatically. Keep encrypted backup keys separately.

For a local code-review rollback with no deployment, switch to the recorded baseline in a separate clean checkout. This review does not require uninstalling the running public panel.

## Measured scope and open acceptance

Unit/fault tests cover wrong/missing signatures, HTTP faults, interruption, ENOSPC, cross-device rename failure, stale writers, owned-text migration and pending recovery. Namespace integration exercises actual panel/engines with simulated systemd, and a separate actual systemd DNS restart uses an owned transient unit. No real server was changed.

Actual nine-core full-VM normal reboot and abrupt power-cut, encrypted core/OS restore, real restricted SSH/selective revocation and private-CA certificate-change checker passed on November13. Stale persistent API sockets and recursive private-key chown were reproduced and fixed. Full base DNS/WARP/public ACME restore, timer/held-stream bounds and concurrent cross-service account-operation reconciliation remain open. See [the scoped measurements](../engineering-acceptance-2026-11-13.md). If OS credential synchronization fails after a committed password change, the command reports pending and requires retry; no complete multi-service transaction is claimed.
