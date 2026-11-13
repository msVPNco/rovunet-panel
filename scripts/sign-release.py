#!/usr/bin/env python3
"""Sign a complete release with a separately provisioned Ed25519 identity.

No private key or generated test authority is committed. Production signing
fails unless the key matches BOTH reviewed bootstrap and panel trust anchors.
"""
import argparse
import base64
import hashlib
import json
import os
import pathlib
import re
import shutil
import stat
import subprocess
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
XRAY = '63ca2e0c1d31b89f67700c33842431f541fedd25'
AWG_GO = 'b5928efb6ca19f0153958460c3d141f04abc5c2e'
AWG_TOOLS = 'ee0f0a9aa34ff0a0da4b3433b9512781cfe02843'


def openssl(*args):
    result = subprocess.run(['openssl', *map(str, args)], capture_output=True)
    if result.returncode:
        raise ValueError('OpenSSL key/signature operation failed; private data was not logged')
    return result.stdout


def key_identity(key):
    info = key.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_mode & 0o077 or info.st_uid != os.geteuid():
        raise ValueError('Signing key must be an owner-only regular file owned by the signer')
    der = openssl('pkey', '-in', key, '-pubout', '-outform', 'DER')
    if len(der) != 44 or not der.startswith(bytes.fromhex('302a300506032b6570032100')):
        raise ValueError('Signing identity must be Ed25519')
    return {'id': hashlib.sha256(der).hexdigest(), 'spki': base64.b64encode(der).decode()}


def metadata(name, commit):
    source, revision, license = 'https://github.com/msVPNco/rovunet-panel', commit, 'AGPL-3.0'
    if name.startswith(('xray-', 'LICENSE-xray')):
        source, revision, license = 'https://github.com/XTLS/Xray-core', XRAY, 'MPL-2.0'
    if name.startswith(('amneziawg-go-', 'LICENSE-amneziawg-go')):
        source, revision, license = 'https://github.com/amnezia-vpn/amneziawg-go', AWG_GO, 'MIT'
    if name.startswith(('awg-linux-', 'amneziawg-tools-', 'LICENSE-amneziawg-tools')):
        source, revision, license = 'https://github.com/amnezia-vpn/amneziawg-tools', AWG_TOOLS, 'GPL-2.0'
    if name in ('PANEL-THIRD-PARTY-LICENSES.tar.gz', 'REVIEW-MATERIALS.tar.gz') or \
            (name.startswith('RUNTIME-CANDIDATES-') and name.endswith('.tar.gz')):
        # Aggregate data archives retain component origins/terms inside them;
        # they are not flat engine assets using the shipping engine pins.
        license = 'Multiple; see included license texts'
    arch = next((a for a in ('amd64', 'arm64') if re.search(r'-linux-' + a + r'$', name)), '')
    kind = 'executable' if arch else ('installer' if name == 'install.sh' else 'data')
    return dict(os='linux' if arch or kind == 'installer' else '', arch=arch, kind=kind,
                source=source, source_commit=revision, license=license)


def windows_path(path):
    if os.name == 'nt':
        return str(path.resolve())
    result = subprocess.run(['wslpath', '-w', str(path.resolve())], capture_output=True, text=True, check=True)
    return result.stdout.strip()


class WindowsVaultSigner:
    """Public metadata only; the private key stays inside current-user DPAPI."""
    def __init__(self, vault, tool, public_file, powershell=None):
        self.vault, self.tool = vault, tool
        self.powershell = powershell
        public = json.loads(public_file.read_text())
        self.identity = {k: public[k] for k in ('id', 'spki')}
        der = base64.b64decode(self.identity['spki'], validate=True)
        if public.get('schema') != 1 or public.get('purpose') != 'production' or \
                len(der) != 44 or not der.startswith(bytes.fromhex('302a300506032b6570032100')) or \
                hashlib.sha256(der).hexdigest() != self.identity['id']:
            raise ValueError('Invalid production vault public identity')

    def sign(self, raw, signature, tag):
        # argv contains paths and the public release tag; never private bytes.
        runner = self.powershell or shutil.which('pwsh.exe')
        if not runner and os.name != 'nt':
            candidate = pathlib.Path('/mnt/c/Program Files/PowerShell/7/pwsh.exe')
            if candidate.is_file():
                runner = str(candidate)
        if not runner:
            raise ValueError('PowerShell 7 is required for the reviewed Windows vault wrapper')
        result = subprocess.run([
            runner, '-NoProfile', '-NonInteractive', '-File',
            windows_path(ROOT/'scripts/panel-signing-vault.ps1'),
            '-Action', 'sign', '-Tool', self.tool, '-Vault', self.vault,
            '-Manifest', windows_path(raw), '-Signature', windows_path(signature), '-Tag', tag,
        ], capture_output=True)
        if result.returncode:
            raise ValueError('Windows vault refused signing; no private data was logged')
        return signature.read_bytes()


def sign(assets, key, tag, commit, test=False, external=None):
    if not re.fullmatch(r'v\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?', tag) or not re.fullmatch('[0-9a-f]{40}', commit):
        raise ValueError('An exact release tag and source commit are required')
    if external and test:
        raise ValueError('Production Windows vault cannot sign a test release')
    identity = external.identity if external else key_identity(key)
    environment = 'test' if test else 'production'
    if not test:
        ring = json.loads((ROOT/'internal/releaseauth/production-keys.json').read_text())
        bootstrap = (ROOT/'install.sh').read_text()
        literal = re.search(r"^TRUST_ROOT_JSON='([^']*)'$", bootstrap, re.M)
        if not literal or json.loads(literal[1]) != ring:
            raise ValueError('Bootstrap and panel production trust anchors differ')
        if not any(k == dict(identity, purpose='production') for k in ring['keys']):
            raise ValueError('Signing identity has not been independently provisioned as a production trust anchor')
        if not (assets/'install.sh').is_file() or (assets/'install.sh').read_bytes() != (ROOT/'install.sh').read_bytes():
            raise ValueError('The exact reviewed installer must be included in the signed release')
    entries = []
    for p in sorted(assets.iterdir()):
        if p.name in ('RELEASE-MANIFEST.json', 'RELEASE-MANIFEST.sig'):
            continue
        if not p.is_file() or p.is_symlink() or not re.fullmatch('[A-Za-z0-9][A-Za-z0-9._-]{0,159}', p.name):
            raise ValueError('Release assets must be regular files with plain names')
        if not 0 < p.stat().st_size <= 512 << 20:
            raise ValueError('Empty or oversized release artifact')
        entries.append(dict(name=p.name, size=p.stat().st_size,
                            sha256=hashlib.sha256(p.read_bytes()).hexdigest(), **metadata(p.name, commit)))
    if not entries or len(entries) > 256:
        raise ValueError('Invalid release asset count')
    manifest = dict(schema=1, environment=environment, key_id=identity['id'],
                    repository='msVPNco/rovunet-panel', tag=tag, source_commit=commit, assets=entries)
    body = (json.dumps(manifest, sort_keys=True, separators=(',', ':'))+'\n').encode()
    # Windows can access the public staging files on the release asset volume.
    with tempfile.TemporaryDirectory(prefix='rovunet-sign-', dir=assets.parent if external else None) as scratch:
        raw = pathlib.Path(scratch)/'manifest.json'
        raw.write_bytes(body)
        signed = pathlib.Path(scratch)/'manifest.sig'
        sig = external.sign(raw, signed, tag) if external else openssl('pkeyutl', '-sign', '-rawin', '-inkey', key, '-in', raw)
        if len(sig) != 64:
            raise ValueError('Invalid Ed25519 signature length')
        public = pathlib.Path(scratch)/'public.der'
        public.write_bytes(base64.b64decode(identity['spki']))
        signed.write_bytes(sig)
        openssl('pkeyutl', '-verify', '-rawin', '-pubin', '-keyform', 'DER',
                '-inkey', public, '-in', raw, '-sigfile', signed)
        (assets/'RELEASE-MANIFEST.json').write_bytes(body)
        (assets/'RELEASE-MANIFEST.sig').write_bytes(sig)
    return dict(identity, purpose=environment)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--assets', required=True, type=pathlib.Path)
    keys = parser.add_mutually_exclusive_group(required=True)
    keys.add_argument('--private-key-file', type=pathlib.Path)
    keys.add_argument('--windows-vault', help='Protected Windows vault path; private bytes are never exported')
    parser.add_argument('--vault-tool', help='Reviewed Windows signing-vault executable path')
    parser.add_argument('--identity-public-file', type=pathlib.Path, help='Public vault metadata only')
    parser.add_argument('--powershell-executable', help='PowerShell 7 executable, for WSL without Windows PATH inheritance')
    parser.add_argument('--tag', required=True)
    parser.add_argument('--commit', required=True)
    parser.add_argument('--test-key', action='store_true', help='Sign a TEST manifest that production rejects')
    args = parser.parse_args()
    try:
        external = None
        if args.windows_vault:
            if not args.vault_tool or not args.identity_public_file:
                raise ValueError('Windows vault requires the reviewed tool and public metadata file')
            external = WindowsVaultSigner(args.windows_vault, args.vault_tool, args.identity_public_file, args.powershell_executable)
        elif args.vault_tool or args.identity_public_file or args.powershell_executable:
            raise ValueError('Vault options require --windows-vault')
        sign(args.assets, args.private_key_file, args.tag, args.commit, args.test_key, external)
    except (ValueError, OSError, KeyError, subprocess.CalledProcessError) as exc:
        parser.exit(1, f'Release signing stopped: {exc}\n')
    print('Release manifest signed ('+('TEST ONLY' if args.test_key else 'production')+').')


if __name__ == '__main__':
    main()
