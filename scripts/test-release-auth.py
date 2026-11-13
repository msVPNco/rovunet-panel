#!/usr/bin/env python3
"""Local ephemeral authority tests; never modifies the shipping trust root."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('release_verification', ROOT/'scripts/verify-release.py')
verify = importlib.util.module_from_spec(spec)
spec.loader.exec_module(verify)


class ReleaseAuthTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='rovunet-test-authority-')
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.key = self.root/'test.private.pem'
        subprocess.run(['openssl','genpkey','-algorithm','Ed25519','-out',str(self.key)], check=True, capture_output=True)
        self.key.chmod(0o600)
        self.assets = self.root/'assets'
        self.assets.mkdir()
        (self.assets/'fixture.txt').write_text('public synthetic data\n')
        self.commit = 'a'*40
        identity = verify.signing.sign(self.assets,self.key,'v1.21.2',self.commit,True)
        self.ring = dict(schema=1,keys=[identity])

    def test_test_authority_is_rejected_by_production(self):
        with self.assertRaises(ValueError):
            verify.verify(self.assets,'v1.21.2',self.commit,self.ring)
        ring = json.loads((ROOT/'internal/releaseauth/production-keys.json').read_text())
        with self.assertRaises(ValueError):
            verify.verify(self.assets,'v1.21.2',self.commit,ring)

    def test_signed_data_verified_then_replacement_rejected(self):
        verify.verify(self.assets,'v1.21.2',self.commit,self.ring,'test')
        (self.assets/'fixture.txt').write_text('replaced')
        with self.assertRaises(ValueError):
            verify.verify(self.assets,'v1.21.2',self.commit,self.ring,'test')

    def test_unsigned_extra_and_wrong_commit_rejected(self):
        with self.assertRaises(ValueError):
            verify.verify(self.assets,'v1.21.2','b'*40,self.ring,'test')
        (self.assets/'extra.txt').write_text('unsigned')
        with self.assertRaises(ValueError):
            verify.verify(self.assets,'v1.21.2',self.commit,self.ring,'test')

    def test_tampered_signature_rejected(self):
        sig = self.assets/'RELEASE-MANIFEST.sig'
        data = bytearray(sig.read_bytes()); data[0] ^= 1; sig.write_bytes(data)
        with self.assertRaises(ValueError):
            verify.verify(self.assets,'v1.21.2',self.commit,self.ring,'test')

    def test_candidate_archive_is_data_with_component_license_terms(self):
        entry = verify.signing.metadata('RUNTIME-CANDIDATES-2026-11-13.tar.gz', self.commit)
        self.assertEqual(entry['kind'], 'data')
        self.assertEqual(entry['arch'], '')
        self.assertEqual(entry['source_commit'], self.commit)
        self.assertEqual(entry['license'], 'Multiple; see included license texts')


if __name__ == '__main__':
    unittest.main()
