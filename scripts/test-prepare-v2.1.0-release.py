#!/usr/bin/env python3
"""Adversarial release-route checks; no network, vault, key creation or writes to GitHub."""
import base64
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('prepare_v210', Path(__file__).with_name('prepare-v2.1.0-release.py'))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)
SOURCE = '1' * 40


def manifest():
    entry = dict(name='rovunet-linux-amd64', size=64, sha256='2' * 64,
                 os='linux', arch='amd64', kind='executable', source='https://github.com/' + release.REPO,
                 source_commit=SOURCE, license='AGPL-3.0')
    second = dict(entry, name='rovunet-linux-arm64', arch='arm64', sha256='3' * 64)
    return dict(schema=1, environment='production', key_id=release.KEY_ID,
                repository=release.REPO, tag=release.TAG, source_commit=SOURCE, assets=[entry, second])


def final_receipt(data, raw):
    return dict(schema=1, purpose='rovunet-final-signed-release-acceptance', tag=release.TAG,
                source_commit=SOURCE, manifest_sha256=hashlib.sha256(raw).hexdigest(),
                production_key_id=release.KEY_ID, panel_sha256={a['name']: a['sha256'] for a in data['assets']},
                runtime_base_tag=release.BASE_TAG, runtime_base_manifest_sha256=release.BASE_MANIFEST_SHA256,
                environment='isolated-linux-namespaces', architecture='amd64', production_signature_verified=True,
                upgrade_baseline=dict(tag=release.BASE_TAG, source_commit=release.BASE_COMMIT),
                checks={g: dict(status='pass', evidence_sha256='4' * 64) for g in release.FINAL_GATES})


def runtime_events():
    rows = []
    for name, cases in release.RUNTIME_CASES.items():
        rows.append(dict(Action='run', Test=name))
        rows.extend(dict(Action='pass', Test=name + '/' + case) for case in sorted(cases))
        rows.append(dict(Action='pass', Test=name))
    return rows


def json_lines(rows):
    return b''.join(release.canonical(row) for row in rows)


def publication_request(data, raw):
    return dict(schema=1, tag=release.TAG, previous_tag=release.BASE_TAG, source_commit=SOURCE,
                draft_id=7, manifest_base64=base64.b64encode(raw).decode(),
                signature_base64=base64.b64encode(bytes(64)).decode(), publish_latest=True)


class ReleaseRouteTests(unittest.TestCase):
    def test_original_installer_and_panel_authority_agree(self):
        self.assertEqual(release.authority()['keys'][0]['id'], release.KEY_ID)

    def test_manifest_rejects_wrong_authority_tag_source_and_duplicate_fields(self):
        for key, value in [('environment', 'test'), ('key_id', '3' * 64), ('repository', 'other/panel'),
                           ('tag', 'v2.0.2'), ('source_commit', '3' * 40)]:
            data = manifest()
            data[key] = value
            with self.subTest(key=key), self.assertRaises(ValueError):
                release.manifest_identity(release.canonical(data), SOURCE)
        raw = release.canonical(manifest()).replace(b'"schema":1', b'"schema":1,"schema":1')
        with self.assertRaises(ValueError):
            release.manifest_identity(raw, SOURCE)

    def test_manifest_rejects_path_traversal_duplicates_and_unbounded_sizes(self):
        for key, value in [('name', '../private'), ('name', 'RELEASE-MANIFEST.sig'),
                           ('size', True), ('size', 0), ('size', 536870913), ('sha256', 'bad')]:
            data = manifest()
            data['assets'][0][key] = value
            with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                release.manifest_identity(release.canonical(data), SOURCE)
        data = manifest()
        data['assets'].append(copy.deepcopy(data['assets'][0]))
        with self.assertRaises(ValueError):
            release.manifest_identity(release.canonical(data), SOURCE)

    def test_source_gate_requires_successful_push_for_exact_source_and_workflow(self):
        good = dict(id=7, head_sha=SOURCE, event='push', path='.github/workflows/ci.yml',
                    status='completed', conclusion='success')
        self.assertEqual(release.successful_source_run([good], SOURCE), 7)
        for key, value in [('head_sha', '0' * 40), ('event', 'pull_request'), ('path', 'another.yml'),
                           ('status', 'in_progress'), ('conclusion', 'skipped'), ('conclusion', 'failure')]:
            bad = dict(good, **{key: value})
            with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                release.successful_source_run([bad], SOURCE)

    def test_runtime_gate_requires_every_concrete_cipher_and_payload_case(self):
        rows = runtime_events()
        result = release.runtime_results(json_lines(rows))
        self.assertEqual({k: len(v) for k, v in result.items()}, {
            'TestGeneratedConfigWithXray': 9, 'TestManagedProtocolsRuntime': 10, 'TestShadowsocksUDPPayload': 3})
        for missing in ['TestShadowsocksUDPPayload/ss2022-aes128',
                        'TestManagedProtocolsRuntime/ss-aead-chacha20-dedicated', 'TestGeneratedConfigWithXray']:
            with self.subTest(missing=missing), self.assertRaises(ValueError):
                release.runtime_results(json_lines([r for r in rows if not (r['Action'] == 'pass' and r['Test'] == missing)]))

    def test_runtime_gate_rejects_skip_failure_empty_or_failed_package(self):
        for extra in [dict(Action='skip', Test='TestShadowsocksUDPPayload'),
                      dict(Action='fail', Test='TestGeneratedConfigWithXray/ss2022-aes128'),
                      dict(Action='fail', Package='coreinstall')]:
            with self.subTest(extra=extra), self.assertRaises(ValueError):
                release.runtime_results(json_lines(runtime_events() + [extra]))
        with self.assertRaises(ValueError):
            release.runtime_results(b'')

    def test_baseline_smoke_requires_original_installer_and_full_upgrade_fixture(self):
        summary = dict(schema=1, purpose='rovunet-signed-baseline-harness-smoke', status='pass',
            tag=release.BASE_TAG, source_commit=release.BASE_COMMIT, manifest_sha256=release.BASE_MANIFEST_SHA256,
            panel_sha256='2' * 64, production_signature_verified=True, final_v210_acceptance=False,
            runner_sha256=release.digest(release.ROOT / 'scripts/test-signed-v210-update.py'),
            simulator_sha256=release.digest(release.ROOT / 'scripts/test-complete-setup.py'),
            scope={'environment': 'isolated-linux-namespaces', 'systemd': 'simulated'},
            signed_baseline_fresh_installer=dict(installed_sha256='2' * 64, installer_sha256='3' * 64,
                negative_cases_rejected=['signature', 'panel'], setup_pending=True, pending_repeat_preserved=True),
            fixture=dict(sqlite_confirmed_bytes=1217, active_legacy_cores=['managed-tuic', 'wireguard'], tls_trust_present=True))
        release.baseline_smoke_summary(summary, '2' * 64, '3' * 64)
        for key, value in [('tag', release.TAG), ('final_v210_acceptance', True),
                           ('production_signature_verified', False), ('runner_sha256', '0' * 64)]:
            with self.subTest(key=key), self.assertRaises(ValueError):
                release.baseline_smoke_summary(dict(summary, **{key: value}), '2' * 64, '3' * 64)
        for group, key, value in [('fixture', 'sqlite_confirmed_bytes', 0), ('fixture', 'active_legacy_cores', ['wireguard']),
                                  ('fixture', 'tls_trust_present', False),
                                  ('signed_baseline_fresh_installer', 'negative_cases_rejected', ['signature']),
                                  ('signed_baseline_fresh_installer', 'pending_repeat_preserved', False),
                                  ('signed_baseline_fresh_installer', 'installer_sha256', '0' * 64)]:
            changed = copy.deepcopy(summary)
            changed[group][key] = value
            with self.subTest(group=group, key=key), self.assertRaises(ValueError):
                release.baseline_smoke_summary(changed, '2' * 64, '3' * 64)

    def test_published_or_foreign_draft_cannot_be_reused(self):
        good = dict(id=9, tag_name=release.TAG, target_commitish=SOURCE, draft=True, prerelease=False)
        release.assert_draft(good, SOURCE, 9)
        for key, value in [('id', 10), ('tag_name', 'v2.0.2'), ('target_commitish', '0' * 40),
                           ('draft', False), ('prerelease', True)]:
            with self.subTest(key=key), self.assertRaises(ValueError):
                release.assert_draft(dict(good, **{key: value}), SOURCE, 9)

    def test_draft_upload_never_overwrites_mismatched_or_unknown_assets(self):
        files = {'panel': dict(size=4, sha256='a' * 64)}
        asset = dict(id=2, name='panel', size=4, state='uploaded', digest='sha256:' + 'a' * 64)
        release.matching_remote_assets(dict(assets=[asset]), files, complete=True)
        for changed in [dict(asset, size=5), dict(asset, digest='sha256:' + 'b' * 64),
                        dict(asset, name='foreign'), dict(asset, state='starter')]:
            with self.subTest(changed=changed), self.assertRaises(ValueError):
                release.matching_remote_assets(dict(assets=[changed]), files)
        with self.assertRaises(ValueError):
            release.matching_remote_assets(dict(assets=[]), files, complete=True)

    def test_preparation_requires_completed_exact_source_and_every_required_gate(self):
        run = dict(id=17, head_sha=SOURCE, path=release.WORKFLOW, status='completed', conclusion='success', event='push')
        steps = [dict(name=n, conclusion='success') for n in list(release.GATES.values()) + ['Create the tested unsigned draft']]
        jobs = [dict(name='prepare', conclusion='success', steps=steps)]
        release.preparation_succeeded(run, jobs, SOURCE, 17)
        for key, value in [('head_sha', '0' * 40), ('conclusion', 'failure'), ('status', 'in_progress'), ('path', 'another.yml')]:
            with self.subTest(key=key), self.assertRaises(ValueError):
                release.preparation_succeeded(dict(run, **{key: value}), jobs, SOURCE, 17)
        for index in range(len(steps)):
            broken = copy.deepcopy(jobs)
            broken[0]['steps'][index]['conclusion'] = 'skipped'
            with self.subTest(gate=index), self.assertRaises(ValueError):
                release.preparation_succeeded(run, broken, SOURCE, 17)

    def test_changed_candidate_cannot_inherit_prior_acceptance(self):
        with tempfile.TemporaryDirectory() as temp:
            work = Path(temp)
            (work / 'assets').mkdir()
            (work / 'signing').mkdir()
            (work / 'signing/signing-vault.exe').write_bytes(b'compiled helper fixture')
            payload = work / 'assets/panel'
            payload.write_bytes(b'original')
            receipt = dict(schema=1, tag=release.TAG, base_tag=release.BASE_TAG,
                           base_manifest_sha256=release.BASE_MANIFEST_SHA256, files=release.file_map(work / 'assets'),
                           signing_files=release.file_map(work / 'signing'))
            (work / 'BUILD-RECEIPT.json').write_bytes(release.canonical(receipt))
            release.checked_build(work)
            payload.write_bytes(b'changed')
            with self.assertRaises(ValueError):
                release.checked_build(work)

    def test_small_signing_handoff_keeps_helper_outside_release_assets(self):
        raw = release.canonical(manifest())
        with tempfile.TemporaryDirectory() as temp:
            work = Path(temp)
            (work / 'signing').mkdir()
            (work / 'assets').mkdir()
            (work / 'assets/RELEASE-MANIFEST.json').write_bytes(raw)
            for name, payload in [('signing-vault.exe', b'compiled Windows helper'),
                                  ('panel-signing-vault.ps1', b'existing wrapper'),
                                  ('SIGN-CANDIDATE.ps1', b'candidate check/sign launcher')]:
                (work / 'signing' / name).write_bytes(payload)
            info = dict(source_commit=SOURCE, draft_id=7, manifest_sha256=hashlib.sha256(raw).hexdigest(),
                        signing_files=release.file_map(work / 'signing'))
            release.signing_handoff(work, info, raw)
            instructions = (work / 'signing/SIGNING-INSTRUCTIONS.txt').read_text()
            self.assertIn('pwsh -NoProfile -File .\\SIGN-CANDIDATE.ps1 -ExpectedManifestSha256 ' + info['manifest_sha256'], instructions)
            self.assertIn('-ExpectedHelperSha256 ' + info['signing_files']['signing-vault.exe']['sha256'], instructions)
            self.assertIn('$env:LOCALAPPDATA\\Rovunet\\PanelProductionSigning-v1', instructions)
            self.assertEqual((work / 'signing/RELEASE-MANIFEST.json').read_bytes(), raw)
            self.assertEqual({p.name for p in (work / 'assets').iterdir()}, {'RELEASE-MANIFEST.json'})

    def test_existing_tag_cannot_be_moved_to_another_source(self):
        for kind, sha in [('commit', '0' * 40), ('tag', SOURCE)]:
            result = SimpleNamespace(returncode=0, stderr=b'', stdout=release.canonical({'object': {'type': kind, 'sha': sha}}))
            with patch.object(release.subprocess, 'run', return_value=result), self.assertRaises(ValueError):
                release.existing_tag(SOURCE)
        missing = SimpleNamespace(returncode=1, stderr=b'gh: Not Found (HTTP 404)', stdout=b'')
        with patch.object(release.subprocess, 'run', return_value=missing):
            release.existing_tag(SOURCE)
        unavailable = SimpleNamespace(returncode=1, stderr=b'gh: unavailable (HTTP 500)', stdout=b'')
        with patch.object(release.subprocess, 'run', return_value=unavailable), self.assertRaises(ValueError):
            release.existing_tag(SOURCE)

    def test_final_receipt_binds_signed_manifest_source_binaries_and_original_root(self):
        data = manifest()
        raw = release.canonical(data)
        good = final_receipt(data, raw)
        payload = release.canonical(good)
        release.final_acceptance(payload, hashlib.sha256(payload).hexdigest(), raw, data)
        for key, value in [('source_commit', '0' * 40), ('manifest_sha256', '0' * 64),
                           ('production_key_id', '0' * 64), ('production_signature_verified', False),
                           ('runtime_base_tag', 'v2.0.1'), ('environment', 'isolated-systemd-vm'),
                           ('panel_sha256', {}), ('upgrade_baseline', dict(tag=release.BASE_TAG, source_commit='0' * 40))]:
            changed = release.canonical(dict(good, **{key: value}))
            with self.subTest(key=key), self.assertRaises(ValueError):
                release.final_acceptance(changed, hashlib.sha256(changed).hexdigest(), raw, data)
        with self.assertRaises(ValueError):
            release.final_acceptance(payload, '0' * 64, raw, data)

    def test_unrun_or_missing_final_install_update_and_preservation_checks_block(self):
        data = manifest()
        raw = release.canonical(data)
        good = final_receipt(data, raw)
        for gate in release.FINAL_GATES:
            for changed_check in [dict(status='not_run', evidence_sha256=None),
                                  dict(status='pass', evidence_sha256='0' * 64),
                                  dict(status='skipped', evidence_sha256='4' * 64)]:
                changed = copy.deepcopy(good)
                changed['checks'][gate] = changed_check
                encoded = release.canonical(changed)
                with self.subTest(gate=gate, check=changed_check), self.assertRaises(ValueError):
                    release.final_acceptance(encoded, hashlib.sha256(encoded).hexdigest(), raw, data)
        changed = copy.deepcopy(good)
        changed['checks'].pop('signed_menu_update')
        encoded = release.canonical(changed)
        with self.assertRaises(ValueError):
            release.final_acceptance(encoded, hashlib.sha256(encoded).hexdigest(), raw, data)

    def test_final_receipt_requires_each_retained_evidence_file_unchanged(self):
        data = manifest()
        raw = release.canonical(data)
        good = final_receipt(data, raw)
        with tempfile.TemporaryDirectory() as temp:
            work = Path(temp)
            evidence = work / 'final/evidence'
            evidence.mkdir(parents=True)
            for name in release.FINAL_GATES:
                phase = 'fresh' if name == 'signed_fresh_install' else 'upgrade'
                logs = work / 'final/logs' / phase
                logs.mkdir(parents=True, exist_ok=True)
                (logs / 'fixture.log').write_bytes(b'passed fixture command')
                (logs / 'https-requests.json').write_bytes(b'[]\n')
                path = evidence / (name + '.json')
                proof = {k: good[k] for k in ('tag', 'source_commit', 'manifest_sha256', 'production_key_id', 'panel_sha256')}
                proof.update(schema=1, check=name, status='pass',
                    runner_sha256=release.digest(release.ROOT / 'scripts/test-signed-v210-update.py'),
                    simulator_sha256=release.digest(release.ROOT / 'scripts/test-complete-setup.py'),
                    scope={'environment': 'isolated-linux-namespaces', 'systemd': 'simulated'},
                    executions=[{'name': 'logs/' + phase + '/fixture.log', 'exit_code': 0,
                                 'sha256': release.digest(logs / 'fixture.log')}],
                    https_requests_sha256=release.digest(logs / 'https-requests.json'))
                path.write_bytes(release.canonical(proof))
                good['checks'][name]['evidence_sha256'] = release.digest(path)
            (work / 'FINAL-ACCEPTANCE.json').write_bytes(release.canonical(good))
            release.retained_final_acceptance(work, raw, data)
            for name in release.FINAL_GATES:
                path = evidence / (name + '.json')
                payload = path.read_bytes()
                path.unlink()
                with self.subTest(gate=name, change='missing'), self.assertRaises(ValueError):
                    release.retained_final_acceptance(work, raw, data)
                path.write_bytes(b'{}')
                with self.subTest(gate=name, change='altered'), self.assertRaises(ValueError):
                    release.retained_final_acceptance(work, raw, data)
                path.unlink()
                other = work / 'outside-evidence.json'
                other.write_bytes(payload)
                path.symlink_to(other)
                with self.subTest(gate=name, change='symlink'), self.assertRaises(ValueError):
                    release.retained_final_acceptance(work, raw, data)
                path.unlink()
                path.write_bytes(payload)
            (work / 'final/logs/fresh/fixture.log').write_bytes(b'changed execution log')
            with self.assertRaises(ValueError):
                release.retained_final_acceptance(work, raw, data)

    def test_owner_request_accepts_signature_without_acceptance_but_not_supplied_receipt(self):
        data = manifest()
        raw = release.canonical(data)
        request = publication_request(data, raw)
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / 'request.json'
            path.write_bytes(release.canonical(request))
            with patch.object(release, 'authenticate_manifest', return_value=data):
                parsed, _, _, _ = release.publication_request(path)
                self.assertEqual(parsed['source_commit'], SOURCE)
                request['final_acceptance_base64'] = base64.b64encode(release.canonical(final_receipt(data, raw))).decode()
                path.write_bytes(release.canonical(request))
                with self.assertRaises(ValueError):
                    release.publication_request(path)

    def test_signed_candidate_readonly_stage_rejects_byte_changes_and_failing_source_ci(self):
        data = manifest()
        panels = {p: (p + '\n').encode() for p in release.PANELS}
        panel_files = {p: {'size': len(payload), 'sha256': hashlib.sha256(payload).hexdigest()}
                       for p, payload in panels.items()}
        acceptance = dict(source_commit=SOURCE, tag=release.TAG, base_tag=release.BASE_TAG,
                          base_manifest_sha256=release.BASE_MANIFEST_SHA256, source_ci=19, prepare_run_id=17,
                          gates={n: dict(source_commit=SOURCE, gate=n, panels=panel_files) for n in release.GATES})
        acceptance['gates']['shadowsocks']['runtime_cases'] = {k: sorted(v) for k, v in release.RUNTIME_CASES.items()}
        acceptance['gates']['signed-baseline-harness']['baseline_smoke'] = dict(tag=release.BASE_TAG,
            source_commit=release.BASE_COMMIT, manifest_sha256=release.BASE_MANIFEST_SHA256,
            summary_sha256='4' * 64, final_v210_acceptance=False)
        payloads = dict(panels, **{'ACCEPTANCE.json': release.canonical(acceptance)})
        prototype = data['assets'][0]
        data['assets'] = [dict(prototype, name=name, size=len(payload), sha256=hashlib.sha256(payload).hexdigest())
                          for name, payload in payloads.items()]
        raw = release.canonical(data)
        signature = bytes(64)
        payloads['RELEASE-MANIFEST.json'] = raw
        draft = dict(id=7, tag_name=release.TAG, target_commitish=SOURCE, draft=True, prerelease=False,
                     assets=[dict(id=i, name=n, size=len(p), state='uploaded', digest='sha256:' + hashlib.sha256(p).hexdigest())
                             for i, (n, p) in enumerate(payloads.items(), 1)])
        source_run = dict(id=19, head_sha=SOURCE, path='.github/workflows/ci.yml', event='push', status='completed', conclusion='success')
        prepare_run = dict(id=17, head_sha=SOURCE, path=release.WORKFLOW, event='push', status='completed', conclusion='success')
        jobs = [dict(name='prepare', conclusion='success', steps=[dict(name=n, conclusion='success')
                for n in list(release.GATES.values()) + ['Create the tested unsigned draft']])]
        responses = {f'repos/{release.REPO}/releases/7': draft,
                     f'repos/{release.REPO}/actions/runs/19': source_run,
                     f'repos/{release.REPO}/actions/runs/17': prepare_run,
                     f'repos/{release.REPO}/actions/runs/17/jobs?filter=latest&per_page=100': {'jobs': jobs}}
        def readonly_api(path, **body):
            self.assertFalse(body, 'Signed staging must never mutate GitHub')
            return responses[path]
        def download(entry, path):
            path.write_bytes(payloads[entry['name']])
        verifier = SimpleNamespace(verify=lambda *args: None)
        with tempfile.TemporaryDirectory() as temp, patch.object(release, 'api', side_effect=readonly_api), \
                patch.object(release, 'download_asset', side_effect=download), \
                patch.object(release, 'load_script', return_value=verifier):
            assets = Path(temp) / 'assets'
            request = publication_request(data, raw)
            release.signed_candidate(request, data, raw, signature, assets, download=True)
            self.assertTrue((assets / 'RELEASE-MANIFEST.sig').is_file())
            release.signed_candidate(request, data, raw, signature, assets)
            source_run['conclusion'] = 'failure'
            with self.assertRaises(ValueError):
                release.signed_candidate(request, data, raw, signature, assets)
            source_run['conclusion'] = 'success'
            (assets / release.PANELS[0]).write_bytes(b'altered after acceptance')
            with self.assertRaises(ValueError):
                release.signed_candidate(request, data, raw, signature, assets)

    def test_staged_candidate_is_bound_to_current_publication_run(self):
        data = manifest()
        raw = release.canonical(data)
        request = publication_request(data, raw)
        with tempfile.TemporaryDirectory() as temp:
            work = Path(temp)
            path = work / 'request.json'
            path.write_bytes(release.canonical(request))
            (work / 'STAGED-CANDIDATE.json').write_bytes(release.canonical(release.stage_identity(request, raw, bytes(64), 99)))
            with patch.object(release, 'authenticate_manifest', return_value=data), \
                    patch.object(release, 'require_actions', return_value=100), \
                    patch.object(release, 'retained_final_acceptance'), patch.object(release, 'api') as api:
                with self.assertRaises(ValueError):
                    release.publish(path, work)
                api.assert_not_called()

    def test_missing_or_invalid_production_signature_blocks_before_github_access(self):
        raw = release.canonical(manifest())
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / 'request.json'
            request = publication_request(manifest(), raw)
            path.write_bytes(release.canonical(request))
            with patch.object(release, 'require_actions'), patch.object(release, 'api') as api:
                with self.assertRaises(ValueError):
                    release.publish(path, Path(temp) / 'output')
                api.assert_not_called()
            request['signature_base64'] = ''
            path.write_bytes(release.canonical(request))
            with self.assertRaises(ValueError):
                release.publication_request(path)
            # Even an authenticated manifest cannot bypass absent local runner evidence.
            with patch.object(release, 'authenticate_manifest', return_value=manifest()), \
                    patch.object(release, 'require_actions'), patch.object(release, 'api') as api:
                with self.assertRaises(ValueError):
                    release.publish(path, Path(temp) / 'output')
                api.assert_not_called()


if __name__ == '__main__':
    unittest.main(verbosity=2)
