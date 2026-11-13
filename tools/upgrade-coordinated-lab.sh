#!/bin/bash
set -euo pipefail
# Review-only deployment harness. Production signing/canary approval is separate.
[[ $EUID -eq 0 && -f /etc/rovunet-lab-authorized ]] || { echo 'Owned laboratory required' >&2; exit 1; }
[[ $# -eq 2 && "$2" =~ ^[0-9a-f]{64}$ ]] || { echo 'Usage: reviewed-binary reviewed-sha256' >&2; exit 1; }
candidate=$(realpath -- "$1")
[[ $(sha256sum "$candidate" | cut -d' ' -f1) == "$2" ]] || exit 1
legacy=$(python3 - <<'PY_PREFLIGHT'
import json,pathlib
p=pathlib.Path('/etc/rovunet/config.json');c=json.loads(p.read_bytes())
b=c.get('subscription_v2_authority')
print('yes' if ((not b and (c.get('subscription_v2') or pathlib.Path(str(p)+'.status-v2.anchor').exists())) or (b and b.get('state')=='legacy')) else 'no')
PY_PREFLIGHT
)
if [[ "$legacy" == yes ]]; then
 [[ -n "${ROVUNET_REVIEWED_ADOPTION_MANIFEST:-}" && -n "${ROVUNET_REVIEWED_ADOPTION_PUBLIC_KEY:-}" ]] || { echo 'Reviewed legacy-authority adoption manifest required before any mutation' >&2; exit 1; }
fi
exec 9>/usr/local/bin/.rovunet-coordinated-upgrade.lock
flock -x 9
mapfile -t units < <(python3 - <<'PY'
import json
c=json.load(open('/etc/rovunet/config.json'))
print('rovunet-user-expiry.timer')
print('rovunet-user-expiry.service')
print('rovunet-subscription-v2.service')
for t in c['tunnels']:
 if t.get('transport')=='vless':print('rovunet-'+t['tag']+'.service')
PY
)
active=()
for unit in "${units[@]}"; do
 [[ "$unit" =~ ^rovunet-[a-zA-Z0-9_-]+\.(service|timer)$ ]] || exit 1
 if [[ $(systemctl show "$unit" --property=LoadState --value) == not-found ]]; then continue; fi
 if [[ "$unit" != rovunet-user-expiry.service ]] && systemctl is-active --quiet "$unit"; then active+=("$unit"); fi
 systemctl stop "$unit" || exit 1
 if [[ "$unit" == *.service ]]; then
  [[ $(systemctl show "$unit" --property=MainPID --value) == 0 ]] || exit 1
 else
  [[ $(systemctl show "$unit" --property=ActiveState --value) == inactive ]] || exit 1
 fi
done
python3 - "$candidate" <<'PY'
import pathlib,sys,os
# Reject an unowned/interactive old writer instead of killing arbitrary tasks.
dest=pathlib.Path('/usr/local/bin/rovunet')
if dest.exists():
 old=dest.stat()
 for process in pathlib.Path('/proc').iterdir():
  if not process.name.isdigit():continue
  try:
   info=(process/'exe').stat()
  except FileNotFoundError:continue
  if (info.st_dev,info.st_ino)==(old.st_dev,old.st_ino):
   raise SystemExit('Another installed panel writer is still running; upgrade refused')

src=pathlib.Path(sys.argv[1]);dest=pathlib.Path('/usr/local/bin/rovunet')
if dest.exists():
 backup=dest.with_name('rovunet.pre-coordinated-upgrade')
 if not backup.exists():backup.write_bytes(dest.read_bytes());backup.chmod(0o700)
tmp=dest.with_name('.rovunet-reviewed-new')
with tmp.open('wb') as f:
 f.write(src.read_bytes());os.fchmod(f.fileno(),0o755);f.flush();os.fsync(f.fileno())
os.replace(tmp,dest)
fd=os.open(dest.parent,os.O_RDONLY);os.fsync(fd);os.close(fd)
PY
if [[ -n "${ROVUNET_REVIEWED_ADOPTION_MANIFEST:-}" || -n "${ROVUNET_REVIEWED_ADOPTION_PUBLIC_KEY:-}" ]]; then
 [[ -n "${ROVUNET_REVIEWED_ADOPTION_MANIFEST:-}" && -n "${ROVUNET_REVIEWED_ADOPTION_PUBLIC_KEY:-}" ]] || exit 1
 /usr/local/bin/rovunet subscription-v2 adopt-legacy "$ROVUNET_REVIEWED_ADOPTION_MANIFEST" "$ROVUNET_REVIEWED_ADOPTION_PUBLIC_KEY"
fi
/usr/local/bin/rovunet post-update --strict
for unit in "${active[@]}"; do systemctl start "$unit"; done
# An error intentionally leaves services stopped for forward repair; an old
# writer/config/DB is never silently restored after new consumption.

