#!/usr/bin/env bash
# Checks that a traffic-limited user holds no credential on a service that does
# not count traffic. Run it as root on a THROWAWAY server with Rovunet Panel
# v1.18.0 or later and at least one Hysteria 2 or WireGuard core, a NaïveProxy
# tunnel and a SOCKS5 proxy. It creates users qafree1 and qaold1 and removes
# them again. It checks the server's files and accounts, not that a client
# really connects; see docs/testing/quota-access.md for the connection checks.
set -u
CONF=/etc/rovunet/config.json
CORES=/etc/rovunet/core
NAIVE_DIR=/etc/rovunet/tunnels
SOCKS=/etc/rovunet/socks-users.conf
pass=0; bad=0
# The panel's questions read /dev/tty, not stdin. setsid removes the terminal and
# </dev/null closes stdin, so a menu or question gets "no answer" instead of
# waiting for a keyboard.
rv() { setsid -w rovunet "$@" </dev/null; }
ok()   { echo "  ok    $*"; pass=$((pass+1)); }
fail() { echo "  FAIL  $*"; bad=$((bad+1)); }
[ "$(id -u)" = 0 ] || { echo "run as root"; exit 2; }
command -v rovunet >/dev/null || { echo "rovunet not found"; exit 2; }
command -v setsid >/dev/null || { echo "setsid (util-linux) needed"; exit 2; }
command -v python3 >/dev/null || { echo "python3 needed"; exit 2; }

cleanup() {
  rv users remove --username qafree1 >/dev/null 2>&1
  rv users remove --username qaold1 >/dev/null 2>&1
}
trap cleanup EXIT

echo "1. An unlimited user is served everywhere"
rv users bulk_add --count 1 --prefix qafree --days 0 --gb 0 >/dev/null 2>&1
id qafree1 >/dev/null 2>&1 && ok "qafree1 has an SSH account" || fail "qafree1 has no SSH account"
grep -q '^qafree1:' "$SOCKS" && ok "qafree1 is in the SOCKS5 credentials" || fail "qafree1 missing from SOCKS5"
for d in "$CORES"/*/; do
  p=$(python3 -c "import json,sys;print(json.load(open(sys.argv[1])).get('protocol',''))" "$d/options.json" 2>/dev/null)
  case "$p" in hysteria2|wireguard)
    ls "$d"clients/qafree1.* >/dev/null 2>&1 && ok "$(basename "$d") ($p) has a client file for qafree1" || fail "$(basename "$d") ($p) has no client for qafree1";; esac
done

echo "2. A user who was unlimited keeps a saved config, then is renewed with a quota"
rv users bulk_add --count 1 --prefix qaold --days 1 --gb 0 >/dev/null 2>&1
mkdir -p /tmp/qa-saved && cp -r "$CORES" /tmp/qa-saved/ 2>/dev/null
# Make the user expire now, stop them, then renew with a quota and the same password.
python3 - "$CONF" <<'PY'
import json,sys
p=sys.argv[1]; c=json.load(open(p))
for u in c["users"]:
    if u["username"]=="qaold1": u["expires_at"]="2020-01-01T00:00:00Z"
json.dump(c,open(p,"w"),indent=2)
PY
rv expire-users >/dev/null 2>&1
rv users renew --username qaold1 --days 1 --gb 1 >/dev/null 2>&1

id qaold1 >/dev/null 2>&1 && fail "limited user qaold1 has an SSH account" || ok "limited user qaold1 has no SSH account"
grep -q '^qaold1:' "$SOCKS" && fail "qaold1 is in the SOCKS5 credentials" || ok "qaold1 is not in the SOCKS5 credentials"
if grep -rq 'qaold1' "$NAIVE_DIR"/*/Caddyfile 2>/dev/null; then fail "qaold1 is in a NaïveProxy Caddyfile"; else ok "qaold1 is not in any NaïveProxy Caddyfile"; fi
for d in "$CORES"/*/; do
  p=$(python3 -c "import json;print(json.load(open('$d/options.json')).get('protocol',''))" 2>/dev/null)
  case "$p" in
    hysteria2|wireguard)
      ls "$d"clients/qaold1.* >/dev/null 2>&1 && fail "$(basename "$d") ($p) still has a client for qaold1" || ok "$(basename "$d") ($p) has no client for qaold1";;
    reality|xhttp)
      ls "$d"clients/qaold1.* >/dev/null 2>&1 && ok "$(basename "$d") ($p) still serves qaold1 (counted)" || fail "$(basename "$d") ($p) lost qaold1";;
  esac
done

echo "3. Nothing is left pending"
for d in "$CORES"/*/; do
  python3 -c "import json,sys;s=json.load(open('$d/state.json'));sys.exit(1 if s.get('revocation') else 0)" 2>/dev/null \
    && ok "$(basename "$d"): no revocation recorded" || fail "$(basename "$d"): revocation recorded"
done

echo; echo "passed $pass, failed $bad"
[ "$bad" = 0 ]
