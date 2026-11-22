#!/usr/bin/env python3
"""Prepare a tested v2.1.0 draft, then publish only its existing-key signature.

No command creates a key, exports a vault, signs in CI, or replaces public assets.
The separate build, gate, draft, signed-stage and publish commands keep publication last.
"""
import argparse
import base64
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[1]
REPO = 'msVPNco/rovunet-panel'
TAG = 'v2.1.0'
BASE_TAG = 'v2.0.2'
BASE_COMMIT = 'efa697256880b0d2c0b1494513b51f4c72a7735b'
BASE_ID = 445672321
BASE_MANIFEST_SHA256 = '38dcbe40474787132f540d7804388c3f0e51f50a58ac9aaf765cf167a30e0f78'
KEY_ID = '885d8aba22365ce42e20527bfa00e82aed3cba2f69501f146ed0496a8bc3dcbe'
SPKI = 'MCowBQYDK2VwAyEAMfWBf2jAnb20LJvMoOfN1R/4OP0tSjPec+GWU+QXdqs='
WORKFLOW = '.github/workflows/release-v2.1.0.yml'
PANELS = ('rovunet-linux-amd64', 'rovunet-linux-arm64')
REPLACED = set(PANELS) | {'install.sh', 'rovunet-panel-source.tar.gz', 'TEST-SCOPE.txt',
                         'SHA256SUMS', 'RELEASE-MANIFEST.json', 'RELEASE-MANIFEST.sig'}
GATES = {
    'signed-baseline-harness': 'Exercise original signed baseline harness',
    'shadowsocks': 'Validate Shadowsocks with reviewed Xray',
    'amneziawg': 'Exercise preserved AmneziaWG with candidate',
    'complete-setup': 'Exercise candidate complete setup',
    'system-recovery': 'Exercise candidate boundaries and recovery',
}
RUNTIME_CASES = {
    'TestGeneratedConfigWithXray': {'reality', 'xhttp', 'hysteria2', 'wireguard',
        'ss2022', 'ss2022-aes128', 'ss-aead-chacha20', 'trojan', 'vlessenc'},
    'TestManagedProtocolsRuntime': {p + suffix for p in
        ('ss2022', 'ss2022-aes128', 'ss-aead-chacha20', 'trojan', 'vlessenc')
        for suffix in ('', '-dedicated')},
    'TestShadowsocksUDPPayload': {'ss2022', 'ss2022-aes128', 'ss-aead-chacha20'},
}
FINAL_GATES = {'signed_fresh_install', 'signed_menu_update', 'account_identity_and_usage', 'tls_trust_preservation'}


def unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('Duplicate JSON member')
        result[key] = value
    return result


def decode_json(raw):
    return json.loads(raw, object_pairs_hook=unique)


def canonical(value):
    return (json.dumps(value, sort_keys=True, separators=(',', ':')) + '\n').encode()


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def run(*args, **kwargs):
    return subprocess.run(list(map(str, args)), check=True, **kwargs)


def gh_json(*args):
    return decode_json(run('gh', *args, capture_output=True).stdout)


def api(path, **body):
    if not body:
        return gh_json('api', path)
    with tempfile.TemporaryDirectory(prefix='release-public-request-') as temp:
        request = Path(temp) / 'request.json'
        request.write_bytes(canonical(body))
        return gh_json('api', '--method', 'POST', path, '--input', request)


def load_script(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def authority():
    expected = {'schema': 1, 'keys': [{'id': KEY_ID, 'spki': SPKI, 'purpose': 'production'}]}
    ring = decode_json((ROOT / 'internal/releaseauth/production-keys.json').read_bytes())
    match = re.search(r"^TRUST_ROOT_JSON='([^']*)'$", (ROOT / 'install.sh').read_text(), re.M)
    if ring != expected or not match or decode_json(match[1]) != expected:
        raise ValueError('The original installer and panel production authority must remain unchanged')
    return expected


def manifest_identity(raw, source, tag=TAG):
    if not re.fullmatch('[0-9a-f]{40}', source) or not 0 < len(raw) <= 1048576:
        raise ValueError('Invalid source or manifest size')
    manifest = decode_json(raw)
    if set(manifest) != {'schema', 'environment', 'key_id', 'repository', 'tag', 'source_commit', 'assets'} or \
            manifest['schema'] != 1 or manifest['environment'] != 'production' or \
            manifest['key_id'] != KEY_ID or manifest['repository'] != REPO or \
            manifest['tag'] != tag or manifest['source_commit'] != source:
        raise ValueError('Manifest identity differs from the reviewed release')
    entries = manifest['assets']
    if not isinstance(entries, list) or not 1 <= len(entries) <= 256:
        raise ValueError('Invalid manifest asset count')
    names = set()
    for entry in entries:
        if set(entry) != {'name', 'size', 'sha256', 'os', 'arch', 'kind', 'source', 'source_commit', 'license'}:
            raise ValueError('Invalid asset metadata')
        name = entry['name']
        if not isinstance(name, str) or not re.fullmatch('[A-Za-z0-9][A-Za-z0-9._-]{0,159}', name) or \
                name in names or name in {'RELEASE-MANIFEST.json', 'RELEASE-MANIFEST.sig'} or \
                type(entry['size']) is not int or not 0 < entry['size'] <= 536870912 or \
                not re.fullmatch('[0-9a-f]{64}', entry['sha256']):
            raise ValueError('Unsafe or duplicate asset entry')
        names.add(name)
    return manifest


def authenticate_manifest(raw, signature, source, tag=TAG):
    authority()
    manifest = manifest_identity(raw, source, tag)
    if len(signature) != 64:
        raise ValueError('An existing production Ed25519 signature is required')
    with tempfile.TemporaryDirectory(prefix='production-manifest-check-') as temp:
        directory = Path(temp)
        (directory / 'public.der').write_bytes(base64.b64decode(SPKI, validate=True))
        (directory / 'manifest').write_bytes(raw)
        (directory / 'signature').write_bytes(signature)
        load_script('sign-release.py').openssl('pkeyutl', '-verify', '-rawin', '-pubin', '-keyform', 'DER',
            '-inkey', directory / 'public.der', '-in', directory / 'manifest', '-sigfile', directory / 'signature')
    return manifest


def successful_source_run(runs, source):
    for entry in runs:
        if entry.get('head_sha') == source and entry.get('path') == '.github/workflows/ci.yml' and \
                entry.get('event') == 'push' and entry.get('status') == 'completed' and entry.get('conclusion') == 'success':
            return entry['id']
    raise ValueError('Exact source push CI must pass before release preparation or publication')


def source_ci(source):
    data = api(f'repos/{REPO}/actions/runs?head_sha={source}&event=push&per_page=100')
    return successful_source_run(data['workflow_runs'], source)


def clean_source(source):
    if not re.fullmatch('[0-9a-f]{40}', source):
        raise ValueError('An exact source commit is required')
    head = run('git', 'rev-parse', 'HEAD', cwd=ROOT, capture_output=True).stdout.decode().strip()
    dirty = run('git', 'status', '--porcelain', '--untracked-files=normal', cwd=ROOT, capture_output=True).stdout
    if head != source or dirty:
        raise ValueError('Preparation requires a clean checkout of the exact source commit')
    version = run('make', '-s', 'print-version', cwd=ROOT, capture_output=True).stdout.decode().strip()
    if version != TAG[1:]:
        raise ValueError('Makefile version differs from v2.1.0')
    authority()
    run('python3', ROOT / 'scripts/check-release-gates.py', '--report-only')
    run('python3', ROOT / 'scripts/check-functional-release.py', '--tag', TAG)


def require_actions(source=None):
    if os.environ.get('GITHUB_ACTIONS') != 'true' or os.environ.get('GITHUB_REPOSITORY') != REPO or \
            os.environ.get('GITHUB_EVENT_NAME') not in {'push', 'workflow_dispatch'}:
        raise ValueError('Draft/publication writes require the reviewed repository workflow')
    if source is not None and os.environ.get('GITHUB_SHA') != source:
        raise ValueError('Preparation workflow head must equal its exact source commit')
    value = os.environ.get('GITHUB_RUN_ID', '')
    if not value.isdecimal() or int(value) <= 0:
        raise ValueError('Missing preparation workflow identity')
    return int(value)


def latest_lease():
    latest = api(f'repos/{REPO}/releases/latest')
    if latest.get('id') != BASE_ID or latest.get('tag_name') != BASE_TAG or latest.get('draft') or latest.get('prerelease'):
        raise ValueError('Public Latest changed from the reviewed v2.0.2 baseline')


def existing_tag(source):
    result = subprocess.run(['gh', 'api', f'repos/{REPO}/git/ref/tags/{TAG}'], capture_output=True)
    if result.returncode:
        if b'HTTP 404' in result.stderr:
            return
        raise ValueError('Could not establish the existing release tag identity')
    ref = decode_json(result.stdout).get('object', {})
    if ref.get('type') != 'commit' or ref.get('sha') != source:
        raise ValueError('An existing release tag points to another source; retagging is forbidden')


def release_assets(release):
    result = {}
    for entry in release.get('assets', []):
        name = entry.get('name', '')
        if not re.fullmatch('[A-Za-z0-9][A-Za-z0-9._-]{0,159}', name) or name in result or \
                type(entry.get('id')) is not int or type(entry.get('size')) is not int or \
                not 0 < entry['size'] <= 536870912 or entry.get('state') != 'uploaded':
            raise ValueError('Unsafe, incomplete or duplicate GitHub asset metadata')
        result[name] = entry
    if len(result) > 256:
        raise ValueError('Too many GitHub assets')
    return result


def download_asset(entry, destination):
    if destination.exists():
        raise ValueError('Refusing to overwrite a downloaded asset')
    with destination.open('xb') as stream:
        run('gh', 'api', f'repos/{REPO}/releases/assets/{entry["id"]}',
            '-H', 'Accept: application/octet-stream', stdout=stream, stderr=subprocess.PIPE)
    if destination.stat().st_size != entry['size'] or entry.get('digest') != 'sha256:' + digest(destination):
        raise ValueError('Downloaded asset differs from its GitHub digest/size')


def authenticate_base(directory):
    release = api(f'repos/{REPO}/releases/{BASE_ID}')
    if release.get('id') != BASE_ID or release.get('tag_name') != BASE_TAG or release.get('draft') or \
            release.get('prerelease') or release.get('target_commitish') != BASE_COMMIT:
        raise ValueError('The immutable v2.0.2 release identity changed')
    entries = release_assets(release)
    directory.mkdir()
    for name in ('RELEASE-MANIFEST.json', 'RELEASE-MANIFEST.sig'):
        download_asset(entries[name], directory / name)
    raw = (directory / 'RELEASE-MANIFEST.json').read_bytes()
    if hashlib.sha256(raw).hexdigest() != BASE_MANIFEST_SHA256:
        raise ValueError('The reviewed v2.0.2 manifest bytes changed')
    manifest = authenticate_manifest(raw, (directory / 'RELEASE-MANIFEST.sig').read_bytes(), BASE_COMMIT, BASE_TAG)
    expected = {x['name'] for x in manifest['assets']} | {'RELEASE-MANIFEST.json', 'RELEASE-MANIFEST.sig'}
    if len(manifest['assets']) != 38 or set(entries) != expected:
        raise ValueError('The complete immutable v2.0.2 bundle is required')
    for item in manifest['assets']:
        entry = entries[item['name']]
        if entry['size'] != item['size'] or entry.get('digest') != 'sha256:' + item['sha256']:
            raise ValueError('Base release metadata differs from its authenticated manifest')
        download_asset(entry, directory / item['name'])
    load_script('verify-release.py').verify(directory, BASE_TAG, BASE_COMMIT, authority())


def file_map(directory):
    result = {}
    for path in sorted(directory.iterdir()):
        if path.is_symlink() or not path.is_file() or not re.fullmatch('[A-Za-z0-9][A-Za-z0-9._-]{0,159}', path.name):
            raise ValueError('Candidate assets must be plain regular files')
        result[path.name] = {'size': path.stat().st_size, 'sha256': digest(path)}
    return result


def build_signing_helper(work):
    """Build the unchanged Windows vault tool; never execute it or access a key."""
    signing = work / 'signing'
    signing.mkdir()
    env = os.environ.copy()
    env.update(GOOS='windows', GOARCH='amd64', CGO_ENABLED='0', GOAMD64='v1',
               GOTOOLCHAIN='local', GOTELEMETRY='off', GOENV='off', GOPROXY='off')
    run('go', 'build', '-buildvcs=false', '-trimpath', '-ldflags=-s -w',
        '-o', signing / 'signing-vault.exe', './tools/signing-vault', env=env, cwd=ROOT)
    shutil.copyfile(ROOT / 'scripts/panel-signing-vault.ps1', signing / 'panel-signing-vault.ps1')
    shutil.copyfile(ROOT / 'scripts/sign-v2.1.0-candidate.ps1', signing / 'SIGN-CANDIDATE.ps1')
    return file_map(signing)


def build(source, work):
    clean_source(source)
    run_id = require_actions(source)
    ci = source_ci(source)
    latest_lease()
    if work.exists():
        raise ValueError('Use a fresh preparation directory')
    work.mkdir(parents=True)
    base, assets = work / 'base', work / 'assets'
    authenticate_base(base)
    assets.mkdir()
    for path in base.iterdir():
        if path.name not in REPLACED:
            shutil.copyfile(path, assets / path.name)
    if run('go', 'env', 'GOVERSION', capture_output=True).stdout.decode().strip() != 'go1.27.4':
        raise ValueError('The reviewed Go1.27.4 toolchain is required')
    flags = '-s -w -X github.com/msVPNco/rovunet-panel/internal/version.Version=2.1.0 ' + \
            '-X github.com/msVPNco/rovunet-panel/internal/version.Commit=' + source
    for arch in ('amd64', 'arm64'):
        env = os.environ.copy()
        env.update(GOOS='linux', GOARCH=arch, CGO_ENABLED='0', GOAMD64='v1', GOARM64='v8.0',
                   GOTOOLCHAIN='local', GOTELEMETRY='off', GOENV='off')
        run('go', 'build', '-buildvcs=false', '-trimpath', '-ldflags', flags,
            '-o', assets / ('rovunet-linux-' + arch), '.', env=env, cwd=ROOT)
    expected_version = f'rovunet {TAG} ({source})'
    result = run(assets / PANELS[0], '--version', capture_output=True)
    if (result.stdout + result.stderr).decode().strip() != expected_version:
        raise ValueError('Candidate version/source identity differs')
    clean_source(source)
    assembler = load_script('assemble-patch-assets.py')
    (assets / 'rovunet-panel-source.tar.gz').write_bytes(assembler.source_archive(assembler.git_tree()))
    shutil.copyfile(ROOT / 'install.sh', assets / 'install.sh')
    shutil.copyfile(ROOT / 'docs/releases/v2.1.0-test-scope.txt', assets / 'TEST-SCOPE.txt')
    signing_files = build_signing_helper(work)
    clean_source(source)
    receipt = {'schema': 1, 'tag': TAG, 'source_commit': source, 'source_ci': ci,
               'prepare_run_id': run_id, 'base_tag': BASE_TAG, 'base_manifest_sha256': BASE_MANIFEST_SHA256,
               'files': file_map(assets), 'signing_files': signing_files}
    (work / 'BUILD-RECEIPT.json').write_bytes(canonical(receipt))
    for path in assets.glob('*-linux-*'):
        if path.name.endswith(('amd64', 'arm64')):
            path.chmod(0o755)
    print('Unsigned candidate built from the exact source and authenticated v2.0.2 bundle.')


def checked_build(work):
    receipt = decode_json((work / 'BUILD-RECEIPT.json').read_bytes())
    if receipt.get('schema') != 1 or receipt.get('tag') != TAG or receipt.get('base_tag') != BASE_TAG or \
            receipt.get('base_manifest_sha256') != BASE_MANIFEST_SHA256 or \
            file_map(work / 'assets') != receipt.get('files') or \
            file_map(work / 'signing') != receipt.get('signing_files'):
        raise ValueError('Candidate changed after preparation')
    return receipt


def runtime_results(raw):
    passed, seen = set(), set()
    for line in raw.splitlines():
        if not line.strip():
            continue
        event = decode_json(line)
        name, action = event.get('Test', ''), event.get('Action')
        top = name.split('/', 1)[0]
        if action == 'fail' or (top in RUNTIME_CASES and action == 'skip'):
            raise ValueError('An exact-runtime test failed or was skipped')
        if top in RUNTIME_CASES:
            seen.add(top)
            if action == 'pass':
                passed.add(name)
    required = set(RUNTIME_CASES) | {name + '/' + case for name, cases in RUNTIME_CASES.items() for case in cases}
    if seen != set(RUNTIME_CASES) or not required <= passed:
        raise ValueError('Missing required AES128/ChaCha/runtime payload test results')
    return {name: sorted(case for case in cases if name + '/' + case in passed) for name, cases in RUNTIME_CASES.items()}


def baseline_smoke_summary(summary, panel_hash, installer_hash):
    if summary.get('schema') != 1 or summary.get('purpose') != 'rovunet-signed-baseline-harness-smoke' or \
            summary.get('status') != 'pass' or summary.get('tag') != BASE_TAG or \
            summary.get('source_commit') != BASE_COMMIT or summary.get('manifest_sha256') != BASE_MANIFEST_SHA256 or \
            summary.get('panel_sha256') != panel_hash or summary.get('production_signature_verified') is not True or \
            summary.get('final_v210_acceptance') is not False or \
            summary.get('runner_sha256') != digest(ROOT / 'scripts/test-signed-v210-update.py') or \
            summary.get('simulator_sha256') != digest(ROOT / 'scripts/test-complete-setup.py') or \
            summary.get('scope', {}).get('environment') != 'isolated-linux-namespaces' or \
            summary.get('scope', {}).get('systemd') != 'simulated':
        raise ValueError('Signed baseline smoke does not match the immutable baseline/reviewed harness')
    fresh = summary.get('signed_baseline_fresh_installer', {})
    fixture = summary.get('fixture', {})
    if fresh.get('installed_sha256') != panel_hash or fresh.get('installer_sha256') != installer_hash or \
            fresh.get('negative_cases_rejected') != ['signature', 'panel'] or \
            fresh.get('setup_pending') is not True or fresh.get('pending_repeat_preserved') is not True or \
            fixture.get('sqlite_confirmed_bytes') != 1217 or \
            fixture.get('active_legacy_cores') != ['managed-tuic', 'wireguard'] or fixture.get('tls_trust_present') is not True:
        raise ValueError('Signed baseline installer/accounting/TLS/legacy-core fixture did not pass')


def execution_logs(work, executions, phase):
    if not isinstance(executions, list) or not 1 <= len(executions) <= 256:
        raise ValueError('Acceptance is missing its execution logs')
    log_root = work / 'logs'
    if log_root.is_symlink() or (log_root / phase).is_symlink():
        raise ValueError('Execution logs must remain in the staged workspace')
    for execution in executions:
        log_name = execution.get('name', '')
        if not re.fullmatch('logs/' + phase + '/[A-Za-z0-9._-]+[.]log', log_name) or \
                type(execution.get('exit_code')) is not int:
            raise ValueError('Unexpected execution-log identity')
        log = work / log_name
        if log.is_symlink() or not log.is_file() or log.stat().st_size > 16777216 or digest(log) != execution.get('sha256'):
            raise ValueError('A retained signed acceptance execution log changed')


def baseline_smoke_result(work):
    raw = (work / 'base/RELEASE-MANIFEST.json').read_bytes()
    if hashlib.sha256(raw).hexdigest() != BASE_MANIFEST_SHA256:
        raise ValueError('Baseline smoke requires the original immutable manifest')
    manifest = authenticate_manifest(raw, (work / 'base/RELEASE-MANIFEST.sig').read_bytes(), BASE_COMMIT, BASE_TAG)
    entries = {a['name']: a for a in manifest['assets']}
    for name in (PANELS[0], 'install.sh'):
        if digest(work / 'base' / name) != entries[name]['sha256']:
            raise ValueError('Signed baseline smoke input changed')
    smoke_work = work / 'baseline-smoke'
    path = smoke_work / 'BASELINE-SMOKE.json'
    if smoke_work.is_symlink() or path.is_symlink() or not path.is_file() or path.stat().st_size > 1048576:
        raise ValueError('Missing signed-baseline harness smoke summary')
    summary = decode_json(path.read_bytes())
    baseline_smoke_summary(summary, entries[PANELS[0]]['sha256'], entries['install.sh']['sha256'])
    execution_logs(smoke_work, summary.get('executions'), 'smoke')
    return {'tag': BASE_TAG, 'source_commit': BASE_COMMIT, 'manifest_sha256': BASE_MANIFEST_SHA256,
            'summary_sha256': digest(path), 'final_v210_acceptance': False}


def baseline_smoke_identity(value):
    if not isinstance(value, dict) or set(value) != {'tag', 'source_commit', 'manifest_sha256', 'summary_sha256', 'final_v210_acceptance'} or \
            value['tag'] != BASE_TAG or value['source_commit'] != BASE_COMMIT or \
            value['manifest_sha256'] != BASE_MANIFEST_SHA256 or value['final_v210_acceptance'] is not False or \
            not isinstance(value['summary_sha256'], str) or not re.fullmatch('[0-9a-f]{64}', value['summary_sha256']) or \
            value['summary_sha256'] == '0' * 64:
        raise ValueError('Candidate is missing its original signed-baseline harness qualification')


def record_gate(work, name, log=None):
    if name not in GATES or (name == 'shadowsocks') != (log is not None):
        raise ValueError('Invalid required gate/log')
    receipt = checked_build(work)
    require_actions(receipt['source_commit'])
    proof = {'schema': 1, 'gate': name, 'source_commit': receipt['source_commit'],
             'panels': {p: receipt['files'][p] for p in PANELS}}
    if log is not None:
        proof['runtime_cases'] = runtime_results(log.read_bytes())
    if name == 'signed-baseline-harness':
        proof['baseline_smoke'] = baseline_smoke_result(work)
    (work / ('GATE-' + name + '.json')).write_bytes(canonical(proof))


def assert_draft(release, source, draft_id=None):
    if release.get('tag_name') != TAG or release.get('target_commitish') != source or \
            release.get('draft') is not True or release.get('prerelease') is not False or \
            (draft_id is not None and release.get('id') != draft_id):
        raise ValueError('Refusing a public, foreign or differently targeted release')


def matching_remote_assets(release, files, complete=False):
    entries = release_assets(release)
    if not set(entries) <= set(files) or (complete and set(entries) != set(files)):
        raise ValueError('Release contains unexpected or missing assets')
    for name, entry in entries.items():
        if entry['size'] != files[name]['size'] or entry.get('digest') != 'sha256:' + files[name]['sha256']:
            raise ValueError('Existing draft asset differs; overwriting is forbidden')
    return entries


def find_release():
    for page in range(1, 11):
        releases = api(f'repos/{REPO}/releases?per_page=100&page={page}')
        matches = [r for r in releases if r.get('tag_name') == TAG]
        if len(matches) > 1:
            raise ValueError('Ambiguous release identity')
        if matches:
            return api(f'repos/{REPO}/releases/{matches[0]["id"]}')
        if len(releases) < 100:
            return None
    raise ValueError('Release inventory exceeded its bound')


def upload_missing(release, assets, source):
    release = api(f'repos/{REPO}/releases/{release["id"]}')
    assert_draft(release, source)
    files = file_map(assets)
    existing = matching_remote_assets(release, files)
    missing = [str(assets / name) for name in sorted(set(files) - set(existing))]
    if missing:
        run('gh', 'release', 'upload', TAG, '--repo', REPO, *missing)
    current = api(f'repos/{REPO}/releases/{release["id"]}')
    matching_remote_assets(current, files, complete=True)
    return current


def stage_draft(work):
    receipt = checked_build(work)
    source = receipt['source_commit']
    if require_actions(source) != receipt['prepare_run_id']:
        raise ValueError('Preparation run changed')
    clean_source(source)
    source_ci(source)
    proofs = {}
    for name in GATES:
        proof = decode_json((work / ('GATE-' + name + '.json')).read_bytes())
        if proof.get('schema') != 1 or proof.get('gate') != name or proof.get('source_commit') != source or \
                proof.get('panels') != {p: receipt['files'][p] for p in PANELS}:
            raise ValueError('Candidate acceptance does not match these exact binaries')
        if name == 'signed-baseline-harness':
            baseline_smoke_identity(proof.get('baseline_smoke'))
        proofs[name] = proof
    if proofs['shadowsocks'].get('runtime_cases') != {k: sorted(v) for k, v in RUNTIME_CASES.items()}:
        raise ValueError('Exact Shadowsocks acceptance is incomplete')
    acceptance = {k: receipt[k] for k in ('schema', 'tag', 'source_commit', 'source_ci', 'prepare_run_id',
                                         'base_tag', 'base_manifest_sha256')}
    acceptance.update(gates=proofs, scope='Automated Linux isolation; no new physical client, ISP, ARM64 or off-site custody certification.')
    assets = work / 'assets'
    (assets / 'ACCEPTANCE.json').write_bytes(canonical(acceptance))
    (assets / 'SHA256SUMS').write_text(''.join(digest(p) + '  ' + p.name + '\n' for p in sorted(assets.iterdir())), encoding='utf-8')
    signer = load_script('sign-release.py')
    manifest = {'schema': 1, 'environment': 'production', 'key_id': KEY_ID, 'repository': REPO,
                'tag': TAG, 'source_commit': source, 'assets': [dict(name=name, **info, **signer.metadata(name, source))
                    for name, info in file_map(assets).items()]}
    raw = canonical(manifest)
    manifest_identity(raw, source)
    (assets / 'RELEASE-MANIFEST.json').write_bytes(raw)
    latest_lease()
    existing_tag(source)
    release = find_release()
    if release is None:
        release = api(f'repos/{REPO}/releases', tag_name=TAG, target_commitish=source, draft=True,
                      prerelease=False, make_latest='false', name='Rovunet Panel v2.1.0 — unsigned candidate',
                      body=(ROOT / 'docs/releases/v2.1.0.md').read_text() + '\n\nDraft: awaiting the existing Windows production-vault signature.\n')
    assert_draft(release, source)
    release = upload_missing(release, assets, source)
    assert_draft(release, source)
    info = {'tag': TAG, 'source_commit': source, 'draft_id': release['id'],
            'prepare_run_id': receipt['prepare_run_id'], 'manifest_sha256': hashlib.sha256(raw).hexdigest(),
            'unsigned_assets': len(manifest['assets']) + 1, 'production_signature_present': False,
            'signing_files': receipt['signing_files']}
    (work / 'CANDIDATE.json').write_bytes(canonical(info))
    final_template = {
        'schema': 1, 'purpose': 'rovunet-final-signed-release-acceptance', 'tag': TAG,
        'source_commit': source, 'manifest_sha256': info['manifest_sha256'], 'production_key_id': KEY_ID,
        'panel_sha256': {p: receipt['files'][p]['sha256'] for p in PANELS},
        'runtime_base_tag': BASE_TAG, 'runtime_base_manifest_sha256': BASE_MANIFEST_SHA256,
        'environment': 'isolated-linux-namespaces', 'architecture': 'amd64',
        'production_signature_verified': False,
        'upgrade_baseline': {'tag': BASE_TAG, 'source_commit': BASE_COMMIT},
        'checks': {name: {'status': 'not_run', 'evidence_sha256': None} for name in sorted(FINAL_GATES)},
    }
    (work / 'FINAL-ACCEPTANCE.template.json').write_bytes(canonical(final_template))
    signing_handoff(work, info, raw)
    print(json.dumps(info))


def signing_handoff(work, info, raw):
    signing = work / 'signing'
    (signing / 'RELEASE-MANIFEST.json').write_bytes(raw)
    (signing / 'CANDIDATE.json').write_bytes(canonical(info))
    command = 'pwsh -NoProfile -File .\\SIGN-CANDIDATE.ps1 -ExpectedManifestSha256 ' + \
        info['manifest_sha256'] + ' -ExpectedHelperSha256 ' + info['signing_files']['signing-vault.exe']['sha256']
    instructions = (
        'Rovunet Panel v2.1.0 unsigned candidate\n\n'
        f'Exact source: {info["source_commit"]}\nDraft release ID: {info["draft_id"]}\n'
        f'Manifest SHA-256: {info["manifest_sha256"]}\nProduction root: {KEY_ID}\n\n'
        '1. Extract the small rovunet-v2.1.0-windows-signing artifact on the original\n'
        '   Windows account that holds the existing production vault.\n'
        '2. Open PowerShell 7 in that extracted folder and run this one command:\n\n'
        + command + '\n\n'
        '3. Return only RELEASE-MANIFEST.json and RELEASE-MANIFEST.sig.\n\n'
        'The command checks the exact manifest/helper hashes, checks the existing\n'
        'production identity, then signs without exporting or changing the key.\n'
        'It uses the existing vault at:\n'
        '  $env:LOCALAPPDATA\\Rovunet\\PanelProductionSigning-v1\n'
        'If that existing vault has a different path, append -Vault "FULL_PATH".\n'
        'No Go, Python, OpenSSL, new key, recovery or execution-policy change is needed.\n'
        'The Windows helper is built from the exact source above; its immutable SHA-256 is:\n'
        + info['signing_files']['signing-vault.exe']['sha256'] + '\n\n'
        'The publication workflow\n'
        'will authenticate both, stage the exact candidate, and exercise the real\n'
        'signed installer and official 2.0.2 menu updater in isolated Linux\n'
        'namespaces before publication. Systemd is simulated in that lane; it\n'
        'does not certify a physical VM, ARM64 execution, or live ISP traffic.\n'
        'The receipt template deliberately contains not_run/false values. Only\n'
        'the executed acceptance runner supplies the receipt and retained\n'
        'evidence to the final publication guard; the owner supplies no receipt.\n'
        'Neither the vault, recovery\n'
        'identity nor a private key belongs in GitHub or this package.\n'
        'The draft remains unpublished until the signed candidate is verified.\n')
    (work / 'SIGNING-INSTRUCTIONS.txt').write_text(instructions, encoding='utf-8')
    (signing / 'SIGNING-INSTRUCTIONS.txt').write_text(instructions, encoding='utf-8')


def publication_request(path):
    if path.stat().st_size > 2 * 1024 * 1024:
        raise ValueError('Publication request exceeds size bound')
    request = decode_json(path.read_bytes())
    if set(request) != {'schema', 'tag', 'previous_tag', 'source_commit', 'draft_id',
                        'manifest_base64', 'signature_base64', 'publish_latest'} or \
            request['schema'] != 1 or request['tag'] != TAG or request['previous_tag'] != BASE_TAG or \
            request['publish_latest'] is not True or type(request['draft_id']) is not int or request['draft_id'] <= 0:
        raise ValueError('Unexpected publication request identity')
    raw = base64.b64decode(request['manifest_base64'], validate=True)
    signature = base64.b64decode(request['signature_base64'], validate=True)
    manifest = authenticate_manifest(raw, signature, request['source_commit'])
    return request, manifest, raw, signature


def final_acceptance(raw, expected_hash, manifest_raw, manifest):
    if not 0 < len(raw) <= 262144 or not isinstance(expected_hash, str) or \
            not re.fullmatch('[0-9a-f]{64}', expected_hash) or hashlib.sha256(raw).hexdigest() != expected_hash:
        raise ValueError('An intact final acceptance receipt/hash is required')
    receipt = decode_json(raw)
    if set(receipt) != {'schema', 'purpose', 'tag', 'source_commit', 'manifest_sha256', 'production_key_id',
                        'panel_sha256', 'runtime_base_tag', 'runtime_base_manifest_sha256', 'environment',
                        'architecture', 'production_signature_verified', 'upgrade_baseline', 'checks'} or \
            receipt['schema'] != 1 or receipt['purpose'] != 'rovunet-final-signed-release-acceptance' or \
            receipt['tag'] != TAG or receipt['source_commit'] != manifest['source_commit'] or \
            receipt['manifest_sha256'] != hashlib.sha256(manifest_raw).hexdigest() or \
            receipt['production_key_id'] != KEY_ID or receipt['runtime_base_tag'] != BASE_TAG or \
            receipt['runtime_base_manifest_sha256'] != BASE_MANIFEST_SHA256 or \
            receipt['environment'] != 'isolated-linux-namespaces' or receipt['architecture'] != 'amd64' or \
            receipt['production_signature_verified'] is not True:
        raise ValueError('Final acceptance is unverified or bound to another source/signature/runtime')
    panels = {a['name']: a['sha256'] for a in manifest['assets'] if a['name'] in PANELS}
    if set(panels) != set(PANELS) or receipt['panel_sha256'] != panels:
        raise ValueError('Final acceptance does not bind both exact panel binaries')
    baseline = receipt['upgrade_baseline']
    if baseline != {'tag': BASE_TAG, 'source_commit': BASE_COMMIT}:
        raise ValueError('Final menu-update acceptance has an unreviewed baseline')
    checks = receipt['checks']
    if not isinstance(checks, dict) or set(checks) != FINAL_GATES:
        raise ValueError('Final signed-install/update acceptance is incomplete')
    for check in checks.values():
        if not isinstance(check, dict) or set(check) != {'status', 'evidence_sha256'} or check['status'] != 'pass' or \
                not isinstance(check['evidence_sha256'], str) or \
                not re.fullmatch('[0-9a-f]{64}', check['evidence_sha256']) or check['evidence_sha256'] == '0' * 64:
            raise ValueError('A final signed-install/update or identity/TLS gate remains unrun')
    return receipt


def retained_final_acceptance(work, manifest_raw, manifest):
    """Require the runner's local receipt and all four retained evidence files."""
    receipt_path = work / 'FINAL-ACCEPTANCE.json'
    if receipt_path.is_symlink() or not receipt_path.is_file() or receipt_path.stat().st_size > 262144:
        raise ValueError('The final signed acceptance runner has not produced its receipt')
    raw = receipt_path.read_bytes()
    receipt = final_acceptance(raw, hashlib.sha256(raw).hexdigest(), manifest_raw, manifest)
    evidence = work / 'final' / 'evidence'
    if (work / 'final').is_symlink() or evidence.is_symlink() or not evidence.is_dir():
        raise ValueError('Final acceptance evidence must be retained in this workflow workspace')
    for name, check in receipt['checks'].items():
        path = evidence / (name + '.json')
        if path.is_symlink() or not path.is_file() or not 0 < path.stat().st_size <= 1048576 or \
                digest(path) != check['evidence_sha256']:
            raise ValueError('Missing or altered final acceptance evidence for ' + name)
        proof = decode_json(path.read_bytes())
        if proof.get('schema') != 1 or proof.get('check') != name or proof.get('status') != 'pass' or \
                any(proof.get(k) != receipt[k] for k in ('tag', 'source_commit', 'manifest_sha256', 'production_key_id', 'panel_sha256')) or \
                proof.get('runner_sha256') != digest(ROOT / 'scripts/test-signed-v210-update.py') or \
                proof.get('simulator_sha256') != digest(ROOT / 'scripts/test-complete-setup.py') or \
                proof.get('scope', {}).get('environment') != 'isolated-linux-namespaces' or \
                proof.get('scope', {}).get('systemd') != 'simulated':
            raise ValueError('Final evidence is not from the exact reviewed signed acceptance runner')
        phase = 'fresh' if name == 'signed_fresh_install' else 'upgrade'
        execution_logs(work / 'final', proof.get('executions'), phase)
        log_root = work / 'final' / 'logs'
        requests = log_root / phase / 'https-requests.json'
        if requests.is_symlink() or not requests.is_file() or requests.stat().st_size > 1048576 or \
                digest(requests) != proof.get('https_requests_sha256'):
            raise ValueError('Retained final HTTPS request evidence changed')
    return receipt


def preparation_succeeded(run_info, jobs, source, run_id):
    if run_info.get('id') != run_id or run_info.get('head_sha') != source or run_info.get('path') != WORKFLOW or \
            run_info.get('status') != 'completed' or run_info.get('conclusion') != 'success' or \
            run_info.get('event') not in {'push', 'workflow_dispatch'}:
        raise ValueError('Exact candidate preparation workflow must have succeeded')
    prepare = [job for job in jobs if job.get('name') == 'prepare' and job.get('conclusion') == 'success']
    if len(prepare) != 1:
        raise ValueError('A successful candidate preparation job is required')
    steps = {s['name']: s.get('conclusion') for s in prepare[0].get('steps', [])}
    if any(steps.get(name) != 'success' for name in GATES.values()) or steps.get('Create the tested unsigned draft') != 'success':
        raise ValueError('Required candidate runtime/system gates did not all pass')


def signed_candidate(request, manifest, raw, signature, assets, *, download=False):
    """Verify the immutable draft and preparation history without GitHub writes."""
    source = request['source_commit']
    release = api(f'repos/{REPO}/releases/{request["draft_id"]}')
    assert_draft(release, source, request['draft_id'])
    entries = release_assets(release)
    names = {a['name'] for a in manifest['assets']} | {'RELEASE-MANIFEST.json'}
    if set(entries) not in (names, names | {'RELEASE-MANIFEST.sig'}):
        raise ValueError('Draft asset set differs from the signed candidate')
    files = {a['name']: {'size': a['size'], 'sha256': a['sha256']} for a in manifest['assets']}
    for name, payload in [('RELEASE-MANIFEST.json', raw), ('RELEASE-MANIFEST.sig', signature)]:
        files[name] = {'size': len(payload), 'sha256': hashlib.sha256(payload).hexdigest()}
    matching_remote_assets(release, files)
    if download:
        assets.mkdir()
        for name, entry in entries.items():
            download_asset(entry, assets / name)
        if 'RELEASE-MANIFEST.sig' not in entries:
            (assets / 'RELEASE-MANIFEST.sig').write_bytes(signature)
    if assets.is_symlink() or file_map(assets) != files:
        raise ValueError('Staged candidate differs from the signed manifest/signature')
    load_script('verify-release.py').verify(assets, TAG, source, authority())
    acceptance = decode_json((assets / 'ACCEPTANCE.json').read_bytes())
    if acceptance.get('source_commit') != source or acceptance.get('tag') != TAG or \
            acceptance.get('base_tag') != BASE_TAG or acceptance.get('base_manifest_sha256') != BASE_MANIFEST_SHA256:
        raise ValueError('Signed candidate acceptance has another source/runtime baseline')
    ci_id = acceptance.get('source_ci')
    if type(ci_id) is not int or ci_id <= 0 or \
            successful_source_run([api(f'repos/{REPO}/actions/runs/{ci_id}')], source) != ci_id:
        raise ValueError('Signed candidate source CI is not successful for this exact source')
    run_id = acceptance.get('prepare_run_id')
    if type(run_id) is not int or run_id <= 0:
        raise ValueError('Invalid candidate preparation run')
    preparation_succeeded(api(f'repos/{REPO}/actions/runs/{run_id}'),
        api(f'repos/{REPO}/actions/runs/{run_id}/jobs?filter=latest&per_page=100')['jobs'], source, run_id)
    for name in GATES:
        proof = acceptance.get('gates', {}).get(name, {})
        if proof.get('source_commit') != source or proof.get('gate') != name or \
                proof.get('panels') != {p: files[p] for p in PANELS}:
            raise ValueError('Signed acceptance does not bind these panel bytes')
        if name == 'signed-baseline-harness':
            baseline_smoke_identity(proof.get('baseline_smoke'))
    if acceptance['gates']['shadowsocks'].get('runtime_cases') != {k: sorted(v) for k, v in RUNTIME_CASES.items()}:
        raise ValueError('Signed Shadowsocks runtime acceptance is incomplete')
    return release


def stage_identity(request, raw, signature, run_id):
    return {'schema': 1, 'tag': TAG, 'source_commit': request['source_commit'],
            'draft_id': request['draft_id'], 'publication_run_id': run_id,
            'manifest_sha256': hashlib.sha256(raw).hexdigest(),
            'signature_sha256': hashlib.sha256(signature).hexdigest(),
            'base_tag': BASE_TAG, 'base_manifest_sha256': BASE_MANIFEST_SHA256}


def stage_signed(request_path, work):
    run_id = require_actions()
    request, manifest, raw, signature = publication_request(request_path)
    source = request['source_commit']
    clean_source(source)
    source_ci(source)
    latest_lease()
    existing_tag(source)
    if work.exists():
        raise ValueError('Use a fresh signed candidate staging directory')
    work.mkdir(parents=True)
    signed_candidate(request, manifest, raw, signature, work / 'assets', download=True)
    authenticate_base(work / 'base')
    (work / 'STAGED-CANDIDATE.json').write_bytes(canonical(stage_identity(request, raw, signature, run_id)))
    print('Authenticated signed candidate and original baseline staged; no release assets or state were changed.')


def publish(request_path, work):
    run_id = require_actions()
    request, manifest, raw, signature = publication_request(request_path)
    # Final acceptance is local runner output, never owner-supplied request data.
    # Fail before any GitHub write (or lookup) if execution/evidence is missing.
    retained_final_acceptance(work, raw, manifest)
    staged = work / 'STAGED-CANDIDATE.json'
    if staged.is_symlink() or decode_json(staged.read_bytes()) != stage_identity(request, raw, signature, run_id):
        raise ValueError('Signed candidate staging belongs to another source/signature/workflow run')
    source = request['source_commit']
    clean_source(source)
    source_ci(source)
    latest_lease()
    existing_tag(source)
    assets = work / 'assets'
    release = signed_candidate(request, manifest, raw, signature, assets)
    release = upload_missing(release, assets, source)
    assert_draft(release, source, request['draft_id'])
    source_ci(source)
    latest_lease()
    existing_tag(source)
    body = {'draft': False, 'prerelease': False, 'make_latest': 'true', 'name': 'Rovunet Panel v2.1.0',
            'body': (ROOT / 'docs/releases/v2.1.0.md').read_text()}
    with tempfile.TemporaryDirectory(prefix='release-publication-') as temp:
        path = Path(temp) / 'publish.json'
        path.write_bytes(canonical(body))
        published = gh_json('api', '--method', 'PATCH', f'repos/{REPO}/releases/{release["id"]}', '--input', path)
    tag = api(f'repos/{REPO}/git/ref/tags/{TAG}')
    if published.get('draft') is not False or published.get('prerelease') is not False or \
            tag.get('object', {}).get('sha') != source or tag.get('object', {}).get('type') != 'commit':
        raise ValueError('Publication returned an unexpected tag/identity; inspect it before retrying')
    matching_remote_assets(api(f'repos/{REPO}/releases/{release["id"]}'), file_map(assets), complete=True)
    if api(f'repos/{REPO}/releases/latest').get('id') != release['id']:
        raise ValueError('Published release is not the selected Latest; inspect the concurrent release state')
    print(json.dumps({'published': TAG, 'source_commit': source, 'release_id': release['id'],
                      'assets': len(manifest['assets']) + 2, 'production_key_id': KEY_ID}))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=['build', 'record-gate', 'draft', 'validate-request', 'stage-signed', 'publish'])
    parser.add_argument('--source')
    parser.add_argument('--work', type=Path)
    parser.add_argument('--gate', choices=sorted(GATES))
    parser.add_argument('--log', type=Path)
    parser.add_argument('--request', type=Path)
    args = parser.parse_args()
    if args.mode == 'build':
        build(args.source, args.work.resolve())
    elif args.mode == 'record-gate':
        record_gate(args.work.resolve(), args.gate, args.log)
    elif args.mode == 'draft':
        stage_draft(args.work.resolve())
    elif args.mode == 'validate-request':
        request, _, _, _ = publication_request(args.request)
        if os.environ.get('GITHUB_OUTPUT'):
            with open(os.environ['GITHUB_OUTPUT'], 'a', encoding='utf-8') as output:
                output.write('source=' + request['source_commit'] + '\n')
        print(json.dumps({'tag': TAG, 'source_commit': request['source_commit'], 'draft_id': request['draft_id']}))
    elif args.mode == 'stage-signed':
        stage_signed(args.request, args.work.resolve())
    else:
        publish(args.request, args.work.resolve())


if __name__ == '__main__':
    try:
        main()
    except subprocess.CalledProcessError as error:
        sys.exit(f'Release operation stopped: {Path(str(error.cmd[0])).name} exited {error.returncode}. No key data was logged.')
    except (ValueError, KeyError, TypeError, OSError) as error:
        sys.exit('Release operation stopped: ' + str(error))
