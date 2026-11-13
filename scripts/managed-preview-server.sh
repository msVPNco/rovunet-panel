#!/usr/bin/env bash
# Private preview deployment. Existing protocol/test services are not restarted.
# Usage: preview-server.sh install HOST TLS_NAME | rollback | status
set -euo pipefail
umask 077
package_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
state=/etc/rovunet/managed-preview.json
target=/usr/local/bin/rovunet
cores=(managed-awg managed-ss2022 managed-trojan managed-vlessenc)
[[ $EUID == 0 ]] || {
    echo 'Run as root.' >&2
    exit 1
}

state_value() {
    python3 - "$state" "$1" << 'PY'
import json,sys
print(json.load(open(sys.argv[1]))[sys.argv[2]])
PY
}

resume_timers() {
    local backup=$1 timer enabled active
    while read -r timer enabled active; do
        if [[ "$enabled" == enabled ]]; then systemctl enable "$timer" > /dev/null 2>&1; fi
        if [[ "$active" == active ]]; then systemctl start "$timer"; fi
    done < "$backup/rollback-timers"
}

pause_timers() {
    local backup=$1 timer service counter
    if [[ ! -f "$backup/rollback-timers" ]]; then
        for timer in rovunet-user-expiry.timer rovunet-core-certs.timer; do
            printf '%s %s %s\n' "$timer" "$(systemctl is-enabled "$timer" 2> /dev/null || true)" "$(systemctl is-active "$timer" 2> /dev/null || true)"
        done > "$backup/rollback-timers"
    fi
    for timer in rovunet-user-expiry.timer rovunet-core-certs.timer; do
        # Disabling also makes an interrupted rollback safe across a host reboot.
        systemctl disable --now "$timer" > /dev/null 2>&1 || {
            resume_timers "$backup"
            return 1
        }
        service=${timer%.timer}.service
        counter=0
        while systemctl is-active --quiet "$service"; do
            if ((counter++ >= 45)); then
                echo 'An accounting/certificate task is still running; rollback was deferred.' >&2
                resume_timers "$backup"
                return 1
            fi
            sleep 1
        done
    done
}

rollback() {
    [[ -f "$state" ]] || {
        echo 'No preview deployment marker found.' >&2
        return 1
    }
    local backup current previous expected failed=0
    backup=$(state_value backup)
    case "$backup" in /root/rovunet-managed-preview.*) ;; *)
        echo 'Invalid backup path.' >&2
        return 1
        ;;
    esac
    [[ -f "$backup/rovunet.previous" ]] || return 1
    expected=$(state_value preview_sha256)
    previous=$(sha256sum "$backup/rovunet.previous" | cut -d' ' -f1)
    current=$(sha256sum "$target" | cut -d' ' -f1)
    if [[ "$current" != "$expected" && "$current" != "$previous" ]]; then
        echo 'Panel binary changed after this preview; automatic rollback will not overwrite it.' >&2
        return 1
    fi
    trap - ERR INT TERM
    pause_timers "$backup" || return 1
    while IFS= read -r name; do
        case "$name" in managed-awg | managed-ss2022 | managed-trojan | managed-vlessenc) ;; *) return 1 ;; esac
        if [[ -d /etc/rovunet/core/"$name" ]]; then
            if ! "$package_dir/rovunet-linux-amd64" core-remove "$name" >> "$backup/rollback.log" 2>&1; then
                failed=1
            fi
        fi
    done < "$backup/created-cores"
    if [[ "$failed" != 0 ]]; then
        echo "A preview core could not be removed; see $backup/rollback.log. The current binary was kept." >&2
        resume_timers "$backup"
        return 1
    fi
    if ! python3 "$package_dir/preview-rollback-usage.py" prepare --usage /var/lib/rovunet/usage --backup "$backup"; then
        resume_timers "$backup"
        return 1
    fi
    if [[ "$current" == "$expected" ]]; then
        if ! install -m 0755 "$backup/rovunet.previous" "$target.rollback-tmp" || ! mv -f -- "$target.rollback-tmp" "$target"; then
            python3 "$package_dir/preview-rollback-usage.py" restore --usage /var/lib/rovunet/usage --backup "$backup"
            resume_timers "$backup"
            return 1
        fi
    fi
    resume_timers "$backup"
    while IFS= read -r path; do
        case "$path" in /usr/local/bin/rovunet-awg | /usr/local/bin/rovunet-amneziawg-go | /usr/local/bin/rovunet-awg-provenance.json) rm -f -- "$path" "$path.provenance.json" ;; *) return 1 ;; esac
    done < "$backup/created-binaries"
    rm -f -- "$state"
    echo "Preview rolled back. Existing services and account records were retained. Private backup: $backup"
}

private_exports() {
    local backup=$1 username
    username=$(
        python3 - << 'PY'
import json,datetime
c=json.load(open('/etc/rovunet/config.json'))
now=datetime.datetime.now(datetime.timezone.utc)
users=[]
for u in c.get('users',[]):
    expiry=u.get('expires_at')
    if expiry and datetime.datetime.fromisoformat(expiry.replace('Z','+00:00'))<=now:continue
    users.append(u)
users.sort(key=lambda u:bool(u.get('quota_gb')))
if not users:raise SystemExit('No active panel account is available for private exports')
print(users[0]['username'])
PY
    )
    "$target" export-clients --user "$username" --client android > "$backup/android-private.txt"
    "$target" export-clients --user "$username" --client windows > "$backup/windows-private.txt"
    python3 - "$username" "$backup" << 'PY'
import json,pathlib,sys
c=json.load(open('/etc/rovunet/config.json'));u=next(u for u in c['users'] if u['username']==sys.argv[1])
out=pathlib.Path(sys.argv[2]);links=[]
for t in c.get('tunnels',[]):
    if t.get('transport')=='naive' and t.get('enabled') and t.get('domain'):
        port=t.get('naive',{}).get('port',443) or 443
        host=t['domain']+(f':{port}' if port!=443 else '')
        token=u.get('sub_token','')
        if token:links=[f'https://{host}/sub/{token}.json',f'https://{host}/sub/{token}.windows.json']
        break
(out/'subscription-links-private.txt').write_text('\n'.join(links)+'\n')
(out/'android-subscription-private.txt').write_text((links[0]+'\n') if links else '')
(out/'windows-subscription-private.txt').write_text((links[1]+'\n') if len(links)>1 else '')
for path in out.glob('*private.txt'):path.chmod(0o600)
PY
    echo "Private profiles and separate Android/Windows subscription files: $backup (do not post their contents publicly)."
}

case ${1:-} in
    rollback)
        rollback
        exit
        ;;
    status)
        "$target" --version
        for name in "${cores[@]}"; do "$target" core-status "$name"; done
        exit
        ;;
    install) ;;
    *)
        echo 'Usage: preview-server.sh install HOST TLS_NAME | rollback | status' >&2
        exit 1
        ;;
esac
[[ $# == 3 ]] || {
    echo 'HOST and TLS_NAME are required.' >&2
    exit 1
}
host=$2
tls_name=$3
[[ $(uname -m) == x86_64 ]] || {
    echo 'This private package is Linux x86_64 only.' >&2
    exit 1
}
for command in python3 systemctl ip ss iptables sha256sum openssl; do command -v "$command" > /dev/null; done
cd -- "$package_dir"
sha256sum --check SHA256SUMS
expected=$(sha256sum rovunet-linux-amd64 | cut -d' ' -f1)
if [[ -f "$state" ]]; then
    [[ $(state_value preview_sha256) == "$expected" ]] || {
        echo 'A different preview is already recorded.' >&2
        exit 1
    }
    [[ $(sha256sum "$target" | cut -d' ' -f1) == "$expected" ]] || {
        echo 'Panel changed after preview deployment.' >&2
        exit 1
    }
    private_exports "$(state_value backup)"
    echo 'Same preview already installed; keys and account periods were kept.'
    exit
fi
version=$("$target" --version 2>&1)
[[ "$version" == 'rovunet v1.20.1 (212eda5)' ]] || {
    echo "Expected the audited v1.20.1 server; found: $version" >&2
    exit 1
}
for name in "${cores[@]}"; do
    [[ ! -e /etc/rovunet/core/"$name" && ! -e /etc/systemd/system/rovunet-core-"$name".service ]] || {
        echo "Name already in use: $name" >&2
        exit 1
    }
done
for path in /usr/local/bin/rovunet-awg /usr/local/bin/rovunet-amneziawg-go /usr/local/bin/rovunet-awg-provenance.json; do
    [[ ! -e "$path" ]] || {
        echo "Runtime path already exists; preserve and inspect it: $path" >&2
        exit 1
    }
done
python3 - << 'PY'
import json,datetime
c=json.load(open('/etc/rovunet/config.json'))
now=datetime.datetime.now(datetime.timezone.utc)
for u in c.get('users',[]):
    expiry=u.get('expires_at')
    if not u.get('quota_gb') and (not expiry or datetime.datetime.fromisoformat(expiry.replace('Z','+00:00'))>now):
        break
else:raise SystemExit('At least one active account without a traffic limit is required for this four-protocol preview. No changes made.')
PY
mapfile -t tls_paths < <(
    python3 - "$tls_name" << 'PY'
import json,pathlib,sys
c=json.load(open('/opt/rovunet-trojan-test/server.json'))
for i in c.get('inbounds',[]):
    for p in i.get('streamSettings',{}).get('tlsSettings',{}).get('certificates',[]):
        cert,key=p.get('certificateFile',''),p.get('keyFile','')
        if cert and key and '\n' not in cert+key and pathlib.Path(cert).is_file() and pathlib.Path(key).is_file():
            print(cert);print(key);raise SystemExit(0)
raise SystemExit('Existing tested Trojan TLS paths could not be read')
PY
)
[[ ${#tls_paths[@]} == 2 ]] || {
    echo 'TLS preflight failed.' >&2
    exit 1
}
cert=${tls_paths[0]}
key=${tls_paths[1]}
openssl x509 -in "$cert" -noout -checkhost "$tls_name" -checkend 604800
cert_key=$(openssl x509 -in "$cert" -pubkey -noout | openssl pkey -pubin -outform DER | sha256sum | cut -d' ' -f1)
private_key=$(openssl pkey -in "$key" -pubout -outform DER | sha256sum | cut -d' ' -f1)
[[ "$cert_key" == "$private_key" ]] || {
    echo 'Certificate and key do not match.' >&2
    exit 1
}
backup=$(mktemp -d /root/rovunet-managed-preview.XXXXXXXX)
chmod 0700 "$backup"
cp -a -- "$target" "$backup/rovunet.previous"
"$target" backup --file "$backup/panel.backup" > "$backup/backup-create.log" 2>&1
"$package_dir/rovunet-linux-amd64" backup-check "$backup/panel.backup"
tar -czf "$backup/etc-rovunet.tgz" -C / etc/rovunet
for path in /root/.local/share/caddy /var/lib/caddy /var/lib/rovunet/usage; do
    if [[ -e "$path" ]]; then tar -czf "$backup/$(printf '%s' "$path" | tr / _).tgz" -C / "${path#/}"; fi
done
iptables-save > "$backup/iptables-before.txt"
systemctl list-units --all --type=service 'rovunet*' --no-pager --plain > "$backup/services-before.txt"
: > "$backup/created-cores"
: > "$backup/created-binaries"
python3 - "$state" "$backup" "$expected" << 'PY'
import json,pathlib,sys
p=pathlib.Path(sys.argv[1]);p.write_text(json.dumps(dict(backup=sys.argv[2],preview_sha256=sys.argv[3]))+'\n');p.chmod(0o600)
PY
abort() {
    local result=$1
    trap - ERR INT TERM
    echo 'Preview deployment interrupted; attempting scoped rollback.' >&2
    rollback || true
    exit "$result"
}
trap 'abort "$?"' ERR
trap 'abort 130' INT
trap 'abort 143' TERM
install -m 0755 rovunet-linux-amd64 "$target.preview-tmp"
mv -f -- "$target.preview-tmp" "$target"
for item in amneziawg-go awg; do
    path=/usr/local/bin/rovunet-"$item"
    echo "$path" >> "$backup/created-binaries"
    install -m 0755 "$item-linux-amd64" "$path"
done
echo /usr/local/bin/rovunet-awg-provenance.json >> "$backup/created-binaries"
install -m 0644 awg-provenance-linux-amd64.json /usr/local/bin/rovunet-awg-provenance.json
echo managed-awg >> "$backup/created-cores"
"$target" core-install --name managed-awg --protocol amneziawg --host "$host" --port 51840 --subnet 10.203.72.0/24
echo managed-ss2022 >> "$backup/created-cores"
"$target" core-install --name managed-ss2022 --protocol ss2022 --host "$host" --port 28444
echo managed-trojan >> "$backup/created-cores"
"$target" core-install --name managed-trojan --protocol trojan --host "$host" --port 30444 --sni "$tls_name" --cert "$cert" --key "$key"
echo managed-vlessenc >> "$backup/created-cores"
"$target" core-install --name managed-vlessenc --protocol vlessenc --host "$host" --port 31444
private_exports "$backup"
systemctl list-units --all --type=service 'rovunet*' --no-pager --plain > "$backup/services-after.txt"
trap - ERR INT TERM
"$target" --version
for name in "${cores[@]}"; do "$target" core-status "$name"; done
echo "Preview installed. Backup and private outputs: $backup"
