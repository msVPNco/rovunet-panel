#!/usr/bin/env python3
"""Publish only the immutable, locally production-signed v1.21.2 release."""
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
from concurrent.futures import ThreadPoolExecutor

REPOSITORY = 'msVPNco/rovunet-panel'
TAG = 'v1.21.2'
COMMIT = 'ac23d1a3ed07009567e004415606da7fed8779d3'
RELEASE_ID = 444066910
DRAFT_PLACEHOLDER = 'untagged-c25a546969202ccba5e2'
HANDOFF = Path(__file__).resolve().parent
SOURCE = Path.cwd() / 'source'
FINAL_FILES = (
    'ACCEPTANCE.json', 'SHA256SUMS',
    'ROVUNET-PANEL-1.21.2-INTEGRATION-FA.txt',
    'ROVUNET-PANEL-1.21.2-INTEGRATION.json',
    'RELEASE-MANIFEST.json', 'RELEASE-MANIFEST.sig',
)


def execute(*args, cwd=None):
    return subprocess.check_output(args, cwd=cwd, text=True)


def api(endpoint, payload=None):
    command = ['gh', 'api', '-H', 'Accept: application/vnd.github+json', endpoint]
    if payload is not None:
        command += ['--method', 'PATCH', '--input', '-']
    result = subprocess.run(
        command, input=json.dumps(payload) if payload is not None else None,
        text=True, stdout=subprocess.PIPE, check=True,
    )
    return json.loads(result.stdout)


def get_release():
    # Draft updates can expose GitHub's temporary untagged name until publish.
    # Pin the exact reviewed draft ID; never select a different release by name.
    release = api(f'repos/{REPOSITORY}/releases/{RELEASE_ID}')
    assert release['id'] == RELEASE_ID
    allowed = {TAG, DRAFT_PLACEHOLDER} if release['draft'] else {TAG}
    assert release['tag_name'] in allowed, 'Unexpected release tag'
    assert release['name'] == 'Rovunet Panel ' + TAG
    assert release['target_commitish'] == COMMIT, 'Unexpected source commit'
    assert release['prerelease'] is False, 'Unexpected prerelease'
    names = [asset['name'] for asset in release['assets']]
    assert len(names) == len(set(names)), 'Duplicate release asset names'
    return release


def download_assets(destination):
    destination.mkdir()
    release = get_release()
    def download(asset):
        name = asset['name']
        assert re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]{0,159}', name)
        with (destination / name).open('xb') as output:
            subprocess.run([
                'gh', 'api', '-H', 'Accept: application/octet-stream',
                f'repos/{REPOSITORY}/releases/assets/{asset["id"]}',
            ], stdout=output, check=True)
    with ThreadPoolExecutor(max_workers=4) as pool:
        list(pool.map(download, release['assets']))


def authenticate_assets(destination):
    output = execute(
        'python3', str(SOURCE / 'scripts/verify-release.py'),
        '--assets', str(destination), '--tag', TAG, '--commit', COMMIT,
    )
    print(output.strip(), flush=True)
    assert len(list(destination.iterdir())) == 34, 'Expected exactly 34 assets'
    execute('sha256sum', '--check', 'SHA256SUMS', cwd=destination)


def check_remote_metadata(release, directory):
    local = {path.name: path for path in directory.iterdir()}
    assert set(local) == {asset['name'] for asset in release['assets']}
    for asset in release['assets']:
        data = local[asset['name']].read_bytes()
        assert asset['state'] == 'uploaded', 'Incomplete remote upload'
        assert len(data) == asset['size'], 'Remote asset size mismatch'
        digest = asset.get('digest')
        if digest:
            assert digest == 'sha256:' + hashlib.sha256(data).hexdigest(), \
                'Remote asset digest mismatch'


def main():
    assert os.environ.get('GITHUB_REPOSITORY') == REPOSITORY
    assert os.environ.get('GITHUB_REF') == 'refs/heads/codex/publish-signed-v1.21.2'
    assert execute('git', '-C', str(SOURCE), 'rev-parse', 'HEAD').strip() == COMMIT
    release = get_release()
    with tempfile.TemporaryDirectory(prefix='rovunet-signed-publication-') as scratch:
        scratch = Path(scratch)
        staged = scratch / 'staged'
        download_assets(staged)
        missing = []
        for name in FINAL_FILES:
            if name.endswith('.sig'):
                encoded = (HANDOFF / (name + '.base64')).read_text().strip()
                data = base64.b64decode(encoded, validate=True)
                assert len(data) == 64, 'Invalid production signature size'
            else:
                data = (HANDOFF / name).read_bytes()
            destination = staged / name
            if destination.exists():
                assert destination.read_bytes() == data, 'Refusing to replace existing asset'
            else:
                assert release['draft'] is True, 'Published asset is missing'
                destination.write_bytes(data)
                missing.append(destination)
        authenticate_assets(staged)
        if release['draft'] is True:
            # No clobber: an unexpected concurrent change fails safely.
            if missing:
                execute('gh', 'release', 'upload', release['tag_name'], '--repo', REPOSITORY,
                        *(str(path) for path in missing))
            release = get_release()
            assert release['draft'] is True, 'Draft changed during upload'
            check_remote_metadata(release, staged)
            notes = (HANDOFF / 'notes.md').read_text()
            api(f'repos/{REPOSITORY}/releases/{release["id"]}', {
                'tag_name': TAG, 'target_commitish': COMMIT, 'body': notes,
            })
        # Download from GitHub again, then authenticate the actual final uploads.
        downloaded = scratch / 'downloaded'
        download_assets(downloaded)
        authenticate_assets(downloaded)
        release = get_release()
        check_remote_metadata(release, downloaded)
        if release['draft'] is True:
            release = api(f'repos/{REPOSITORY}/releases/{release["id"]}', {
                'tag_name': TAG, 'target_commitish': COMMIT,
                'draft': False, 'prerelease': False, 'make_latest': 'true',
            })
        assert release['draft'] is False and release['prerelease'] is False
        latest = api(f'repos/{REPOSITORY}/releases/latest')
        assert latest['id'] == release['id'] and latest['tag_name'] == TAG
        assert latest['target_commitish'] == COMMIT
        check_remote_metadata(latest, downloaded)
        summary = (
            f'Published [{TAG}]({latest["html_url"]}) from `{COMMIT}`. '
            'All 34 uploaded assets were downloaded and authenticated against '
            'the immutable production trust root before publication. '
            'The private signing key remained on the owner\'s Windows computer.\n'
        )
        with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as stream:
            stream.write(summary)
        print(json.dumps({'release_id': latest['id'], 'url': latest['html_url'],
                          'tag': TAG, 'source_commit': COMMIT,
                          'authenticated_assets': 34}), flush=True)


if __name__ == '__main__':
    main()

