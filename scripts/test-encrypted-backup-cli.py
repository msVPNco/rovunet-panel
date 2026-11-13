#!/usr/bin/env python3
"""Actual encrypted backup/restore CLI in private namespaces with toy state.

OS accounts, service control and ownership commands are simulated. This proves
crypto/state/exit behavior, not a full host/service or private-key recovery VM.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile


def inside(args):
    for name in ('mnt','net','pid'):
        parent=os.environ.get('ROVUNET_BACKUP_PARENT_'+name.upper())
        if not parent or parent==os.readlink('/proc/self/ns/'+name):
            raise RuntimeError('Private mount/network/PID namespaces required')
    subprocess.run(['mount','--make-rprivate','/'],check=True)
    for name,target in [('etc','/etc'),('lib','/var/lib'),('root','/root'),('bin','/usr/local/bin'),('home','/home')]:
        source=args.work/name;source.mkdir()
        subprocess.run(['mount','--bind',str(source),target],check=True)
    fake=args.work/'fakebin';fake.mkdir()
    scripts={
        'id':'#!/bin/sh\n[ "$1" = rovunet ]\n',
        'getent':'''#!/bin/sh
if [ "$ROVUNET_TEST_BREAK_USAGE" = 1 ] && [ -d /var/lib/rovunet/usage ]; then
    /bin/mv /var/lib/rovunet/usage /var/lib/rovunet/usage.before
    : > /var/lib/rovunet/usage
fi
exit 0
''',
        'chown':'#!/bin/sh\nexit 0\n',
        'systemctl':'#!/bin/sh\n[ "$1" != is-active ]\n',
    }
    for name in ('useradd','userdel','usermod','groupadd','groupdel','groupmod','chpasswd','pkill','iptables','ip6tables'):
        scripts[name]='#!/bin/sh\nexit 0\n'
    for name,text in scripts.items():
        p=fake/name;p.write_text(text);p.chmod(0o755)
    env=dict(os.environ,PATH=str(fake)+':'+os.environ['PATH'],ROVUNET_TEST_BREAK_USAGE='0')
    cfg=Path('/etc/rovunet/config.json');cfg.parent.mkdir(parents=True)
    Path('/etc/systemd/system').mkdir(parents=True)
    config=dict(listen=dict(address='127.0.0.1:5300'),tunnels=[],backends=[],route=dict(mode='multi'),
                public_address='192.0.2.1',server_id='a1'*16,
                users=[dict(username='fixturealice',password='synthetic-private-password',quota_gb=1,
                            sub_token='b2'*16,account_id='c3'*16,period_id='d4'*16,usage_base=17)])
    cfg.write_text(json.dumps(config));cfg.chmod(0o600)
    usage=Path('/var/lib/rovunet/usage');usage.mkdir(parents=True)
    history=b'{"epoch":"synthetic","value":42}\n'
    (usage/'xray-fixture.source').write_bytes(history)
    (usage/'vless.json').write_text('{"fixturealice":1234}\n')
    results=[]

    def call(*cmd,expected=True,overrides=None):
        result=subprocess.run([str(args.panel),*map(str,cmd)],input=b'',capture_output=True,
                              env=dict(env,**(overrides or {})),timeout=30)
        if (result.returncode==0)!=expected:
            raise RuntimeError('Backup CLI result did not match expected success/failure; private output withheld')
        return result

    def snapshot():
        return {str(p):hashlib.sha256(p.read_bytes()).hexdigest()
                for base in (Path('/etc/rovunet'),Path('/etc/slipgate'),Path('/var/lib/rovunet'))
                for p in base.rglob('*') if p.is_file()}

    identity=args.work/'identity.agekey';wrong=args.work/'wrong.agekey'
    recipient=call('backup-keygen','--identity-file',identity).stdout.decode().strip()
    call('backup-keygen','--identity-file',wrong)
    if identity.stat().st_mode&0o077:raise RuntimeError('Private identity permissions')
    cipher=args.work/'backup.age'
    call('backup','--file',cipher,'--recipient',recipient)
    data=cipher.read_bytes()
    if not data.startswith(b'-----BEGIN AGE ENCRYPTED FILE-----') or b'synthetic-private-password' in data:
        raise RuntimeError('Encrypted default export exposed plaintext')
    call('backup-check',cipher,'--identity-file',identity)
    baseline=snapshot()
    for label,file,key in [('wrong-key',cipher,wrong),('truncated',args.work/'truncated.age',identity),
                           ('tampered',args.work/'tampered.age',identity)]:
        if label=='truncated':file.write_bytes(data[:len(data)//2])
        if label=='tampered':
            lines=data.decode().splitlines()
            for i in range(len(lines)-3,1,-1):
                if len(lines[i])>8:
                    lines[i]=('A' if lines[i][0]!='A' else 'B')+lines[i][1:];break
            file.write_text('\n'.join(lines)+'\n')
        call('restore','--file',file,'--identity-file',key,'--yes',expected=False)
        if snapshot()!=baseline:raise RuntimeError('Unauthenticated restore changed private state')
        results.append(dict(case=label,rejected_before_mutation=True))
    # A legacy-only layout used to be migrated by Cobra before decryption.
    # This must fail at the crypto boundary, without starting migration.
    legacy=Path('/etc/slipgate')
    cfg.parent.rename(legacy)
    legacy_before=snapshot()
    bad=call('restore','--file',cipher,'--identity-file',wrong,'--yes',expected=False)
    if b'backup could not be decrypted' not in bad.stdout+bad.stderr or snapshot()!=legacy_before or cfg.parent.exists():
        raise RuntimeError('Invalid legacy-layout restore started migration or bypassed authentication ordering')
    bad=call('backup-check',cipher,'--identity-file',wrong,expected=False)
    if b'backup could not be decrypted' not in bad.stdout+bad.stderr:
        raise RuntimeError('Read-only backup validation attempted legacy migration')
    call('backup-keygen','--identity-file',args.work/'legacy-readonly.agekey')
    call('--version')
    if snapshot()!=legacy_before or cfg.parent.exists():
        raise RuntimeError('Read-only CLI command migrated the server')
    results.append(dict(case='legacy-auth-before-migration',invalid_restore_unchanged=True,readonly_commands_unchanged=True))
    # Correctly authenticated and explicitly confirmed restore may migrate.
    call('restore','--file',cipher,'--identity-file',identity,'--yes')
    if legacy.exists() or not cfg.exists():
        raise RuntimeError('Authenticated confirmed legacy restoration did not finish')
    results.append(dict(case='authenticated-legacy-restore',migration_completed=True))
    changed=dict(config,server_id='e5'*16);cfg.write_text(json.dumps(changed))
    for p in usage.iterdir():p.unlink()
    call('restore','--file',cipher,'--identity-file',identity,'--yes')
    restored=json.loads(cfg.read_text())
    if restored['users']!=config['users'] or restored['server_id']!=config['server_id']:
        raise RuntimeError('Account/password/token/identity changed in roundtrip')
    if (usage/'xray-fixture.source').read_bytes()!=history:
        raise RuntimeError('Usage source history changed')
    totals=sum(json.loads(p.read_text()).get('fixturealice',0) for p in usage.glob('*.json'))
    if totals!=1234:raise RuntimeError('Usage total lost or duplicated')
    results.append(dict(case='authenticated-restore',account_and_token_preserved=True,usage_total=1234,history_preserved=True))
    partial=call('restore','--file',cipher,'--identity-file',identity,'--yes',expected=False,
                 overrides={'ROVUNET_TEST_BREAK_USAGE':'1'})
    if b'Restore complete:' in partial.stdout or b'recovery is incomplete' not in partial.stdout+partial.stderr:
        raise RuntimeError('Partial restore falsely reported complete')
    results.append(dict(case='counter-storage-failure',nonzero_exit=True,false_completion=False))
    print(json.dumps(dict(scope='actual crypto/backup CLI; synthetic state; simulated OS/service commands; private namespaces',
                          panel_sha256=hashlib.sha256(args.panel.read_bytes()).hexdigest(),results=results),indent=2))


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--panel',type=Path,required=True)
    p.add_argument('--work',type=Path,required=True)
    p.add_argument('--inside',action='store_true',help=argparse.SUPPRESS)
    args=p.parse_args();args.panel=args.panel.resolve()
    if os.geteuid()!=0:p.error('Use an authorized dedicated Linux lab')
    if args.inside:return inside(args)
    args.work.mkdir(parents=True,exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='backup-cli-',dir=args.work) as directory:
        env=dict(os.environ,**{'ROVUNET_BACKUP_PARENT_'+n.upper():os.readlink('/proc/self/ns/'+n) for n in ('mnt','net','pid')})
        subprocess.run(['unshare','--mount','--net','--pid','--fork','--kill-child','--mount-proc',
                        sys.executable,str(Path(__file__).resolve()),'--inside','--panel',str(args.panel),
                        '--work',directory],env=env,check=True)


if __name__=='__main__':
    main()
