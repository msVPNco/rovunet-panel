#!/usr/bin/env python3
"""Accept the final production-signed installer and official-2.0.2 update.

Linux/amd64 only. Uses real signed panels, real HTTPS, real SQLite, and reviewed
Xray/sing-box processes in private mount/network/PID namespaces. systemd is
simulated. The HTTPS transport CA is ephemeral; the production Ed25519 release
authority is unchanged. No VM, ISP, public DNS, ARM execution, or live Caddy/
Slipstream traffic claim is made. A receipt is written only after every phase.
"""
import argparse
import ast
import base64
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import re
import shutil
import signal
import ssl
import stat
import struct
import subprocess
import sys
import tempfile
import threading
from urllib.parse import urlsplit
from urllib.request import ProxyHandler, HTTPSHandler, build_opener

sys.dont_write_bytecode = True
REPO = 'msVPNco/rovunet-panel'
TAG, BASE_TAG = 'v2.1.0', 'v2.0.2'
BASE_COMMIT = 'efa697256880b0d2c0b1494513b51f4c72a7735b'
BASE_MANIFEST_SHA256 = '38dcbe40474787132f540d7804388c3f0e51f50a58ac9aaf765cf167a30e0f78'
KEY_ID = '885d8aba22365ce42e20527bfa00e82aed3cba2f69501f146ed0496a8bc3dcbe'
SPKI = 'MCowBQYDK2VwAyEAMfWBf2jAnb20LJvMoOfN1R/4OP0tSjPec+GWU+QXdqs='
PANELS = ('rovunet-linux-amd64', 'rovunet-linux-arm64')
RUNTIMES = ('caddy-naive-linux-amd64', 'slipstream-server-linux-amd64',
            'xray-linux-amd64', 'sing-box-linux-amd64')
BASE_FILES = ('rovunet-linux-amd64', 'install.sh') + RUNTIMES
CANDIDATE_FILES = PANELS + ('install.sh',)
CHECKS = ('signed_fresh_install', 'signed_menu_update', 'account_identity_and_usage', 'tls_trust_preservation')
SCOPE = {'environment': 'isolated-linux-namespaces', 'architecture': 'amd64',
         'systemd': 'simulated', 'https': 'real TLS with a private transport CA',
         'release_listing': 'private fixture; public GitHub release state is unchanged',
         'release_trust': 'unchanged production Ed25519 authority',
         'runtime_processes': 'reviewed Xray and sing-box',
         'traffic': 'listener/configuration acceptance; no ISP or proxy payload claim',
         'tls_front_and_slipstream_services': 'configured but administratively inactive',
         'arm64': 'signed bytes authenticated; not executed'}
CONFIG = Path('/etc/rovunet/config.json')
INSTALLED = Path('/usr/local/bin/rovunet')
SUBDIR = Path('/var/lib/rovunet/subscriptions/sub')
USER, TOKEN = 'fixturealice', 'b2' * 16
APP_HOST = 'n.acceptance.test'


def require(condition, message):
    if not condition:
        raise ValueError(message)


def unique(pairs):
    value = {}
    for key, item in pairs:
        require(key not in value, 'Duplicate JSON member')
        value[key] = item
    return value


def decode(raw):
    return json.loads(raw, object_pairs_hook=unique)


def canonical(value):
    return (json.dumps(value, sort_keys=True, separators=(',', ':')) + '\n').encode()


def digest(path):
    h = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(chunk)
    return h.hexdigest()


def private_write(path, data):
    path = Path(path)
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix='.acceptance-', dir=path.parent)
    try:
        with os.fdopen(fd, 'wb') as stream:
            os.fchmod(stream.fileno(), 0o600)
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        # Publish complete bytes without overwriting a prior receipt or symlink.
        os.link(temporary, path)
        directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        os.unlink(temporary)


def small_file(path, maximum):
    require(path.is_file() and not path.is_symlink(), 'Missing or nonregular verification input: ' + path.name)
    with path.open('rb') as stream:
        raw = stream.read(maximum + 1)
    require(0 < len(raw) <= maximum, 'Oversized verification input: ' + path.name)
    return raw


def verify_elf(path, arch):
    with path.open('rb') as stream:
        head = stream.read(64)
        require(len(head) == 64 and head[:6] == b'\x7fELF\x02\x01' and
                struct.unpack_from('<H', head, 18)[0] == {'amd64': 62, 'arm64': 183}[arch],
                'Authenticated executable has an unexpected ELF architecture')
        offset = struct.unpack_from('<Q', head, 32)[0]
        width, count = struct.unpack_from('<HH', head, 54)
        require(struct.unpack_from('<H', head, 16)[0] in (2, 3) and width == 56 and
                0 < count <= 1024 and offset >= 64 and offset + width * count <= path.stat().st_size,
                'Authenticated executable has an invalid ELF load table')
        stream.seek(offset)
        table = stream.read(width * count)
        require(any(struct.unpack_from('<I', table, n * width)[0] == 1 and
                    struct.unpack_from('<Q', table, n * width + 32)[0] > 0 and
                    struct.unpack_from('<Q', table, n * width + 8)[0] +
                    struct.unpack_from('<Q', table, n * width + 32)[0] <= path.stat().st_size
                    for n in range(count)), 'Authenticated executable has no valid load segment')


def authenticate(directory, tag, required):
    raw = small_file(directory / 'RELEASE-MANIFEST.json', 1048576)
    signature = small_file(directory / 'RELEASE-MANIFEST.sig', 64)
    require(len(signature) == 64, 'Production Ed25519 signature must contain exactly64 bytes')
    manifest = decode(raw)
    require(set(manifest) == {'schema', 'environment', 'key_id', 'repository', 'tag', 'source_commit', 'assets'} and
            type(manifest['schema']) is int and manifest['schema'] == 1 and manifest['environment'] == 'production' and
            manifest['key_id'] == KEY_ID and manifest['repository'] == REPO and manifest['tag'] == tag and
            re.fullmatch('[0-9a-f]{40}', manifest['source_commit']), 'Unexpected production release identity')
    if tag == BASE_TAG:
        require(manifest['source_commit'] == BASE_COMMIT and hashlib.sha256(raw).hexdigest() == BASE_MANIFEST_SHA256,
                'Baseline is not the exact authenticated official2.0.2 release')
    public = base64.b64decode(SPKI, validate=True)
    require(hashlib.sha256(public).hexdigest() == KEY_ID, 'Production root fingerprint mismatch')
    with tempfile.TemporaryDirectory(prefix='signed-release-auth-', dir='/var/tmp') as temporary:
        root = Path(temporary)
        for name, data in [('public.der', public), ('manifest', raw), ('signature', signature)]:
            private_write(root / name, data)
        result = subprocess.run(['openssl', 'pkeyutl', '-verify', '-rawin', '-pubin', '-keyform', 'DER',
                                 '-inkey', str(root / 'public.der'), '-in', str(root / 'manifest'),
                                 '-sigfile', str(root / 'signature')], capture_output=True, timeout=15)
        require(result.returncode == 0, 'Original production Ed25519 authority rejected ' + tag)
    require(isinstance(manifest['assets'], list) and 0 < len(manifest['assets']) <= 256, 'Invalid signed asset inventory')
    entries = {}
    for entry in manifest['assets']:
        require(set(entry) == {'name', 'size', 'sha256', 'os', 'arch', 'kind', 'source', 'source_commit', 'license'},
                'Incomplete signed asset metadata')
        name = entry['name']
        require(isinstance(name, str) and re.fullmatch('[A-Za-z0-9][A-Za-z0-9._-]{0,159}', name) and
                name not in entries and name not in ('RELEASE-MANIFEST.json', 'RELEASE-MANIFEST.sig') and
                type(entry['size']) is int and 0 < entry['size'] <= 536870912 and
                re.fullmatch('[0-9a-f]{64}', entry['sha256']) and
                all(isinstance(entry[k], str) and entry[k] for k in ('source', 'source_commit', 'license')),
                'Invalid or duplicate signed asset')
        entries[name] = entry
    for name in required:
        require(name in entries, 'Signed release omits required asset: ' + name)
        path, entry = directory / name, entries[name]
        require(path.is_file() and not path.is_symlink() and path.stat().st_size == entry['size'] and
                digest(path) == entry['sha256'], 'Signed asset bytes differ: ' + name)
        if name != 'install.sh':
            arch = name.rsplit('-', 1)[1]
            require(entry['kind'] == 'executable' and entry['os'] == 'linux' and entry['arch'] == arch,
                    'Unexpected executable asset metadata')
            verify_elf(path, arch)
    installer = (directory / 'install.sh').read_text()
    match = re.search(r"^TRUST_ROOT_JSON='([^']*)'$", installer, re.M)
    expected = {'schema': 1, 'keys': [{'id': KEY_ID, 'spki': SPKI, 'purpose': 'production'}]}
    require(match is not None and decode(match[1]) == expected and '\nTRUST_ENVIRONMENT=production\n' in installer,
            'Authenticated installer no longer embeds the original production root')
    return {'manifest': manifest, 'entries': entries, 'directory': directory,
            'manifest_sha256': hashlib.sha256(raw).hexdigest()}


def checked_pair(candidate, baseline):
    current = authenticate(candidate, TAG, CANDIDATE_FILES)
    previous = authenticate(baseline, BASE_TAG, BASE_FILES)
    for name in ('install.sh',) + RUNTIMES:
        require(current['entries'].get(name, {}).get('sha256') == previous['entries'][name]['sha256'],
                'Expected unchanged official installer/runtime asset: ' + name)
    return current, previous


class Calls:
    def __init__(self, work, phase):
        self.root, self.phase, self.items = work / 'logs', phase, []
        self.root.mkdir(mode=0o700, parents=True)

    def run(self, label, *command, input=b'', success=True, timeout=120, env=None):
        path = self.root / (label + '.log')
        with path.open('xb') as log:
            result = subprocess.run(list(map(str, command)), input=input, stdout=log, stderr=subprocess.STDOUT,
                                    env=env, timeout=timeout)
        require(path.stat().st_size <= 16 << 20, 'Unexpectedly large acceptance command log')
        self.items.append({'name': 'logs/' + self.phase + '/' + path.name,
                           'sha256': digest(path), 'exit_code': result.returncode})
        require((result.returncode == 0) == success, 'Unexpected command result: ' + label)
        return path.read_bytes()


class Releases:
    def __init__(self, current, previous, calls, cert, key):
        self.current, self.previous, self.calls = current, previous, calls
        self.fault = None
        self.requests = []
        fixture = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *unused):
                pass

            def do_GET(self):
                host = self.headers.get('Host', '').split(':', 1)[0]
                parsed = urlsplit(self.path)
                fixture.requests.append({'host': host, 'path': parsed.path, 'fault': fixture.fault})
                if parsed.query or parsed.fragment:
                    self.send_error(404)
                    return
                if host == 'api.github.com' and parsed.path == '/repos/' + REPO + '/releases/latest':
                    selected_tag = current['manifest']['tag']
                    data = canonical({'tag_name': selected_tag, 'draft': False, 'prerelease': False,
                                      'assets': [{'name': name, 'browser_download_url':
                                          'https://github.com/' + REPO + '/releases/download/' + selected_tag + '/' + name}
                                          for name in current['entries']]})
                    self.send_response(200)
                    self.send_header('Content-Length', str(len(data)))
                    self.end_headers()
                    self.wfile.write(data)
                    return
                for release in (current, previous):
                    prefix = '/' + REPO + '/releases/download/' + release['manifest']['tag'] + '/'
                    if host != 'github.com' or not parsed.path.startswith(prefix):
                        continue
                    name = parsed.path[len(prefix):]
                    if name not in release['entries'] and name not in ('RELEASE-MANIFEST.json', 'RELEASE-MANIFEST.sig'):
                        break
                    path = release['directory'] / name
                    if not path.is_file():
                        break
                    self.send_response(200)
                    self.send_header('Content-Length', str(path.stat().st_size))
                    self.end_headers()
                    with path.open('rb') as stream:
                        first = True
                        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
                            corrupt = release is current and first and (
                                fixture.fault == 'signature' and name == 'RELEASE-MANIFEST.sig' or
                                fixture.fault == 'panel' and name == 'rovunet-linux-amd64')
                            if corrupt:
                                chunk = bytes([chunk[0] ^ 1]) + chunk[1:]
                            try:
                                self.wfile.write(chunk)
                            except (BrokenPipeError, ConnectionResetError):
                                return
                            first = False
                    return
                self.send_error(404)

        self.server = ThreadingHTTPServer(('127.0.0.1', 443), Handler)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.minimum_version = ssl.TLSVersion.TLSv1_2
        context.load_cert_chain(cert, key)
        self.server.socket = context.wrap_socket(self.server.socket, server_side=True)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def close(self):
        self.server.shutdown()
        self.server.server_close()


def make_certificate(calls, root, name, hosts, ca=None):
    cert, key = root / (name + '.crt'), root / (name + '.key')
    config = root / (name + '.cnf')
    config.write_text('[req]\ndistinguished_name=dn\nprompt=no\n[dn]\nCN=' + hosts[0] +
                      '\n[extensions]\nsubjectAltName=' + ','.join('DNS:' + host for host in hosts) +
                      '\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\n' +
                      'basicConstraints=critical,CA:FALSE\n')
    request = root / (name + '.csr')
    calls.run(name + '-key', 'openssl', 'req', '-new', '-newkey', 'rsa:2048', '-nodes',
              '-keyout', key, '-out', request, '-config', config)
    issuer = ['-CA', str(ca[0]), '-CAkey', str(ca[1]), '-CAcreateserial'] if ca else ['-signkey', str(key)]
    calls.run(name + '-certificate', 'openssl', 'x509', '-req', '-in', request, *issuer,
              '-out', cert, '-days', '2', '-extfile', config, '-extensions', 'extensions')
    os.chmod(key, 0o600)
    return cert, key


def namespace_environment(work, calls):
    for kind in ('mnt', 'net', 'pid'):
        original = os.environ.get('ROVUNET_SIGNED_PARENT_' + kind.upper())
        require(original and original != os.readlink('/proc/self/ns/' + kind), 'Private namespaces are required')
    require(os.getpid() == 1, 'Acceptance process must be PID1 in its private namespace')
    tools = {}
    for name in ('bash', 'python3', 'openssl', 'curl', 'ip', 'iptables', 'iptables-save', 'iptables-restore',
                 'ip6tables', 'ip6tables-save', 'ip6tables-restore'):
        found = shutil.which(name)
        require(found is not None, 'Required acceptance tool is missing: ' + name)
        tools[name] = Path(found).resolve()
    fake = work / 'tools'
    fake.mkdir(mode=0o700)
    for name, source in tools.items():
        if str(source).startswith(('/usr/local/bin/', '/home/', '/root/', '/tmp/')):
            shutil.copy2(source, fake / name)
        else:
            (fake / name).symlink_to(source)
    simulator_path = Path(__file__).with_name('test-complete-setup.py')
    tree = ast.parse(simulator_path.read_text())
    simulator = next(node.value.value for node in tree.body if isinstance(node, ast.Assign) and
                     any(isinstance(target, ast.Name) and target.id == 'SYSTEMCTL' for target in node.targets))
    private_write(fake / 'systemctl', simulator.encode())
    os.chmod(fake / 'systemctl', 0o755)
    calls.run('private-mount-propagation', 'mount', '--make-rprivate', '/')
    for index, target in enumerate(('/etc', '/var/lib', '/var/log', '/usr/local/bin', '/root', '/home', '/run', '/tmp')):
        source = work / ('mount-' + str(index))
        source.mkdir(mode=0o700)
        calls.run('private-mount-' + str(index), 'mount', '--bind', source, target)
    state = work / 'systemd'
    state.mkdir(mode=0o700)
    os.environ['COMPLETE_TEST_STATE'] = str(state)
    os.environ['PATH'] = str(fake) + ':/usr/sbin:/usr/bin:/sbin:/bin'
    os.environ['TERM'] = 'dumb'
    for directory in ('/etc/systemd/system', '/etc/ssl/certs', '/etc/rovunet/tunnels',
                      '/var/lib/rovunet/usage', '/var/lib/rovunet-quota'):
        Path(directory).mkdir(mode=0o700, parents=True, exist_ok=True)
    Path('/etc/passwd').write_text('root:x:0:0:root:/root:/bin/bash\nrovunet:x:10001:10001:fixture:/var/lib/rovunet:/usr/sbin/nologin\n')
    Path('/etc/group').write_text('root:x:0:\nrovunet:x:10001:\n')
    Path('/etc/nsswitch.conf').write_text('hosts: files dns\npasswd: files\ngroup: files\n')
    Path('/etc/hosts').write_text('127.0.0.1 localhost github.com api.github.com ' + APP_HOST + ' slip.acceptance.test\n')
    Path('/etc/resolv.conf').write_text('nameserver 127.0.0.1\noptions timeout:1 attempts:1\n')
    Path('/etc/openssl.cnf').write_text('[req]\ndistinguished_name=dn\n[dn]\nCN=acceptance\n')
    os.environ['OPENSSL_CONF'] = '/etc/openssl.cnf'
    calls.run('private-loopback', 'ip', 'link', 'set', 'lo', 'up')
    links = decode(calls.run('private-interfaces', 'ip', '-json', 'link', 'show'))
    require({item['ifname'] for item in links} == {'lo'}, 'Acceptance namespace unexpectedly has an external interface')
    ca_cert, ca_key = work / 'transport-ca.crt', work / 'transport-ca.key'
    calls.run('transport-ca', 'openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '2',
              '-subj', '/CN=Rovunet acceptance TRANSPORT CA', '-keyout', ca_key, '-out', ca_cert,
              '-addext', 'basicConstraints=critical,CA:TRUE', '-addext', 'keyUsage=critical,keyCertSign,cRLSign')
    cert, key = make_certificate(calls, work, 'github-transport', ['github.com', 'api.github.com'], (ca_cert, ca_key))
    shutil.copyfile(ca_cert, '/etc/ssl/certs/ca-certificates.crt')
    os.environ['CURL_CA_BUNDLE'] = '/etc/ssl/certs/ca-certificates.crt'
    os.environ['SSL_CERT_FILE'] = '/etc/ssl/certs/ca-certificates.crt'
    os.environ['SSL_CERT_DIR'] = '/etc/ssl/certs'
    return cert, key, state


def file_hashes(root):
    return {str(path.relative_to(root)): digest(path) for path in root.rglob('*')
            if stat.S_ISREG(path.lstat().st_mode)}


def account_snapshot(calls, label):
    cfg = decode(CONFIG.read_bytes())
    status = decode(calls.run(label + '-sqlite', INSTALLED, 'unlimited-accounting', 'status', '--user', USER))
    require(status['backend'] == 'sqlite' and status['confirmed_bytes'] == 1217 and status['pending_bytes'] == 0 and
            status['limit_kind'] == 'unlimited' and status['period_preserved'], 'Recorded account usage did not remain1217 bytes')
    feeds = {}
    for suffix in ('.json', '.windows.json'):
        value = decode((SUBDIR / (TOKEN + suffix)).read_bytes())
        require(value['version'] == 1 and value['name'] == USER, 'Subscription identity changed')
        feeds[suffix] = {item['id']: item['link'] for item in value['items']}
    return {'config_sha256': digest(CONFIG), 'users': cfg['users'], 'server_id': cfg['server_id'],
            'tunnels': cfg['tunnels'], 'status': status, 'feeds': feeds,
            'usage_files': file_hashes(Path('/var/lib/rovunet/usage')),
            'urls': ['https://' + APP_HOST + ':8443/sub/' + TOKEN + suffix for suffix in ('.json', '.windows.json')]}


def tls_files(calls, work):
    front = Path('/root/.local/share/caddy/certificates/acceptance') / APP_HOST
    front.mkdir(mode=0o700, parents=True)
    front_cert, front_key = make_certificate(calls, front, APP_HOST, [APP_HOST])
    slip = Path('/etc/rovunet/tunnels/slipstream-socks')
    slip.mkdir(mode=0o700, parents=True)
    slip_cert, slip_key = make_certificate(calls, slip, 'slipstream', ['slip.acceptance.test'])
    return front_cert, front_key, slip_cert, slip_key


def check_trust(files):
    trust = decode((SUBDIR / (TOKEN + '.trust.json')).read_bytes())
    require(trust['server_id'] == 'a1' * 16 and trust['account_id'] == 'c3' * 16 and trust['period_id'] == 'd4' * 16 and
            len(trust['items']) == 1 and trust['items'][0]['profile_id'] == 'slipstream-socks', 'Slipstream trust identity changed')
    cert = files[2].read_text()
    der = ssl.PEM_cert_to_DER_cert(cert)
    require(trust['items'][0]['certificate_pem'] == cert and
            trust['items'][0]['certificate_sha256'] == hashlib.sha256(der).hexdigest(), 'Slipstream trust fingerprint changed')
    return {'files': {str(path): digest(path) for path in files},
            'trust_sha256': digest(SUBDIR / (TOKEN + '.trust.json')),
            'certificate_sha256': trust['items'][0]['certificate_sha256']}


def replacements(calls, old_hashes):
    core = Path('/etc/rovunet/core')
    names = {path.name for path in core.iterdir() if path.is_dir()}
    require(names == {'managed-ss2022-aes128', 'managed-ss-aead-chacha20'}, 'Partial update installed unrelated automatic cores')
    owners = decode(Path('/etc/rovunet/firewall-owners.json').read_bytes())
    proof = {}
    for previous, name, protocol, method, port in (
        ('wireguard', 'managed-ss2022-aes128', 'ss2022-aes128', '2022-blake3-aes-128-gcm', 51822),
        ('managed-tuic', 'managed-ss-aead-chacha20', 'ss-aead-chacha20', 'chacha20-ietf-poly1305', 33446),
    ):
        options = decode((core / name / 'options.json').read_bytes())
        require(options['protocol'] == protocol and options['port'] == port and options['dedicatedXray'],
                'Replacement changed the saved port, alias or dedicated runtime')
        unit = (Path('/etc/systemd/system') / ('rovunet-core-' + name + '.service')).read_text()
        require('ExecStart=/usr/local/bin/rovunet-xray-managed run -c ' + str(core / name / 'server.json') in unit,
                'Replacement unit fell back to another executable')
        config = decode((core / name / 'server.json').read_bytes())
        inbound = [value for value in config['inbounds'] if value['protocol'] == 'shadowsocks']
        require(len(inbound) == 1 and inbound[0]['port'] == port and inbound[0]['settings']['network'] == 'tcp,udp',
                'Replacement server is not the requested TCP/UDP listener')
        settings = inbound[0]['settings']
        require(len(settings['clients']) == 1 and
                (settings.get('method') == method if protocol == 'ss2022-aes128' else
                 not settings.get('method') and settings['clients'][0]['method'] == method), 'Replacement cipher differs')
        calls.run('confirm-' + name, INSTALLED, 'core-confirm', name)
        record = decode((core.parent / 'core-replacements' / (previous + '.json')).read_bytes())
        require(record['stage'] == 'done' and record['port'] == port and record['new_name'] == name and record['was_active'],
                'Replacement recovery record did not complete')
        require(file_hashes(Path(record['backup'])) == old_hashes[previous], 'Replacement backup did not retain exact old files')
        for proto in ('tcp', 'udp'):
            require(owners[str(port) + '/' + proto]['owners'] == {'core:' + name: True}, 'Replacement lost firewall ownership')
            calls.run('firewall-' + name + '-' + proto, 'iptables', '-C', 'INPUT', '-p', proto, '--dport', port, '-j', 'ACCEPT')
        proof[previous] = {'replacement': name, 'protocol': protocol, 'method': method, 'saved_port': port,
                           'confirmed': True, 'original_files_backed_up': True, 'tcp_udp_firewall_retained': True}
    return proof


def fresh_phase(work, calls, releases):
    installer = releases.current['directory'] / 'install.sh'
    rejected = []
    for fault in ('signature', 'panel'):
        releases.fault = fault
        calls.run('fresh-reject-' + fault, 'bash', installer, '--no-run', success=False)
        require(not INSTALLED.exists() and not CONFIG.exists() and not INSTALLED.with_name('.rovunet-install-state.json').exists(),
                'Unauthenticated bootstrap mutated installed state')
        rejected.append(fault)
    releases.fault = None
    calls.run('fresh-signed-installer', 'bash', installer, '--no-run')
    require(digest(INSTALLED) == releases.current['entries'][PANELS[0]]['sha256'] and not CONFIG.exists(),
            'Fresh installer did not stage the exact signed panel without running setup')
    receipt = INSTALLED.with_name('.rovunet-install-state.json')
    before = receipt.read_bytes()
    parsed = decode(before)
    require(parsed['schema'] == 1 and parsed['phase'] == 'verified-before-replace' and parsed['tag'] == releases.current['manifest']['tag'] and
            parsed['sha256'] == digest(INSTALLED), 'Fresh bootstrap recovery receipt is incorrect')
    calls.run('fresh-pending-repeat', 'bash', installer, '--no-run', success=False)
    require(receipt.read_bytes() == before and digest(INSTALLED) == parsed['sha256'], 'Pending bootstrap retry changed the verified installation')
    require(not (work / 'systemd' / 'calls').exists(), 'Fresh --no-run installer invoked service management')
    return {'signed_fresh_install': {'installed_sha256': digest(INSTALLED), 'installer_sha256': digest(installer),
            'negative_cases_rejected': rejected, 'setup_pending': True, 'pending_repeat_preserved': True}}


def initialize_upgrade_fixture(work, calls, releases):
    previous = releases.previous
    shutil.copyfile(previous['directory'] / PANELS[0], INSTALLED)
    os.chmod(INSTALLED, 0o755)
    for name in ('caddy-naive', 'slipstream-server'):
        shutil.copyfile(previous['directory'] / (name + '-linux-amd64'), INSTALLED.parent / name)
        os.chmod(INSTALLED.parent / name, 0o755)
    version = calls.run('baseline-version', INSTALLED, '--version').decode()
    require('v2.0.2' in version and BASE_COMMIT[:7] in version and '-dev' not in version, 'Unexpected official baseline version')
    files = tls_files(calls, work)
    Path('/etc/rovunet/tunnels/naive').mkdir(mode=0o700)
    cfg = {'listen': {'address': '127.0.0.1:5300'}, 'route': {'mode': 'multi'},
           'public_address': APP_HOST, 'server_id': 'a1' * 16, 'instance_id': 'e5' * 16,
           'backends': [{'tag': 'socks', 'type': 'socks', 'address': '127.0.0.1:1080'}],
           'users': [{'username': USER, 'password': 'synthetic-private-password', 'quota_gb': 0,
                      'sub_token': TOKEN, 'account_id': 'c3' * 16, 'period_id': 'd4' * 16, 'usage_base': 17}],
           'tunnels': [
               {'tag': 'naive', 'transport': 'naive', 'backend': 'socks', 'domain': APP_HOST, 'enabled': True,
                'naive': {'email': 'fixture@example.test', 'decoy_url': 'https://www.example.com', 'port': 8443}},
               {'tag': 'slipstream-socks', 'transport': 'slipstream', 'backend': 'socks',
                'domain': 'slip.acceptance.test', 'port': 5310, 'enabled': True,
                'slipstream': {'cert': str(files[2]), 'key': str(files[3])}}]}
    private_write(CONFIG, canonical(cfg))
    private_write(Path('/var/lib/rovunet/usage/vless.json'), canonical({USER: 1234}))
    migration = Path('/var/lib/rovunet-quota/signed-acceptance')
    calls.run('baseline-account-plan', INSTALLED, 'unlimited-accounting', 'plan', CONFIG, migration)
    calls.run('baseline-account-apply', INSTALLED, 'unlimited-accounting', 'apply', migration)
    calls.run('baseline-post-update', INSTALLED, 'post-update', '--strict', timeout=180)
    calls.run('baseline-wireguard', INSTALLED, 'core-install', '--name', 'wireguard', '--protocol', 'wireguard',
              '--host', APP_HOST, '--port', '51822', timeout=180)
    calls.run('baseline-tuic', INSTALLED, 'core-install', '--name', 'managed-tuic', '--protocol', 'tuic',
              '--host', APP_HOST, '--port', '33446', '--sni', APP_HOST, '--cert', files[0], '--key', files[1], timeout=180)
    for name in ('wireguard', 'managed-tuic'):
        calls.run('baseline-confirm-' + name, INSTALLED, 'core-confirm', name)
        active = calls.run('baseline-active-' + name, 'systemctl', 'is-active', 'rovunet-core-' + name + '.service')
        require(active.strip() == b'active', 'Baseline managed engine is not running')
    calls.run('baseline-refresh', INSTALLED, 'subscriptions-refresh')
    before = account_snapshot(calls, 'before')
    trust_before = check_trust(files)
    old_core_files = {name: file_hashes(Path('/etc/rovunet/core') / name) for name in ('wireguard', 'managed-tuic')}
    require(all({'wireguard', 'managed-tuic'} <= set(feed) for feed in before['feeds'].values()), 'Baseline feed is missing old managed defaults')
    return files, before, trust_before, old_core_files


def upgrade_phase(work, calls, releases):
    previous = releases.previous
    files, before, trust_before, old_core_files = initialize_upgrade_fixture(work, calls, releases)
    # Real updater must reject a bad production signature before its transaction.
    releases.fault = 'signature'
    calls.run('update-reject-signature', INSTALLED, 'update', success=False)
    require(digest(INSTALLED) == previous['entries'][PANELS[0]]['sha256'] and
            account_snapshot(calls, 'after-rejection') == before and check_trust(files) == trust_before and
            not INSTALLED.with_name('.rovunet-update-state.json').exists(), 'Rejected update changed account or installation state')
    releases.fault = None
    menu = calls.run('signed-menu-update', INSTALLED, input=b'8\n', timeout=300)
    require(b'Update complete!' in menu and b'Restart rovunet to use the new version.' in menu,
            'Actual menu update did not complete')
    current = releases.current
    require(digest(INSTALLED) == current['entries'][PANELS[0]]['sha256'] and
            digest(INSTALLED.with_name('rovunet.bak')) == previous['entries'][PANELS[0]]['sha256'],
            'Updater did not atomically install exact signed bytes and retain the baseline')
    transaction = decode(INSTALLED.with_name('.rovunet-update-state.json').read_bytes())
    require(transaction['phase'] == 'complete' and transaction['previous_sha256'] == previous['entries'][PANELS[0]]['sha256'] and
            transaction['installed_sha256'] == current['entries'][PANELS[0]]['sha256'], 'Signed update transaction remains pending')
    snapshot = Path(transaction['recovery']) / 'snapshot'
    require(snapshot.is_dir() and (snapshot / 'config.json').read_bytes() == CONFIG.read_bytes() and
            (snapshot / 'manifest.json').is_file(), 'Coherent account recovery snapshot was not retained')
    proof = replacements(calls, old_core_files)
    require(digest(Path('/usr/local/bin/rovunet-xray-managed')) == previous['entries']['xray-linux-amd64']['sha256'],
            'Replacement runtime differs from exact authenticated2.0.2 bytes')
    after = account_snapshot(calls, 'after')
    for key in ('config_sha256', 'users', 'server_id', 'tunnels', 'status', 'urls'):
        require(after[key] == before[key], 'Update changed existing account field: ' + key)
    for path, value in before['usage_files'].items():
        require(after['usage_files'].get(path) == value, 'Update rewrote existing usage source')
    for suffix, feed in before['feeds'].items():
        expected = {key: value for key, value in feed.items() if key not in ('wireguard', 'managed-tuic')}
        actual = after['feeds'][suffix]
        require(all(actual.get(key) == value for key, value in expected.items()) and
                set(actual) == set(expected) | {'managed-ss2022-aes128', 'managed-ss-aead-chacha20'},
                'Update changed an unrelated subscription profile or missed a replacement')
    require(check_trust(files) == trust_before, 'Signed update rotated an existing TLS identity')
    stable_cores = file_hashes(Path('/etc/rovunet/core'))
    stable_private = file_hashes(Path('/etc/rovunet/core-replacements'))
    calls.run('normal-update-retry', INSTALLED, 'update', timeout=180)
    require(account_snapshot(calls, 'retry') == after and check_trust(files) == trust_before and
            file_hashes(Path('/etc/rovunet/core')) == stable_cores and
            file_hashes(Path('/etc/rovunet/core-replacements')) == stable_private and
            digest(INSTALLED.with_name('rovunet.bak')) == previous['entries'][PANELS[0]]['sha256'],
            'Normal update retry changed the accepted installation or recovery')
    updated_version = calls.run('updated-version', INSTALLED, '--version').decode()
    require('v2.1.0' in updated_version and current['manifest']['source_commit'] in updated_version and '-dev' not in updated_version,
            'Installed version does not identify the exact reviewed stable source')
    service_calls = (work / 'systemd' / 'calls').read_text().splitlines()
    for name in ('rovunet-naive.service', 'rovunet-slipstream-socks.service'):
        require(not any(line in service_calls for line in ('start ' + name, 'restart ' + name, 'stop ' + name)),
                'Update changed an administratively inactive transport service')
    return {
        'signed_menu_update': {'menu_choice': 8, 'transaction_phase': transaction['phase'],
            'previous_sha256': transaction['previous_sha256'], 'installed_sha256': transaction['installed_sha256'],
            'rejected_bad_signature': True, 'coherent_recovery_retained': True,
            'normal_update_retry_preserved': True, 'partial_setup_replacements': proof},
        'account_identity_and_usage': {'account_fields_preserved': True, 'server_identity_preserved': True,
            'config_sha256': after['config_sha256'], 'period_preserved': True, 'confirmed_bytes': 1217,
            'pending_bytes': 0, 'sqlite_binding_preserved': True, 'original_usage_sources_preserved': True,
            'subscription_urls_preserved': True, 'unrelated_profile_links_preserved': True},
        'tls_trust_preservation': {'tls_key_and_certificate_hashes': trust_before['files'],
            'slipstream_certificate_sha256': trust_before['certificate_sha256'],
            'subscription_trust_sha256': trust_before['trust_sha256'],
            'private_key_bytes_preserved': True, 'same_public_trust_capability': True,
            'inactive_transport_services_preserved': True},
    }


def inside(args):
    # Do not write system paths before establishing namespace isolation.
    for kind in ('mnt', 'net', 'pid'):
        require(os.environ.get('ROVUNET_SIGNED_PARENT_' + kind.upper()) not in (None, os.readlink('/proc/self/ns/' + kind)),
                'Internal acceptance entry requires new namespaces')
    args.work.mkdir(mode=0o700, parents=True)
    calls = Calls(args.work, args.phase)
    if args.baseline_smoke:
        require(args.phase == 'smoke' and args.candidate is None and args.receipt is None,
                'Baseline smoke cannot accept a candidate or write a final receipt')
        current = previous = authenticate(args.baseline, BASE_TAG, BASE_FILES)
    else:
        current, previous = checked_pair(args.candidate, args.baseline)
    cert, key, state = namespace_environment(args.work, calls)
    releases = Releases(current, previous, calls, cert, key)
    try:
        # Verify real TLS with hostname checking; no curl shim or insecure flag.
        transport_context = ssl.create_default_context(cafile='/etc/ssl/certs/ca-certificates.crt')
        opener = build_opener(ProxyHandler({}), HTTPSHandler(context=transport_context))
        with opener.open('https://api.github.com/repos/' + REPO + '/releases/latest', timeout=10) as response:
            require(decode(response.read())['tag_name'] == current['manifest']['tag'], 'Private HTTPS release endpoint failed')
        if args.baseline_smoke:
            fresh = fresh_phase(args.work, calls, releases)
            files, before, trust, cores = initialize_upgrade_fixture(args.work, calls, releases)
            private_write(calls.root / 'https-requests.json', canonical(releases.requests))
            if (state / 'calls').exists():
                shutil.copyfile(state / 'calls', calls.root / 'systemctl.log')
            smoke = {'schema': 1, 'purpose': 'rovunet-signed-baseline-harness-smoke', 'status': 'pass',
                     'tag': BASE_TAG, 'source_commit': BASE_COMMIT, 'manifest_sha256': BASE_MANIFEST_SHA256,
                     'panel_sha256': previous['entries'][PANELS[0]]['sha256'], 'scope': SCOPE,
                     'runner_sha256': digest(Path(__file__)),
                     'simulator_sha256': digest(Path(__file__).with_name('test-complete-setup.py')),
                     'production_signature_verified': True, 'final_v210_acceptance': False,
                     'signed_baseline_fresh_installer': fresh['signed_fresh_install'],
                     'fixture': {'sqlite_confirmed_bytes': before['status']['confirmed_bytes'],
                                 'active_legacy_cores': sorted(cores), 'tls_trust_present': bool(trust)},
                     'executions': calls.items}
            private_write(args.work / 'BASELINE-SMOKE.json', canonical(smoke))
            print(json.dumps({'baseline_smoke': 'PASS', 'tag': BASE_TAG, 'final_v210_acceptance': False}), flush=True)
            return
        details = fresh_phase(args.work, calls, releases) if args.phase == 'fresh' else upgrade_phase(args.work, calls, releases)
        request_path = calls.root / 'https-requests.json'
        private_write(request_path, canonical(releases.requests))
        call_log = state / 'calls'
        if call_log.exists():
            shutil.copyfile(call_log, calls.root / 'systemctl.log')
        binding = {'schema': 1, 'tag': TAG, 'source_commit': current['manifest']['source_commit'],
                   'manifest_sha256': current['manifest_sha256'], 'production_key_id': KEY_ID,
                   'runner_sha256': digest(Path(__file__)),
                   'simulator_sha256': digest(Path(__file__).with_name('test-complete-setup.py')),
                   'panel_sha256': {name: current['entries'][name]['sha256'] for name in PANELS},
                   'baseline_panel_sha256': previous['entries'][PANELS[0]]['sha256'],
                   'scope': SCOPE, 'executions': calls.items,
                   'https_requests_sha256': digest(request_path)}
        for name, result in details.items():
            private_write(args.work / 'evidence' / (name + '.json'), canonical(dict(binding, check=name, status='pass', details=result)))
        print(json.dumps({'phase': args.phase, 'checks_passed': sorted(details), 'scope': SCOPE}), flush=True)
    finally:
        releases.close()
        # PID1 exiting tears down every real engine in this private namespace.


def cleaned_environment():
    return {key: value for key, value in os.environ.items() if
            key.lower() not in ('http_proxy', 'https_proxy', 'all_proxy', 'no_proxy') and
            not key.startswith(('ROVUNET_', 'CURL_', 'SSL_', 'GH_TOKEN', 'GITHUB_TOKEN'))}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('candidate', 'baseline', 'work', 'receipt'):
        parser.add_argument('--' + name, type=Path, required=name in ('baseline', 'work'))
    parser.add_argument('--baseline-smoke', action='store_true',
                        help='Test the harness using only signed2.0.2; never emit final2.1 acceptance')
    parser.add_argument('--inside', action='store_true', help=argparse.SUPPRESS)
    parser.add_argument('--phase', choices=('fresh', 'upgrade', 'smoke'), help=argparse.SUPPRESS)
    args = parser.parse_args()
    for name in ('candidate', 'baseline', 'work', 'receipt'):
        value = getattr(args, name)
        if value is not None:
            setattr(args, name, value.resolve())
    require(sys.platform == 'linux' and os.uname().machine == 'x86_64' and os.geteuid() == 0,
            'Use root on an authorized disposable Linux/amd64 acceptance runner')
    os.umask(0o077)
    if args.inside:
        require(args.phase is not None, 'Internal phase is required')
        return inside(args)
    require(args.phase is None, 'Invalid internal phase')
    if args.baseline_smoke:
        require(args.candidate is None and args.receipt is None,
                'Baseline smoke must not receive a candidate or final receipt path')
    else:
        require(args.candidate is not None and args.receipt is not None and not args.receipt.exists(),
                'Final candidate/receipt arguments are missing, or the receipt already exists')
    require(not args.work.exists() or not any(args.work.iterdir()), 'Acceptance work directory must be empty')
    if args.baseline_smoke:
        current = previous = authenticate(args.baseline, BASE_TAG, BASE_FILES)
    else:
        current, previous = checked_pair(args.candidate, args.baseline)
    args.work.mkdir(mode=0o700, parents=True, exist_ok=True)
    # Hosted runners commonly put both checkout and RUNNER_TEMP below /home.
    # Keep all executing input bytes outside the private mounts before masking it.
    with tempfile.TemporaryDirectory(prefix='rovunet-final-signed-', dir='/var/tmp') as temporary:
        stage = Path(temporary)
        inputs = [(args.baseline, BASE_FILES, 'baseline')]
        if not args.baseline_smoke:
            inputs.append((args.candidate, CANDIDATE_FILES, 'candidate'))
        for source, names, subdir in inputs:
            target = stage / subdir
            target.mkdir(mode=0o700)
            for name in names + ('RELEASE-MANIFEST.json', 'RELEASE-MANIFEST.sig'):
                shutil.copyfile(source / name, target / name)
        if args.baseline_smoke:
            authenticate(stage / 'baseline', BASE_TAG, BASE_FILES)
        else:
            checked_pair(stage / 'candidate', stage / 'baseline')
        for name in ('test-signed-v210-update.py', 'test-complete-setup.py'):
            shutil.copyfile(Path(__file__).with_name(name), stage / name)
        for phase in (('smoke',) if args.baseline_smoke else ('fresh', 'upgrade')):
            child_work = stage / phase
            env = cleaned_environment()
            env.update({'ROVUNET_SIGNED_PARENT_' + kind.upper(): os.readlink('/proc/self/ns/' + kind)
                        for kind in ('mnt', 'net', 'pid')})
            command = ['unshare', '--mount', '--net', '--pid', '--fork', '--kill-child', '--mount-proc',
                       sys.executable, str(stage / 'test-signed-v210-update.py'), '--inside', '--phase', phase,
                       '--baseline', str(stage / 'baseline'), '--work', str(child_work)]
            if args.baseline_smoke:
                command.append('--baseline-smoke')
            else:
                command += ['--candidate', str(stage / 'candidate'), '--receipt', str(stage / 'unused-child-receipt')]
            log_dir = args.work / 'logs'
            log_dir.mkdir(mode=0o700, exist_ok=True)
            print('Running production-signed acceptance phase: ' + phase, flush=True)
            with (log_dir / (phase + '-runner.log')).open('xb') as log:
                process = subprocess.Popen(command, env=env, stdin=subprocess.DEVNULL, stdout=log,
                                           stderr=subprocess.STDOUT, start_new_session=True)
                try:
                    code = process.wait(timeout=900)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait()
                    code = 124
            if (child_work / 'logs').exists():
                shutil.copytree(child_work / 'logs', log_dir / phase)
            if (child_work / 'evidence').exists():
                shutil.copytree(child_work / 'evidence', args.work / 'evidence', dirs_exist_ok=True)
            require(code == 0, 'Final signed ' + phase + ' acceptance failed; retained logs under ' + str(log_dir))
            if args.baseline_smoke:
                shutil.copyfile(child_work / 'BASELINE-SMOKE.json', args.work / 'BASELINE-SMOKE.json')
    if args.baseline_smoke:
        smoke = decode((args.work / 'BASELINE-SMOKE.json').read_bytes())
        require(smoke['status'] == 'pass' and smoke['final_v210_acceptance'] is False and smoke['tag'] == BASE_TAG and
                smoke['runner_sha256'] == digest(Path(__file__)), 'Invalid signed-baseline smoke evidence')
        print(json.dumps({'baseline_smoke': 'PASS', 'tag': BASE_TAG, 'final_v210_acceptance': False,
                          'summary_sha256': digest(args.work / 'BASELINE-SMOKE.json')}), flush=True)
        return
    checks = {}
    for name in CHECKS:
        path = args.work / 'evidence' / (name + '.json')
        proof = decode(path.read_bytes())
        require(proof['check'] == name and proof['status'] == 'pass' and proof['manifest_sha256'] == current['manifest_sha256'] and
                proof['runner_sha256'] == digest(Path(__file__)) and
                proof['simulator_sha256'] == digest(Path(__file__).with_name('test-complete-setup.py')),
                'Final phase evidence does not match this reviewed runner')
        checks[name] = {'status': 'pass', 'evidence_sha256': digest(path)}
    receipt = {'schema': 1, 'purpose': 'rovunet-final-signed-release-acceptance', 'tag': TAG,
               'source_commit': current['manifest']['source_commit'], 'manifest_sha256': current['manifest_sha256'],
               'production_key_id': KEY_ID, 'panel_sha256': {name: current['entries'][name]['sha256'] for name in PANELS},
               'runtime_base_tag': BASE_TAG, 'runtime_base_manifest_sha256': BASE_MANIFEST_SHA256,
               'environment': 'isolated-linux-namespaces', 'architecture': 'amd64',
               'production_signature_verified': True, 'upgrade_baseline': {'tag': BASE_TAG, 'source_commit': BASE_COMMIT},
               'checks': checks}
    private_write(args.receipt, canonical(receipt))
    print(json.dumps({'final_signed_acceptance': 'PASS', 'receipt_sha256': digest(args.receipt),
                      'environment': receipt['environment'], 'checks': sorted(checks)}), flush=True)


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        sys.exit('Final signed acceptance stopped: ' + str(error))
