#!/usr/bin/env python3
"""Validate the owner's bounded functional draft; never certify open full gates."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

ROOT=Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser(description=__doc__)
p.add_argument('--tag',required=True)
a=p.parse_args()
scope=json.loads((ROOT/'docs/releases/v1.21.2-scope.json').read_text())
gates=json.loads((ROOT/'docs/release-gates.json').read_text())
version=subprocess.check_output(['make','-s','print-version'],cwd=ROOT,text=True).strip()
if scope['schema']!=1 or scope['tag']!=a.tag or a.tag!='v'+version or scope['profile']!='owner-authorized-functional-update':
    p.exit(1,'No matching owner-authorized functional release scope.\n')
if not scope.get('authorization') or not scope.get('excluded') or scope['full_qualification'].startswith('verified'):
    p.exit(1,'Missing explicit scope/limits; full qualification must not be fabricated.\n')
for name,digest in scope['unchanged_files_sha256'].items():
    if hashlib.sha256((ROOT/name).read_bytes()).hexdigest()!=digest:
        p.exit(1,'Runtime/account policy changed outside this accepted scope.\n')
ring=json.loads((ROOT/'internal/releaseauth/production-keys.json').read_text())
if not any(k.get('id')=='885d8aba22365ce42e20527bfa00e82aed3cba2f69501f146ed0496a8bc3dcbe' and k.get('purpose')=='production' for k in ring['keys']):
    p.exit(1,'Reviewed production identity is unavailable.\n')
if len(gates['gates'])!=4:
    p.exit(1,'Full qualification inventory changed; review required.\n')
print(a.tag+' functional draft scope checked; '+str(sum(g['status']!='verified' for g in gates['gates']))+' full qualification gates remain open.')
print('Publication requires final exact-byte acceptance and Windows-local signing; this command does not certify or publish a release.')
