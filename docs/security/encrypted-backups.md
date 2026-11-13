# Encrypted backups

Operator backup export uses authenticated age encryption by default. With a terminal, choose a long unique passphrase using the hidden prompt, then test the backup:

~~~bash
sudo rovunet backup /root/rovunet-backup.age
sudo rovunet backup-check /root/rovunet-backup.age
sudo rovunet restore /root/rovunet-backup.age
~~~

For unattended export, create a private recovery identity once and retain it separately/offline. Only its public recipient is printed:

~~~bash
sudo rovunet backup-keygen --identity-file /root/recovery.agekey
# Use the printed age1... public recipient below:
sudo rovunet backup /root/rovunet-backup.age --recipient PUBLIC_AGE_RECIPIENT
sudo rovunet backup-check /root/rovunet-backup.age --identity-file /root/recovery.agekey
sudo rovunet restore /root/rovunet-backup.age --identity-file /root/recovery.agekey
~~~

PUBLIC_AGE_RECIPIENT is a placeholder for the actual printed public value. Never put private identity/passphrase contents in arguments, logs, chat or the same shared location as a backup. Key files must be regular, owned by the current user, owner-only and at most64KiB; symlinks are refused. A passphrase file can be supplied through --passphrase-file and has the same file protections. Losing the recovery secret prevents decryption.

For an SSH pipeline, supply --recipient PUBLIC_AGE_RECIPIENT to the source backup command and --identity-file /root/recovery.agekey to the receiving restore command; add --yes only after reviewing the intended destination. The private identity stays at the destination. Restore authentication finishes before configuration/service mutation.

Legacy ROVUNET BACKUP bundles still restore. An explicit --plaintext export is available for a reviewed need; the format is compressed/Base64, not encryption. The new encrypted wrapper contains the existing validated bundle. Preserve identities, usage and keys with a matching reviewed version or standard age tools when planning downgrade.

Restore can continue independent steps after an error to retain useful recovered files, but then exits nonzero and reports incomplete recovery. Keep the private pre-restore snapshot and resolve the warnings before calling the server restored. Internal update/setup/pre-restore snapshots remain root-only plaintext; no claim is made that all live secrets are encrypted at rest.
