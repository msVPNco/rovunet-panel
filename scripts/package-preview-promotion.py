#!/usr/bin/env python3
"""Package the accepted preview.3 -> public release panel-only upgrade."""
import argparse
import hashlib
import json
import pathlib
import re
import shutil
import subprocess
import tarfile
import tempfile

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--panel', type=pathlib.Path, required=True)
parser.add_argument('--commit', required=True)
parser.add_argument('--version', required=True)
parser.add_argument('--output', type=pathlib.Path, required=True)
args = parser.parse_args()
if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', args.version) or not re.fullmatch(r'[0-9a-f]{40}', args.commit):
    raise SystemExit('Expected a stable version and a full source commit.')
expected_version = 'rovunet v' + args.version + ' (' + args.commit[:7] + ')'
if subprocess.check_output([str(args.panel.resolve()), '--version'], stderr=subprocess.STDOUT, text=True).strip() != expected_version:
    raise SystemExit('Panel version does not match the release source.')
source = pathlib.Path(__file__).resolve().parents[1]
args.output.mkdir(parents=True, exist_ok=True)


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


with tempfile.TemporaryDirectory(prefix='rovunet-promotion.') as temporary:
    package = pathlib.Path(temporary)
    shutil.copyfile(args.panel, package / 'rovunet-linux-amd64')
    (package / 'rovunet-linux-amd64').chmod(0o755)
    script = (source / 'scripts/upgrade-managed-preview.py').read_text()
    replacements = {
        "PREVIOUS_VERSION = 'rovunet v1.20.1-managed-preview.2 (037e829)'": "PREVIOUS_VERSION = 'rovunet v1.20.1-managed-preview.3 (2ecfe96)'",
        "PREVIOUS_SHA256 = '1999b9ab312f4008ff2bb9697794bb1947d1c9d15d8748b8aec929cd3fa6c2fd'": "PREVIOUS_SHA256 = '68f0c4d18a61db2f73d5120000be5557b42394e5ed4a60221c303460ae56746b'",
        "TARGET_VERSION = '1.20.1-managed-preview.3'": "TARGET_VERSION = '" + args.version + "'",
    }
    for old, new in replacements.items():
        if script.count(old) != 1:
            raise SystemExit('Upgrade template baseline changed; inspect before packaging.')
        script = script.replace(old, new)
    (package / 'upgrade-managed-preview.py').write_text(script)
    shutil.copyfile(source / 'internal/clientcfg/display_names.json', package / 'profile-display-names.json')
    shutil.copyfile(source / 'scripts/test-managed-preview-upgrade.py', package / 'test-managed-preview-upgrade.py')
    shutil.copyfile(source / 'LICENSE', package / 'LICENSE-panel.txt')
    cache = pathlib.Path(subprocess.check_output(['go', 'env', 'GOMODCACHE'], text=True).strip())
    goroot = pathlib.Path(subprocess.check_output(['go', 'env', 'GOROOT'], text=True).strip())
    shutil.copyfile(goroot / 'LICENSE', package / 'LICENSE-go.txt')
    # Build metadata is the exact list of libraries in this panel executable.
    for line in subprocess.check_output(['go', 'version', '-m', str(args.panel)], text=True).splitlines():
        fields = line.split()
        if not fields or fields[0] != 'dep':
            continue
        module, version = fields[1:3]
        escaped = ''.join('!' + c.lower() if c.isupper() else c for c in module)
        root = cache / (escaped + '@' + version)
        for path in root.iterdir():
            if path.is_file() and path.name.lower() in ('license', 'license.txt', 'license.md', 'copying', 'notice', 'notice.txt'):
                target = package / 'licenses' / (escaped + '@' + version) / path.name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(path, target)
    manifest = {'version': args.version, 'commit': args.commit, 'previous_version': '1.20.1-managed-preview.3',
                'previous_sha256': '68f0c4d18a61db2f73d5120000be5557b42394e5ed4a60221c303460ae56746b',
                'panel_sha256': digest(args.panel), 'display_names_only': True,
                'approved_windows_core_additions': ['managed-awg', 'managed-ss2022'],
                'approved_standard_core_encodings': ['managed-awg', 'managed-ss2022'],
                'source_url': 'https://github.com/msVPNco/rovunet-panel/tree/' + args.commit}
    (package / 'UPGRADE-MANIFEST.json').write_text(json.dumps(manifest, indent=2) + '\n')
    (package / 'START.txt').write_text('''Accepted preview.3 -> public release, Linux amd64 only.
Keep the original private preview backup. Extract this archive into a separate
private directory, then run as root:
  python3 upgrade-managed-preview.py apply
  python3 upgrade-managed-preview.py status
The installer verifies its files, takes a private validated backup, preserves
accounts/keys/URLs, compares all profile fields and does not restart network
services. Two timers are briefly paused and restored. Do not run concurrent
account/core administration. A repeated apply recovers an interrupted journal.
Optional panel-only revert: python3 upgrade-managed-preview.py revert
This restores preview.3, not the original stable panel, and keeps all cores.
Source: see UPGRADE-MANIFEST.json and the release's source-code archive.
''')
    files = sorted(path for path in package.rglob('*') if path.is_file())
    (package / 'SHA256SUMS').write_text(''.join(digest(path) + '  ' + path.relative_to(package).as_posix() + '\n' for path in files))
    target = args.output / ('rovunet-preview-to-v' + args.version + '-linux-amd64.tar.gz')
    with tarfile.open(target, 'w:gz') as archive:
        for path in sorted(package.rglob('*')):
            if path.is_file():
                archive.add(path, arcname=path.relative_to(package).as_posix(), recursive=False)
    print(target.name)
