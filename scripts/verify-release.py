#!/usr/bin/env python3
"""Verify the signed, exact release directory against compiled production roots."""
import argparse
import base64
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import struct
import tempfile

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('release_signing', ROOT/'scripts/sign-release.py')
signing = importlib.util.module_from_spec(spec)
spec.loader.exec_module(signing)


def strict_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('Duplicate manifest member')
        result[key] = value
    return result


def verify_elf(data, arch):
    machine = {'amd64': 62, 'arm64': 183}.get(arch)
    if len(data) < 64 or data[:6] != b'\x7fELF\x02\x01' or not machine:
        raise ValueError('Invalid release ELF header')
    kind, actual = struct.unpack_from('<HH', data, 16)
    if kind not in (2, 3) or actual != machine:
        raise ValueError('Release ELF architecture mismatch')
    offset = struct.unpack_from('<Q', data, 32)[0]
    stride, count = struct.unpack_from('<HH', data, 54)
    if stride < 56 or not count or offset + stride*count > len(data):
        raise ValueError('Invalid ELF program table')
    for i in range(count):
        p = offset + i*stride
        typ = struct.unpack_from('<I', data, p)[0]
        start, size = struct.unpack_from('<Q', data, p+8)[0], struct.unpack_from('<Q', data, p+32)[0]
        if typ == 1 and size > 0 and start <= len(data) and size <= len(data)-start:
            return
    raise ValueError('Release ELF has no valid load segment')


def verify(assets, tag, commit, ring, environment='production'):
    # environment/ring parameters are only used by the local ephemeral-key
    # test. The shipping CLI below has no test-key or downloaded-key option.
    if ring.get('schema') != 1 or not ring.get('keys'):
        raise ValueError('Production trust root has not been provisioned')
    raw = (assets/'RELEASE-MANIFEST.json').read_bytes()
    signature = (assets/'RELEASE-MANIFEST.sig').read_bytes()
    if len(raw) > 1 << 20 or len(signature) != 64:
        raise ValueError('Invalid manifest or signature size')
    manifest = json.loads(raw, object_pairs_hook=strict_object)
    if set(manifest) != {'schema','environment','key_id','repository','tag','source_commit','assets'}:
        raise ValueError('Invalid manifest fields')
    if manifest['schema'] != 1 or manifest['environment'] != environment or manifest['repository'] != 'msVPNco/rovunet-panel':
        raise ValueError('Wrong manifest authority or repository')
    if manifest['tag'] != tag or manifest['source_commit'] != commit:
        raise ValueError('Wrong release tag or source commit')
    key = next((k for k in ring['keys'] if k['id'] == manifest['key_id'] and k['purpose'] == environment), None)
    if not key:
        raise ValueError('Manifest signing key is not trusted for this purpose')
    der = base64.b64decode(key['spki'], validate=True)
    if hashlib.sha256(der).hexdigest() != key['id'] or len(der) != 44 or der[:12] != bytes.fromhex('302a300506032b6570032100'):
        raise ValueError('Invalid trust root identity')
    with tempfile.TemporaryDirectory(prefix='rovunet-verify-') as scratch:
        public = Path(scratch)/'public.der'
        public.write_bytes(der)
        signing.openssl('pkeyutl', '-verify', '-rawin', '-pubin', '-keyform', 'DER',
                        '-inkey', public, '-in', assets/'RELEASE-MANIFEST.json',
                        '-sigfile', assets/'RELEASE-MANIFEST.sig')
    entries = manifest['assets']
    if not isinstance(entries, list) or not 1 <= len(entries) <= 256:
        raise ValueError('Invalid asset count')
    names = set()
    for entry in entries:
        if set(entry) != {'name','os','arch','kind','size','sha256','source','source_commit','license'}:
            raise ValueError('Invalid asset fields')
        name = entry['name']
        if not isinstance(name, str) or not re.fullmatch('[A-Za-z0-9][A-Za-z0-9._-]{0,159}', name) or name in names:
            raise ValueError('Invalid or duplicate asset name')
        names.add(name)
        p = assets/name
        if p.is_symlink() or not p.is_file() or not isinstance(entry['size'], int) or not 0 < entry['size'] <= 512 << 20:
            raise ValueError('Invalid release asset')
        data = p.read_bytes()
        if len(data) != entry['size'] or hashlib.sha256(data).hexdigest() != entry['sha256']:
            raise ValueError('Release asset digest/size mismatch')
        expected = signing.metadata(name, commit)
        if any(entry[k] != expected[k] for k in expected):
            raise ValueError('Release asset platform/provenance metadata mismatch')
        if entry['kind'] == 'executable':
            verify_elf(data, entry['arch'])
    if {p.name for p in assets.iterdir()} != names | {'RELEASE-MANIFEST.json','RELEASE-MANIFEST.sig'}:
        raise ValueError('Unsigned extra files in release directory')
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--assets', required=True, type=Path)
    parser.add_argument('--tag', required=True)
    parser.add_argument('--commit', required=True)
    args = parser.parse_args()
    try:
        ring = json.loads((ROOT/'internal/releaseauth/production-keys.json').read_text())
        verify(args.assets, args.tag, args.commit, ring)
    except (ValueError, OSError, KeyError, TypeError) as exc:
        parser.exit(1, f'Release verification stopped: {exc}\n')
    print('All release assets authenticated against the compiled production trust root.')


if __name__ == '__main__':
    main()
