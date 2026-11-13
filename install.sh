#!/bin/bash
# Fetch this installer by an independently reviewed immutable commit/digest.
set -euo pipefail
umask 077
REPO="${ROVUNET_REPO:-msVPNco/rovunet-panel}"
INSTALL_DIR=/usr/local/bin
# Only a reviewed production public key belongs here. No downloaded key is trusted.
TRUST_ROOT_JSON='{"schema":1,"keys":[{"id":"885d8aba22365ce42e20527bfa00e82aed3cba2f69501f146ed0496a8bc3dcbe","spki":"MCowBQYDK2VwAyEAMfWBf2jAnb20LJvMoOfN1R/4OP0tSjPec+GWU+QXdqs=","purpose":"production"}]}'
TRUST_ENVIRONMENT=production
info() { printf '[+] %s\n' "$*"; }
error() {
    printf '[-] %s\n' "$*" >&2
    exit 1
}
[[ $EUID -eq 0 ]] || error 'Run this installer as root.'
[[ "$REPO" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || error 'Invalid release repository.'
[[ $# -eq 0 || ($# -eq 1 && "$1" == --no-run) ]] || error 'Usage: bash install.sh [--no-run]'
for tool in python3 openssl; do command -v "$tool" > /dev/null || error "Required verification tool is missing: $tool"; done
# Coordinated state must never be downgraded to a writer that ignores its fence.
python3 - << 'PY'
import json,pathlib,sys
p=pathlib.Path('/etc/rovunet/config.json')
if p.exists():
 c=json.loads(p.read_bytes())
 users=c.get('users',[])+c.get('archived',[])
 if c.get('subscription_v2_authority') or c.get('subscription_v2') or c.get('recovery_sealed') or any(u.get('sqlite_meter') for u in users):
  sys.exit('Coordinated installation: use its reviewed stop-and-upgrade/recovery procedure; no files were replaced.')
r=pathlib.Path('/var/lib/rovunet/subscriptions/.rotation-receipts')
if r.exists() and any(r.iterdir()):
 sys.exit('Operation receipts require coordinated forward recovery; the public bootstrap is disabled.')
PY
if command -v curl > /dev/null; then
    download() { (
        ulimit -f "$(((${3:-1048576} + 1023) / 1024))"
        curl --max-filesize "${3:-1048576}" --proto '=https' --proto-redir '=https' --tlsv1.2 --fail --silent --show-error --location --retry 2 --connect-timeout 15 --max-time 120 --output "$2" "$1"
    ); }
elif command -v wget > /dev/null; then
    download() { (
        ulimit -f "$(((${3:-1048576} + 1023) / 1024))"
        wget --https-only --timeout=120 --tries=3 --quiet --output-document="$2" "$1"
    ); }
else
    error 'Install curl or wget before running this installer.'
fi
[[ $(uname -s) == Linux ]] || error 'Only Linux is supported.'
case "$(uname -m)" in
    x86_64) ARCH=amd64 ;;
    aarch64) ARCH=arm64 ;;
    *) error 'Unsupported architecture.' ;;
esac
BINARY="rovunet-linux-$ARCH"
# Missing authority fails before touching a healthy installation.
python3 - "$TRUST_ROOT_JSON" "$TRUST_ENVIRONMENT" << 'PY'
import json,sys
r=json.loads(sys.argv[1])
if r.get('schema')!=1 or not r.get('keys') or any(k.get('purpose')!=sys.argv[2] for k in r['keys']):
    sys.exit('Release signing authority is not provisioned. Keep the current installation; obtain a reviewed signed package.')
PY
mkdir -p "$INSTALL_DIR"
stage=$(mktemp -d "$INSTALL_DIR/.rovunet-install.XXXXXXXX")
cleanup() { rm -rf -- "$stage"; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM HUP
RELEASE_TAG="${ROVUNET_RELEASE_TAG:-}"
if [[ -z "$RELEASE_TAG" ]]; then
    download "https://api.github.com/repos/$REPO/releases/latest" "$stage/release.json"
    RELEASE_TAG=$(
        python3 - "$stage/release.json" << 'PY'
import json,pathlib,sys
p=pathlib.Path(sys.argv[1])
if p.stat().st_size>1048576: sys.exit('Release metadata too large')
r=json.loads(p.read_bytes())
if r.get('draft') or r.get('prerelease'): sys.exit('Stable release required')
print(r['tag_name'])
PY
    )
fi
[[ "$RELEASE_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.-]+)?$ ]] || error 'Invalid release tag.'
base="https://github.com/$REPO/releases/download/$RELEASE_TAG"
info "Downloading the signed $RELEASE_TAG release for Linux/$ARCH..."
download "$base/RELEASE-MANIFEST.json" "$stage/manifest.json"
download "$base/RELEASE-MANIFEST.sig" "$stage/manifest.sig"
# Never execute a candidate to ask for its version. Verify exact signed bytes.
expected_size=$(
    python3 - "$stage" "$TRUST_ROOT_JSON" "$TRUST_ENVIRONMENT" "$REPO" "$RELEASE_TAG" "$BINARY" "$ARCH" << 'PY'
import base64,hashlib,json,pathlib,re,struct,subprocess,sys
d=pathlib.Path(sys.argv[1]); roots=json.loads(sys.argv[2])
def unique(pairs):
    out={}
    for k,v in pairs:
        if k in out: raise ValueError('duplicate manifest key')
        out[k]=v
    return out
raw=(d/'manifest.json').read_bytes(); sig=(d/'manifest.sig').read_bytes()
if not 0<len(raw)<=1048576 or len(sig)!=64: sys.exit('Missing or oversized release verification data')
m=json.loads(raw,object_pairs_hook=unique)
fields={'schema','environment','key_id','repository','tag','source_commit','assets'}
if set(m)!=fields or m['schema']!=1 or m['environment']!=sys.argv[3] or m['repository']!=sys.argv[4] or m['tag']!=sys.argv[5] or not re.fullmatch('[0-9a-f]{40}',m['source_commit']): sys.exit('Release identity does not match')
keys=[k for k in roots['keys'] if k['id']==m['key_id'] and k['purpose']==sys.argv[3]]
if len(keys)!=1: sys.exit('Untrusted release signing key')
der=base64.b64decode(keys[0]['spki'],validate=True)
if hashlib.sha256(der).hexdigest()!=keys[0]['id']: sys.exit('Trust root fingerprint mismatch')
(d/'anchor.der').write_bytes(der)
check=subprocess.run(['openssl','pkeyutl','-verify','-rawin','-pubin','-keyform','DER','-inkey',str(d/'anchor.der'),'-in',str(d/'manifest.json'),'-sigfile',str(d/'manifest.sig')],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
if check.returncode: sys.exit('Release signature is invalid')
if not isinstance(m['assets'],list) or not 0<len(m['assets'])<=256: sys.exit('Invalid release asset list')
seen=set(); selected=None
for a in m['assets']:
    fields={'name','os','arch','kind','size','sha256','source','source_commit','license'}
    if set(a)!=fields or not re.fullmatch('[A-Za-z0-9][A-Za-z0-9._-]{0,159}',a['name']) or a['name'] in seen or type(a['size']) is not int or not 0<a['size']<=536870912 or not re.fullmatch('[0-9a-f]{64}',a['sha256']) or not all(isinstance(a[k],str) and a[k] for k in ['source','source_commit','license']): sys.exit('Incomplete or duplicate release asset')
    seen.add(a['name'])
    if a['kind'] not in ['executable','data','installer']: sys.exit('Unknown release asset kind')
    if a['kind']=='executable' and (a['os']!='linux' or a['arch'] not in ['amd64','arm64']): sys.exit('Unsupported executable architecture')
    if a['name']==sys.argv[6]: selected=a
if selected is None or selected['kind']!='executable' or selected['os']!='linux' or selected['arch']!=sys.argv[7]: sys.exit('No matching authenticated executable')
(d/'selected.json').write_text(json.dumps(selected))
print(selected['size'])

PY
)
download "$base/$BINARY" "$stage/candidate" "$expected_size"
python3 - "$stage" "$ARCH" << 'PY'
import hashlib,json,pathlib,struct,sys
d=pathlib.Path(sys.argv[1]); selected=json.loads((d/'selected.json').read_text())
p=d/'candidate'
if p.stat().st_size!=selected['size']: sys.exit('Truncated or oversized release asset')
with p.open('rb') as f:
    if hashlib.file_digest(f,'sha256').hexdigest()!=selected['sha256']: sys.exit('Release asset digest mismatch')
with p.open('rb') as f: header=f.read(64)
machine={'amd64':62,'arm64':183}[sys.argv[2]]
if len(header)!=64 or header[:6]!=b'\x7fELF\x02\x01' or struct.unpack_from('<H',header,18)[0]!=machine or struct.unpack_from('<H',header,16)[0] not in [2,3]: sys.exit('ELF architecture mismatch')
with p.open('rb') as f:
    off=struct.unpack_from('<Q',header,32)[0]; size,count=struct.unpack_from('<HH',header,54)
    if size!=56 or not 0<count<=1024 or off<64 or off+size*count>selected['size']: sys.exit('Invalid ELF load table')
    f.seek(off); table=f.read(size*count)
    loads=[table[n*size:(n+1)*size] for n in range(count) if struct.unpack_from('<I',table,n*size)[0]==1]
    if not any(struct.unpack_from('<Q',h,32)[0]>0 and struct.unpack_from('<Q',h,8)[0]+struct.unpack_from('<Q',h,32)[0]<=selected['size'] for h in loads): sys.exit('ELF has no valid load segment')
PY
# Network and verification finish before any replacement. No process kills or
# service stops are necessary. Keep a complete recovery binary on the same FS.
python3 - "$stage" "$INSTALL_DIR" "$RELEASE_TAG" << 'PY'
import hashlib,json,os,pathlib,shutil,sys
stage=pathlib.Path(sys.argv[1]); root=pathlib.Path(sys.argv[2]); dest=root/'rovunet'; backup=root/'rovunet.bak'
journal=root/'.rovunet-install-state.json'
if journal.exists(): sys.exit('An earlier verified installation needs completion. Run rovunet install; retain rovunet.bak for recovery.')
if dest.is_symlink() or (dest.exists() and not dest.is_file()) or backup.is_symlink(): sys.exit('Refusing non-regular installation destination')
def sync_file(p,mode):
    os.chmod(p,mode)
    with p.open('rb') as f: os.fsync(f.fileno())
def sync_dir():
    fd=os.open(root,os.O_RDONLY|os.O_DIRECTORY)
    try: os.fsync(fd)
    finally: os.close(fd)
if dest.exists():
    shutil.copyfile(dest,stage/'previous'); sync_file(stage/'previous',0o755)
    os.replace(stage/'previous',backup); sync_dir()
state={'schema':1,'phase':'verified-before-replace','tag':sys.argv[3],'sha256':hashlib.sha256((stage/'candidate').read_bytes()).hexdigest(),'backup':str(backup) if backup.exists() else None}
(stage/'journal').write_text(json.dumps(state)+'\n'); sync_file(stage/'journal',0o600)
os.replace(stage/'journal',journal); sync_dir()
sync_file(stage/'candidate',0o755)
os.replace(stage/'candidate',dest); sync_dir()
PY
info 'Verified panel installed. An existing panel is retained at /usr/local/bin/rovunet.bak.'
if [[ ${1:-} == --no-run ]]; then
    info 'Setup is pending. Run: sudo rovunet install'
    exit 0
fi
info 'Starting setup. If interrupted, retry with: sudo rovunet install'
cleanup
trap - EXIT
exec "$INSTALL_DIR/rovunet" install < /dev/tty
