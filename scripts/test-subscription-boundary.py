#!/usr/bin/env python3
"""Exercise actual subscription CLI path containment in private namespaces.

An optional historical binary demonstrates the regression using toy accounts
and writes only within the disposable namespace's /var/lib. No public server
or real user configuration is read or changed.
"""
import argparse
import base64
import gzip
import hashlib
import json
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile


def fixture(token):
    return dict(listen={'address': '127.0.0.1:5300'}, public_address='203.0.113.7', server_id='5e'*16,
                tunnels=[dict(tag='naive', transport='naive', backend='socks', enabled=True,
                              domain='fixture.example', naive={'port': 443})],
                backends=[dict(tag='socks', type='socks', address='127.0.0.1:1080', socks={}),
                          dict(tag='ssh', type='ssh', address='127.0.0.1:22')],
                users=[dict(username='alice', password='synthetic-fixture-password', sub_token=token,
                            account_id='ac'*16, period_id='9a'*16)], route={'mode': 'multi'})


def inside(args):
    for name in ['mnt', 'net', 'pid']:
        expected = os.environ.get('ROVUNET_LAB_PARENT_'+name.upper())
        if not expected or expected == os.readlink('/proc/self/ns/'+name):
            raise RuntimeError('Disposable namespaces are required')
    subprocess.run(['mount', '--make-rprivate', '/'], check=True)
    for name, dest in [('etc', '/etc'), ('lib', '/var/lib')]:
        path = args.work/name
        path.mkdir()
        subprocess.run(['mount', '--bind', str(path), dest], check=True)
    config = pathlib.Path('/etc/rovunet')
    config.mkdir()
    root = pathlib.Path('/var/lib/rovunet/subscriptions')
    results = []
    for label, panel in [('historical', args.baseline), ('review', args.panel)]:
        if panel is None:
            continue
        row = dict(binary_case=label, sha256=hashlib.sha256(panel.read_bytes()).hexdigest())
        for token, key in [('a1'*16, 'valid_control'), ('../escaped-fixture', 'unsafe_token')]:
            if root.exists():
                shutil.rmtree(root)
            cfg = fixture(token)
            (config/'config.json').write_text(json.dumps(cfg))
            refresh = subprocess.run([str(panel), 'subscriptions-refresh'], capture_output=True, timeout=15)
            bundle = dict(format=1, created='2026-11-12T00:00:00Z', version='synthetic-fixture', config=cfg, files={})
            raw = base64.b64encode(gzip.compress(json.dumps(bundle).encode())).decode()
            backup = args.work/'input.backup'
            backup.write_text('-----BEGIN ROVUNET BACKUP-----\n'+raw+'\n-----END ROVUNET BACKUP-----\n')
            check = subprocess.run([str(panel), 'backup-check', str(backup)], capture_output=True, timeout=15)
            row[key] = dict(refresh_exit=refresh.returncode, backup_check_exit=check.returncode,
                            outside_sub_file_written=(root/'escaped-fixture.json').exists(),
                            expected_feed_exists=(root/'sub'/(token+'.json')).is_file() if key == 'valid_control' else None)
        good, bad = row['valid_control'], row['unsafe_token']
        if good['refresh_exit'] or good['backup_check_exit'] or not good['expected_feed_exists']:
            raise RuntimeError('Valid control input failed: '+label)
        if label == 'review' and (not bad['refresh_exit'] or not bad['backup_check_exit'] or bad['outside_sub_file_written']):
            raise RuntimeError('Unsafe subscription filename reached the review writer/restore')
        if label == 'historical' and (bad['refresh_exit'] or bad['backup_check_exit'] or not bad['outside_sub_file_written']):
            raise RuntimeError('Claimed historical regression was not reproduced')
        results.append(row)
    print(json.dumps(dict(scope='synthetic accounts; private mount/network/PID namespaces; no live host paths changed', results=results), indent=2))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--panel', type=pathlib.Path, required=True)
    parser.add_argument('--baseline', type=pathlib.Path)
    parser.add_argument('--work', type=pathlib.Path, required=True)
    parser.add_argument('--inside', action='store_true', help=argparse.SUPPRESS)
    args = parser.parse_args()
    args.panel = args.panel.resolve()
    if args.baseline:
        args.baseline = args.baseline.resolve()
    if os.geteuid() != 0:
        parser.error('Root in a dedicated namespace-capable Linux lab is required')
    if args.inside:
        inside(args)
    else:
        args.work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(prefix='subscription-boundary-', dir=args.work) as directory:
            env = dict(os.environ, **{'ROVUNET_LAB_PARENT_'+n.upper(): os.readlink('/proc/self/ns/'+n) for n in ('mnt', 'net', 'pid')})
            command = ['unshare', '--mount', '--net', '--pid', '--fork', '--kill-child','--mount-proc', sys.executable,
                       str(pathlib.Path(__file__).resolve()), '--inside', '--panel', str(args.panel), '--work', directory]
            if args.baseline:
                command += ['--baseline', str(args.baseline)]
            subprocess.run(command, env=env, check=True)


if __name__ == '__main__':
    main()
