#!/usr/bin/env python3
"""Add missing, authenticated v2.0.2 runtimes for one identified preview panel.

No panel upgrade, configuration changes, engine execution or complete-setup.
Publication is individually atomic, not an all-or-nothing multi-file transaction:
after interruption, rerunning retains verified files and resumes missing ones.
"""
import base64
import fcntl
import hashlib
import json
import os
import pathlib
import platform
import re
import shutil
import stat
import struct
import subprocess
import sys
import tempfile
import time

BIN = pathlib.Path('/usr/local/bin')
RECEIPTS = pathlib.Path('/var/lib/rovunet-runtime-repair')
PANEL_SHA256 = '26a796b3891c662e8c8d788f2430e7391f0cc4012cca77343251344b49b1640d'
MANIFEST_SHA256 = '38dcbe40474787132f540d7804388c3f0e51f50a58ac9aaf765cf167a30e0f78'
SIGNATURE_SHA256 = '690db4e0fa801f4bb420b00b242597344908231ec3d9079322bd950bdefa3894'
KEY_ID = '885d8aba22365ce42e20527bfa00e82aed3cba2f69501f146ed0496a8bc3dcbe'
SPKI = 'MCowBQYDK2VwAyEAMfWBf2jAnb20LJvMoOfN1R/4OP0tSjPec+GWU+QXdqs='
REPOSITORY = 'msVPNco/rovunet-panel'
TAG = 'v2.0.2'
SOURCE = 'efa697256880b0d2c0b1494513b51f4c72a7735b'
BASE = 'https://github.com/' + REPOSITORY + '/releases/download/' + TAG + '/'
TARGETS = {
    'xray': 'xray-linux-amd64',
    'rovunet-xray-managed': 'xray-linux-amd64',
    'rovunet-sing-box': 'sing-box-linux-amd64',
    'rovunet-mita': 'mita-linux-amd64',
    'rovunet-amneziawg-go': 'amneziawg-go-linux-amd64',
    'rovunet-awg': 'awg-linux-amd64',
    'rovunet-awg-provenance.json': 'awg-provenance-linux-amd64.json',
}
MAX_METADATA = 1024 * 1024
MAX_ASSET = 512 * 1024 * 1024


class RepairError(RuntimeError):
    pass


def require(ok, message):
    if not ok:
        raise RepairError(message)


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def unique(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, 'Duplicate manifest member')
        result[key] = value
    return result


def download(name, destination, limit):
    # -q suppresses curlrc; redirects remain HTTPS and normal TLS verification
    # stays enabled. The streaming cap also covers unknown/chunked lengths on
    # curl versions where --max-filesize only checks Content-Length.
    command = ['/usr/bin/curl', '-q', '--fail', '--location', '--silent',
               '--show-error', '--proto', '=https', '--proto-redir', '=https',
               '--tlsv1.2', '--connect-timeout', '30', '--max-time', '300',
               '--max-filesize', str(limit), BASE + name]
    process = subprocess.Popen(command, stdout=subprocess.PIPE,
                               stderr=subprocess.DEVNULL)
    try:
        total = 0
        with destination.open('xb') as output:
            while True:
                chunk = process.stdout.read(min(256 * 1024, limit + 1 - total))
                if not chunk:
                    break
                total += len(chunk)
                require(total <= limit, 'Download exceeded bound: ' + name)
                output.write(chunk)
        code = process.wait()
        require(code == 0, 'HTTPS download failed: {} (curl exit {})'.format(name, code))
    finally:
        if process.poll() is None:
            process.kill()
            process.wait()
        process.stdout.close()


def authenticated_assets(stage, fetch):
    manifest, signature = stage / 'manifest.json', stage / 'manifest.sig'
    fetch('RELEASE-MANIFEST.json', manifest, MAX_METADATA)
    fetch('RELEASE-MANIFEST.sig', signature, 64)
    require(0 < manifest.stat().st_size <= MAX_METADATA and
            signature.stat().st_size == 64, 'Invalid release metadata sizes')
    raw, sig = manifest.read_bytes(), signature.read_bytes()
    require(sha256(raw) == MANIFEST_SHA256 and sha256(sig) == SIGNATURE_SHA256,
            'Release metadata differs from the reviewed v2.0.2 bytes')
    der = base64.b64decode(SPKI, validate=True)
    require(der[:12] == bytes.fromhex('302a300506032b6570032100') and
            len(der) == 44 and sha256(der) == KEY_ID, 'Invalid production anchor')
    anchor = stage / 'anchor.der'
    anchor.write_bytes(der)
    check = subprocess.run(['/usr/bin/openssl', 'pkeyutl', '-verify', '-rawin',
                            '-pubin', '-keyform', 'DER', '-inkey', str(anchor),
                            '-in', str(manifest), '-sigfile', str(signature)],
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    require(check.returncode == 0, 'Release Ed25519 signature is invalid')
    document = json.loads(raw, object_pairs_hook=unique)
    fields = {'schema', 'environment', 'key_id', 'repository', 'tag', 'source_commit', 'assets'}
    require(isinstance(document, dict) and set(document) == fields and
            type(document['schema']) is int and document['schema'] == 1 and
            (document['environment'], document['key_id'], document['repository'],
             document['tag'], document['source_commit']) ==
            ('production', KEY_ID, REPOSITORY, TAG, SOURCE), 'Release identity mismatch')
    require(isinstance(document['assets'], list) and 0 < len(document['assets']) <= 256,
            'Invalid release asset list')
    assets = {}
    fields = {'name', 'os', 'arch', 'kind', 'size', 'sha256', 'source', 'source_commit', 'license'}
    for asset in document['assets']:
        require(isinstance(asset, dict) and set(asset) == fields,
                'Invalid release asset fields')
        require(all(isinstance(asset[key], str) for key in fields - {'size'}) and
                re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]{0,159}', asset['name']) and
                asset['name'] not in assets and type(asset['size']) is int and
                0 < asset['size'] <= MAX_ASSET and
                re.fullmatch(r'[0-9a-f]{64}', asset['sha256']) and
                all(asset[key] for key in ('source', 'source_commit', 'license')),
                'Incomplete or duplicate release asset')
        assets[asset['name']] = asset
    for name in set(TARGETS.values()):
        require(name in assets, 'Required runtime absent from signed manifest: ' + name)
        asset = assets[name]
        expected = ('data', '', '') if name.endswith('.json') else ('executable', 'linux', 'amd64')
        require((asset['kind'], asset['os'], asset['arch']) == expected,
                'Runtime architecture or kind mismatch: ' + name)
    return assets


def directory(path, owner, private=False):
    info = path.lstat()
    require(stat.S_ISDIR(info.st_mode) and info.st_uid == owner and
            info.st_mode & (0o077 if private else 0o022) == 0,
            'Unsafe directory: ' + str(path))


def identity(info):
    return (info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns,
            info.st_ctime_ns, info.st_mode, info.st_uid)


def verified_file(path, asset, owner, installed=False):
    """Return False only for absence; never follow a link or accept changed bytes."""
    try:
        before = path.lstat()
    except FileNotFoundError:
        return False
    require(stat.S_ISREG(before.st_mode) and before.st_uid == owner and
            before.st_mode & 0o022 == 0, 'Unsafe existing file: ' + str(path))
    if installed and asset['kind'] == 'executable':
        require(before.st_mode & stat.S_IXUSR, 'Existing executable lacks owner execute permission: ' + str(path))
    fd = os.open(str(path), os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, 'rb') as source:
        require(identity(os.fstat(source.fileno())) == identity(before), 'File changed: ' + str(path))
        require(0 < before.st_size <= MAX_ASSET and
                (asset.get('size') is None or before.st_size == asset['size']),
                'Runtime size mismatch: ' + str(path))
        header = source.read(64)
        digest = hashlib.sha256(header)
        for chunk in iter(lambda: source.read(256 * 1024), b''):
            digest.update(chunk)
        require(digest.hexdigest() == asset['sha256'], 'Runtime digest mismatch: ' + str(path))
        if asset['kind'] == 'executable':
            require(len(header) == 64 and header[:7] == b'\x7fELF\x02\x01\x01' and
                    struct.unpack_from('<H', header, 16)[0] in (2, 3) and
                    struct.unpack_from('<H', header, 18)[0] == 62 and
                    struct.unpack_from('<I', header, 20)[0] == 1 and
                    struct.unpack_from('<H', header, 52)[0] == 64,
                    'Expected an ELF64 Linux amd64 executable: ' + str(path))
        require(identity(os.fstat(source.fileno())) == identity(before) and
                identity(path.lstat()) == identity(before), 'File changed: ' + str(path))
    return True


def sync_directory(path):
    fd = os.open(str(path), os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def write_receipt(path, receipt):
    fd, name = tempfile.mkstemp(prefix='.receipt-', dir=str(path.parent))
    temporary = pathlib.Path(name)
    try:
        with os.fdopen(fd, 'w') as output:
            json.dump(receipt, output, sort_keys=True, indent=2)
            output.write('\n')
            output.flush()
            os.fsync(output.fileno())
        os.replace(str(temporary), str(path))
        sync_directory(path.parent)
    finally:
        if temporary.exists():
            temporary.unlink()


def repair(bin_dir, receipt_dir, owner=0, fetch=download):
    """Internal fixture seam; the CLI has no path, identity or download overrides."""
    directory(bin_dir, owner)
    panel = {'kind': 'executable', 'sha256': PANEL_SHA256}
    require(verified_file(bin_dir / 'rovunet', panel, owner, True), 'Expected installed panel is missing')
    receipt_dir.mkdir(mode=0o700, exist_ok=True)
    directory(receipt_dir, owner, private=True)
    lock_fd = os.open(str(receipt_dir / 'repair.lock'), os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        lock = os.fstat(lock_fd)
        require(stat.S_ISREG(lock.st_mode) and lock.st_uid == owner and
                lock.st_mode & 0o077 == 0 and lock.st_nlink == 1, 'Unsafe repair lock')
        fcntl.flock(lock_fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        with tempfile.TemporaryDirectory(prefix='.rovunet-runtime-repair-', dir=str(bin_dir)) as temporary:
            stage = pathlib.Path(temporary)
            print('Authenticating the pinned v2.0.2 release manifest and signature...', flush=True)
            assets = authenticated_assets(stage, fetch)
            missing, retained = [], []
            # Check every destination before downloading or publishing runtimes.
            for target, name in TARGETS.items():
                (retained if verified_file(bin_dir / target, assets[name], owner, True)
                 else missing).append(target)
            downloads = set(TARGETS[target] for target in missing)
            required = sum(assets[name]['size'] for name in downloads)
            required += sum(assets[TARGETS[target]]['size'] for target in missing) + 50 * 1024 * 1024
            require(shutil.disk_usage(bin_dir).free >= required, 'Insufficient free disk space for verified staging')
            for name in sorted(downloads):
                print('Downloading and verifying {} ({} bytes)...'.format(name, assets[name]['size']), flush=True)
                fetch(name, stage / name, assets[name]['size'])
                require(verified_file(stage / name, assets[name], owner), 'Downloaded runtime is missing')
            # Separate per-target copies: the two final Xray paths never share
            # an inode merely because they came from the same release asset.
            copies = stage / 'copies'
            copies.mkdir(mode=0o700)
            for target in missing:
                asset = assets[TARGETS[target]]
                candidate = copies / target
                with (stage / TARGETS[target]).open('rb') as source, candidate.open('xb') as output:
                    shutil.copyfileobj(source, output, 256 * 1024)
                    output.flush()
                    os.fchmod(output.fileno(), 0o644 if asset['kind'] == 'data' else 0o755)
                    os.fsync(output.fileno())
                require(verified_file(candidate, asset, owner, True), 'Staged runtime is missing')
            # A collision discovered now must still stop before the first link.
            for target, name in TARGETS.items():
                verified_file(bin_dir / target, assets[name], owner, True)
            fd, receipt_name = tempfile.mkstemp(prefix='repair-', suffix='.json', dir=str(receipt_dir))
            os.close(fd)
            receipt_path = pathlib.Path(receipt_name)
            receipt = {'schema': 1, 'started_at': int(time.time()), 'status': 'prepared',
                       'panel_sha256': PANEL_SHA256, 'tag': TAG, 'source_commit': SOURCE,
                       'manifest_sha256': MANIFEST_SHA256, 'signature_sha256': SIGNATURE_SHA256,
                       'installed': [], 'retained': retained, 'assets': {
                           target: {'asset': name, 'sha256': assets[name]['sha256'], 'size': assets[name]['size']}
                           for target, name in TARGETS.items()}}
            write_receipt(receipt_path, receipt)
            try:
                print('Publishing {} missing runtime file(s) without replacing existing files...'.format(len(missing)), flush=True)
                for target in missing:
                    require(verified_file(bin_dir / 'rovunet', panel, owner, True), 'Installed panel disappeared')
                    asset = assets[TARGETS[target]]
                    if verified_file(bin_dir / target, asset, owner, True):
                        receipt['retained'].append(target)
                        continue
                    try:
                        os.link(str(copies / target), str(bin_dir / target), follow_symlinks=False)
                    except FileExistsError:
                        require(verified_file(bin_dir / target, asset, owner, True), 'Destination collision disappeared')
                        receipt['retained'].append(target)
                    else:
                        receipt['installed'].append(target)
                        sync_directory(bin_dir)
                        require(verified_file(bin_dir / target, asset, owner, True), 'Published runtime disappeared')
                    receipt['status'] = 'in-progress'
                    write_receipt(receipt_path, receipt)
                for target, name in TARGETS.items():
                    require(verified_file(bin_dir / target, assets[name], owner, True), 'Runtime disappeared: ' + target)
                require(verified_file(bin_dir / 'rovunet', panel, owner, True), 'Installed panel disappeared')
                receipt['status'] = 'ready'
                write_receipt(receipt_path, receipt)
            except BaseException as error:
                receipt['status'], receipt['error'] = 'failed', str(error)
                write_receipt(receipt_path, receipt)
                raise
            return receipt_path
    finally:
        os.close(lock_fd)


def main():
    require(not sys.argv[1:], 'Run without arguments; this repair has no override flags')
    require(os.geteuid() == 0, 'Run as root: python3 repair-preview-runtimes.py')
    require(platform.system() == 'Linux' and platform.machine() == 'x86_64',
            'This guarded repair supports only the identified Linux x86_64 panel')
    os.umask(0o077)
    for path in ('/', '/usr', '/usr/local', '/usr/local/bin', '/var', '/var/lib'):
        directory(pathlib.Path(path), 0)
    receipt = repair(BIN, RECEIPTS)
    print('READY: all seven official v2.0.2 runtime files are verified. Receipt: ' + str(receipt))
    print('The installed panel and configuration were not replaced. complete-setup was not run.')


if __name__ == '__main__':
    try:
        main()
    except (Exception, KeyboardInterrupt) as error:
        print('FAILED: ' + str(error), file=sys.stderr)
        print('This is a resumable repair, not a multi-file rollback. Existing files were not overwritten; '
              'any files added before interruption contain verified bytes. Do not run complete-setup '
              'until a rerun reports READY.', file=sys.stderr)
        sys.exit(1)
