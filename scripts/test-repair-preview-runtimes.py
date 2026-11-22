#!/usr/bin/env python3
"""Offline regression checks; synthetic ELF fixtures are never executed."""
import importlib.util
import io
import json
import os
import pathlib
import shutil
import stat
import struct
import subprocess
import tempfile
import unittest
from unittest import mock

SCRIPT = pathlib.Path(__file__).with_name('repair-preview-runtimes.py')
SPEC = importlib.util.spec_from_file_location('repair_preview_runtimes', SCRIPT)
repair = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(repair)


def elf(label):
    header = bytearray(64)
    header[:7] = b'\x7fELF\x02\x01\x01'
    struct.pack_into('<HHI', header, 16, 2, 62, 1)
    struct.pack_into('<H', header, 52, 64)
    return bytes(header) + label.encode('ascii')


class RepairTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.keys = tempfile.TemporaryDirectory()
        root = pathlib.Path(cls.keys.name)
        cls.private = root / 'private.pem'
        subprocess.run(['/usr/bin/openssl', 'genpkey', '-algorithm', 'ED25519',
                        '-out', str(cls.private)], check=True,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        cls.public = subprocess.check_output(['/usr/bin/openssl', 'pkey', '-in',
                                             str(cls.private), '-pubout', '-outform', 'DER'])

    @classmethod
    def tearDownClass(cls):
        cls.keys.cleanup()

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = pathlib.Path(self.temporary.name)
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        self.receipts = self.root / 'receipts'
        self.panel = elf('identified-preview-panel')
        (self.bin / 'rovunet').write_bytes(self.panel)
        (self.bin / 'rovunet').chmod(0o755)
        self.payloads = {name: (b'{"fixture":"provenance"}\n' if name.endswith('.json') else elf(name))
                         for name in set(repair.TARGETS.values())}
        self.requests = []
        self.patch('PANEL_SHA256', repair.sha256(self.panel))
        self.patch('SPKI', repair.base64.b64encode(self.public).decode('ascii'))
        self.patch('KEY_ID', repair.sha256(self.public))
        self.sign()

    def patch(self, name, value):
        change = mock.patch.object(repair, name, value)
        change.start()
        self.addCleanup(change.stop)

    def sign(self, tag=None):
        self.document = {'schema': 1, 'environment': 'production', 'key_id': repair.KEY_ID,
                         'repository': repair.REPOSITORY, 'tag': tag or repair.TAG,
                         'source_commit': repair.SOURCE, 'assets': []}
        for name, body in sorted(self.payloads.items()):
            data = name.endswith('.json')
            self.document['assets'].append({
                'name': name, 'kind': 'data' if data else 'executable',
                'os': '' if data else 'linux', 'arch': '' if data else 'amd64',
                'size': len(body), 'sha256': repair.sha256(body),
                'source': 'https://example.test/synthetic', 'source_commit': '1' * 40,
                'license': 'synthetic-test-only'})
        raw = json.dumps(self.document, sort_keys=True).encode('utf-8')
        manifest = self.root / 'fixture-manifest'
        manifest.write_bytes(raw)
        signature = subprocess.check_output(['/usr/bin/openssl', 'pkeyutl', '-sign', '-rawin',
                                             '-inkey', str(self.private), '-in', str(manifest)])
        self.metadata = {'RELEASE-MANIFEST.json': raw, 'RELEASE-MANIFEST.sig': signature}
        self.patch('MANIFEST_SHA256', repair.sha256(raw))
        self.patch('SIGNATURE_SHA256', repair.sha256(signature))

    def fetch(self, name, destination, limit):
        self.requests.append(name)
        body = self.metadata[name] if name in self.metadata else self.payloads[name]
        self.assertLessEqual(len(body), limit)
        destination.write_bytes(body)

    def run_repair(self, fetch=None):
        with mock.patch('sys.stdout', new_callable=io.StringIO):
            return repair.repair(self.bin, self.receipts, owner=os.geteuid(), fetch=fetch or self.fetch)

    def assert_no_targets(self):
        for name in repair.TARGETS:
            self.assertFalse(os.path.lexists(str(self.bin / name)), name)
        self.assertEqual((self.bin / 'rovunet').read_bytes(), self.panel)

    def test_verified_publication_idempotence_and_separate_xray_inodes(self):
        receipt = self.run_repair()
        first = json.loads(receipt.read_text())
        self.assertEqual(first['status'], 'ready')
        self.assertEqual(first['installed'], list(repair.TARGETS))
        self.assertEqual(stat.S_IMODE(receipt.stat().st_mode), 0o600)
        self.assertEqual(stat.S_IMODE(self.receipts.stat().st_mode), 0o700)
        inodes = {}
        for name, asset in repair.TARGETS.items():
            target = self.bin / name
            self.assertEqual(target.read_bytes(), self.payloads[asset])
            self.assertEqual(stat.S_IMODE(target.stat().st_mode), 0o644 if name.endswith('.json') else 0o755)
            inodes[name] = target.stat().st_ino
        self.assertNotEqual(inodes['xray'], inodes['rovunet-xray-managed'])
        self.requests.clear()
        second = json.loads(self.run_repair().read_text())
        self.assertEqual(second['installed'], [])
        self.assertEqual(second['retained'], list(repair.TARGETS))
        self.assertEqual(self.requests, ['RELEASE-MANIFEST.json', 'RELEASE-MANIFEST.sig'])
        self.assertEqual(inodes, {name: (self.bin / name).stat().st_ino for name in repair.TARGETS})
        self.assertEqual((self.bin / 'rovunet').read_bytes(), self.panel)

    def test_matching_existing_file_is_not_replaced_or_chmodded(self):
        existing = self.bin / 'xray'
        existing.write_bytes(self.payloads[repair.TARGETS['xray']])
        existing.chmod(0o700)
        inode = existing.stat().st_ino
        self.run_repair()
        self.assertEqual(existing.stat().st_ino, inode)
        self.assertEqual(stat.S_IMODE(existing.stat().st_mode), 0o700)

    def test_mismatching_existing_destination_stops_before_any_target_mutation(self):
        existing = self.bin / 'rovunet-awg-provenance.json'
        existing.write_bytes(b'operator-owned-content')
        inode = existing.stat().st_ino
        with self.assertRaises(repair.RepairError):
            self.run_repair()
        self.assertEqual(set(self.bin.iterdir()), {self.bin / 'rovunet', existing})
        self.assertEqual(existing.read_bytes(), b'operator-owned-content')
        self.assertEqual(existing.stat().st_ino, inode)
        self.assertEqual(self.requests, ['RELEASE-MANIFEST.json', 'RELEASE-MANIFEST.sig'])

    def test_symlink_destination_is_rejected_even_if_bytes_match(self):
        outside = self.root / 'external-xray'
        outside.write_bytes(self.payloads[repair.TARGETS['xray']])
        (self.bin / 'xray').symlink_to(outside)
        with self.assertRaises(repair.RepairError):
            self.run_repair()
        self.assertTrue((self.bin / 'xray').is_symlink())
        self.assertFalse((self.bin / 'rovunet-xray-managed').exists())
        self.assertEqual(outside.read_bytes(), self.payloads[repair.TARGETS['xray']])

    def test_failure_of_last_asset_download_publishes_nothing(self):
        def failing_fetch(name, destination, limit):
            if name == 'xray-linux-amd64':
                destination.write_bytes(b'truncated')
                raise repair.RepairError('simulated transfer failure')
            self.fetch(name, destination, limit)
        with self.assertRaises(repair.RepairError):
            self.run_repair(failing_fetch)
        self.assert_no_targets()

    def test_wrong_elf_architecture_with_authentic_fixture_hash_is_rejected(self):
        body = bytearray(self.payloads['xray-linux-amd64'])
        struct.pack_into('<H', body, 18, 183)
        self.payloads['xray-linux-amd64'] = bytes(body)
        self.sign()
        with self.assertRaisesRegex(repair.RepairError, 'ELF64'):
            self.run_repair()
        self.assert_no_targets()

    def test_bad_asset_digest_publishes_nothing(self):
        self.payloads['xray-linux-amd64'] = elf('wrong-content').ljust(len(self.payloads['xray-linux-amd64']), b'!')
        with self.assertRaises(repair.RepairError):
            self.run_repair()
        self.assert_no_targets()

    def test_wrong_manifest_identity_is_rejected_after_signature_verification(self):
        self.sign(tag='v2.0.3')
        with self.assertRaisesRegex(repair.RepairError, 'identity'):
            self.run_repair()
        self.assert_no_targets()

    def test_signature_crypto_is_checked_even_when_signature_digest_is_pinned(self):
        signature = b'\0' * 64
        self.metadata['RELEASE-MANIFEST.sig'] = signature
        self.patch('SIGNATURE_SHA256', repair.sha256(signature))
        with self.assertRaisesRegex(repair.RepairError, 'Ed25519'):
            self.run_repair()
        self.assert_no_targets()

    def test_changed_panel_during_download_prevents_publication(self):
        def fetch_and_change(name, destination, limit):
            self.fetch(name, destination, limit)
            if name == 'xray-linux-amd64':
                (self.bin / 'rovunet').write_bytes(elf('different-panel'))
        with self.assertRaises(repair.RepairError):
            self.run_repair(fetch_and_change)
        for name in repair.TARGETS:
            self.assertFalse((self.bin / name).exists())

    def test_link_collision_reports_partial_failure_and_rerun_resumes(self):
        real_link = os.link
        collision = self.bin / 'rovunet-xray-managed'
        def collide(source, destination, **kwargs):
            if pathlib.Path(destination) == collision:
                collision.write_bytes(b'concurrent-unrelated-writer')
            return real_link(source, destination, **kwargs)
        with mock.patch.object(repair.os, 'link', side_effect=collide):
            with self.assertRaises(repair.RepairError):
                self.run_repair()
        self.assertEqual((self.bin / 'xray').read_bytes(), self.payloads['xray-linux-amd64'])
        inode = (self.bin / 'xray').stat().st_ino
        failed = json.loads(next(self.receipts.glob('repair-*.json')).read_text())
        self.assertEqual(failed['status'], 'failed')
        self.assertEqual(failed['installed'], ['xray'])
        self.assertEqual(collision.read_bytes(), b'concurrent-unrelated-writer')
        self.assertFalse((self.bin / 'rovunet-sing-box').exists())
        collision.unlink()  # Operator resolves the conflicting external writer.
        resumed = json.loads(self.run_repair().read_text())
        self.assertEqual(resumed['status'], 'ready')
        self.assertIn('xray', resumed['retained'])
        self.assertEqual((self.bin / 'xray').stat().st_ino, inode)

    def test_disk_budget_is_checked_before_runtime_downloads(self):
        with mock.patch.object(repair.shutil, 'disk_usage', return_value=shutil._ntuple_diskusage(100, 99, 1)):
            with self.assertRaisesRegex(repair.RepairError, 'disk space'):
                self.run_repair()
        self.assertEqual(self.requests, ['RELEASE-MANIFEST.json', 'RELEASE-MANIFEST.sig'])
        self.assert_no_targets()

    def test_cli_rejects_override_flags(self):
        with mock.patch.object(repair.sys, 'argv', [str(SCRIPT), '--root', str(self.root)]):
            with self.assertRaisesRegex(repair.RepairError, 'override flags'):
                repair.main()

    def test_curl_stream_has_size_bound_https_only_and_numeric_failure(self):
        process = mock.Mock(stdout=io.BytesIO(b'12345'))
        process.poll.return_value = None
        with mock.patch.object(repair.subprocess, 'Popen', return_value=process) as run:
            with self.assertRaisesRegex(repair.RepairError, 'exceeded bound'):
                repair.download('fixture', self.root / 'download', 4)
        process.kill.assert_called_once()
        command = run.call_args[0][0]
        self.assertEqual(command[:2], ['/usr/bin/curl', '-q'])
        self.assertEqual(command[command.index('--proto') + 1], '=https')
        self.assertEqual(command[command.index('--proto-redir') + 1], '=https')
        self.assertEqual(command[command.index('--max-filesize') + 1], '4')
        self.assertNotIn('--insecure', command)
        self.assertNotIn('-k', command)
        failed = mock.Mock(stdout=io.BytesIO(b''))
        failed.wait.return_value = 60
        failed.poll.return_value = 60
        with mock.patch.object(repair.subprocess, 'Popen', return_value=failed):
            with self.assertRaisesRegex(repair.RepairError, 'curl exit 60'):
                repair.download('fixture', self.root / 'failed-download', 4)


if __name__ == '__main__':
    unittest.main(verbosity=2)
