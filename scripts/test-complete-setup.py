#!/usr/bin/env python3
"""Actual CLI/engines in private mount+network namespaces; systemd is simulated.

Only synthetic accounts/certificates are used. No host services, routes or
firewall rules are changed. Run as root on a disposable Linux build host.
"""
import argparse, hashlib, json, os, pathlib, shlex, shutil, signal, socket, ssl, subprocess, sys, tempfile, threading, time

SYSTEMCTL = r'''#!/usr/bin/python3
import json,os,pathlib,shlex,signal,subprocess,sys,time
root=pathlib.Path(os.environ['COMPLETE_TEST_STATE'])
args=sys.argv[1:]
action=args[0]
units=[x for x in args[1:] if not x.startswith('-')]
name=units[0] if units else ''
if name and not name.endswith(('.timer','.service')):name+='.service'
with (root/'calls').open('a') as f:f.write(action+' '+name+'\n')
pidfile=root/(name+'.pid')
def active():
    if name.endswith('.timer'):return True
    if not pidfile.exists():return False
    try:os.kill(int(pidfile.read_text()),0);return True
    except ProcessLookupError:return False
if action=='is-active':print('active' if active() else 'inactive');sys.exit(0 if active() else 3)
if action=='is-enabled':print('enabled');sys.exit(0)
if action=='show':
    value='active' if active() else 'inactive'
    if '--value' in args:print(value)
    else:print('Id='+name+'\nActiveState='+value+'\nUnitFileState=enabled\nMainPID='+(pidfile.read_text() if pidfile.exists() else '0'))
    sys.exit(0)
if action in ('daemon-reload','enable','disable','reset-failed'):sys.exit(0)
if action=='start':
    if name.endswith('.timer') or active():sys.exit(0)
    unit=pathlib.Path('/etc/systemd/system')/name
    settings={};environment=dict(os.environ)
    for line in unit.read_text().splitlines():
        k,sep,v=line.partition('=')
        if sep:
            settings[k]=v
            if k=='Environment':
                for entry in shlex.split(v):
                    key,_,value=entry.partition('=');environment[key]=value
    if settings.get('ExecCondition'):subprocess.run(shlex.split(settings['ExecCondition']),check=True)
    log=(root/(name+'.log')).open('ab')
    command=shlex.split(settings['ExecStart'])
    if settings.get('StateDirectory'):
        directory=pathlib.Path('/var/lib')/settings['StateDirectory'];directory.mkdir(mode=0o700,exist_ok=True)
        pathlib.Path('/var/lib/mita').mkdir(exist_ok=True)
        command=['unshare','--mount','sh','-c','mount --make-rprivate /; mount --bind "$1" /var/lib/mita; shift; exec "$@"','mita-test',str(directory)]+command
    child=subprocess.Popen(command,env=environment,stdout=log,stderr=log,start_new_session=True)
    pidfile.write_text(str(child.pid));log.close()
    if settings.get('ExecStartPost'):subprocess.run(shlex.split(settings['ExecStartPost']),check=True)
    sys.exit(0)
if action=='stop':
    if pidfile.exists():
        try:os.kill(int(pidfile.read_text()),signal.SIGTERM)
        except ProcessLookupError:pass
    sys.exit(0)
raise SystemExit('Unexpected systemctl operation: '+action+' '+name)
'''

def run(*args,**kw):
    result=subprocess.run(list(args),stdout=subprocess.PIPE,stderr=subprocess.STDOUT,**kw)
    if result.returncode:
        print(result.stdout.decode(errors='replace'),flush=True)
        raise subprocess.CalledProcessError(result.returncode,list(args),output=result.stdout)
    return result

def feeds():
    root=pathlib.Path('/var/lib/rovunet/subscriptions/sub')
    result={}
    for path in root.glob('*.json'):
        data=json.loads(path.read_text())
        if isinstance(data.get('items'),list) and not path.name.endswith('.trust.json'):result['windows' if path.name.endswith('.windows.json') else 'android']=(path.name,data)
    return result

def snapshot(root):
    return {str(p.relative_to(root)):p.read_bytes() for p in root.rglob('*') if p.is_file()}

def child(a):
    for name in ('mnt','net','pid'):
        parent=os.environ.get('ROVUNET_COMPLETE_PARENT_'+name.upper())
        if not parent or parent==os.readlink('/proc/self/ns/'+name):
            raise RuntimeError('Private mount/network/PID namespaces are required')
    work=pathlib.Path(a.work)
    # Preserve resolved tools: binding /etc hides Debian alternatives symlinks.
    nettools={name:str(pathlib.Path(shutil.which(name)).resolve()) for name in ('iptables','iptables-save','iptables-restore') if shutil.which(name)}
    state=work/'systemd';state.mkdir()
    run('mount','--make-rprivate','/')
    for name,target in [('etc','/etc'),('varlib','/var/lib'),('bin','/usr/local/bin'),('root','/root')]:
        source=work/name;source.mkdir()
        run('mount','--bind',str(source),target)
    os.environ['COMPLETE_TEST_STATE']=str(state)
    fake=work/'fake-bin';fake.mkdir();(fake/'systemctl').write_text(SYSTEMCTL);(fake/'systemctl').chmod(0o755)
    for name,target in nettools.items():(fake/name).symlink_to(target)
    os.environ['PATH']=str(fake)+':'+os.environ['PATH']
    os.environ['SSL_CERT_FILE']=str(work/'ca.crt')
    os.environ['SSL_CERT_DIR']=str(work/'empty-ca-directory')
    pathlib.Path('/etc/systemd/system').mkdir(parents=True)
    pathlib.Path('/etc/rovunet').mkdir()
    shutil.copytree(work/'fixture/core','/etc/rovunet/core')
    shutil.copyfile(work/'fixture/config.json','/etc/rovunet/config.json')
    for source,target in [(a.panel,'rovunet'),(a.xray,'rovunet-xray-managed'),(a.engine,'rovunet-amneziawg-go'),(a.tool,'rovunet-awg'),(a.sing_box,'rovunet-sing-box'),(a.mita,'rovunet-mita')]:
        shutil.copyfile(source,'/usr/local/bin/'+target);os.chmod('/usr/local/bin/'+target,0o755)
    # A deliberately incompatible legacy runtime must remain untouched.
    legacy=b'#!/bin/sh\necho legacy-fixture-runtime\nexit 0\n'
    pathlib.Path('/usr/local/bin/xray').write_bytes(legacy);os.chmod('/usr/local/bin/xray',0o755)
    arch=os.uname().machine
    assert arch=='x86_64'
    records=[]
    for asset,path,commit in [('amneziawg-go','rovunet-amneziawg-go','b5928efb6ca19f0153958460c3d141f04abc5c2e'),('awg','rovunet-awg','ee0f0a9aa34ff0a0da4b3433b9512781cfe02843')]:
        records.append(dict(asset=asset+'-linux-amd64',commit=commit,sha256=hashlib.sha256(pathlib.Path('/usr/local/bin',path).read_bytes()).hexdigest()))
    pathlib.Path('/usr/local/bin/rovunet-awg-provenance.json').write_text(json.dumps({'engines':records}))
    certdir=pathlib.Path('/root/.local/share/caddy/certificates/fixture/n.example.com');certdir.mkdir(parents=True)
    shutil.copyfile(work/'leaf.crt',certdir/'n.example.com.crt');shutil.copyfile(work/'leaf.key',certdir/'n.example.com.key')
    run('ip','link','set','lo','up')
    # Real TLS 1.3 + h2 target inside this private namespace; no WAN exception.
    pathlib.Path('/etc/hosts').write_text('127.0.0.1 localhost www.cloudflare.com\n')
    tls=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER);tls.minimum_version=ssl.TLSVersion.TLSv1_3
    tls.load_cert_chain(str(work/'leaf.crt'),str(work/'leaf.key'));tls.set_alpn_protocols(['h2'])
    target=socket.socket();target.bind(('127.0.0.1',443));target.listen()
    def target_loop():
        while True:
            try:
                conn,_=target.accept()
                try:
                    with tls.wrap_socket(conn,server_side=True):pass
                except (ssl.SSLError,OSError):conn.close()
            except OSError:return
    threading.Thread(target=target_loop,daemon=True).start()
    run('ip','link','add','complete0','type','dummy')
    run('ip','addr','add','198.51.100.1/24','dev','complete0')
    run('ip','link','set','complete0','up')
    run('ip','route','add','default','dev','complete0')
    processes=[]
    try:
        old=pathlib.Path('/etc/rovunet/core')
        old_names=sorted(p.name for p in old.iterdir() if p.is_dir())
        for name in old_names:
            p=subprocess.Popen(['/bin/sleep','240']);processes.append(p)
            (state/('rovunet-core-'+name+'.service.pid')).write_text(str(p.pid))
        run('/usr/local/bin/rovunet','subscriptions-refresh')
        before=feeds();assert len(before['android'][1]['items'])==20
        old_files=snapshot(old)
        config_before=pathlib.Path('/etc/rovunet/config.json').read_bytes()
        output=run('/usr/local/bin/rovunet','complete-setup').stdout.decode()
        (work/'first-setup.log').write_text(output)
        after=feeds()
        assert len(after['android'][1]['items'])==30 and len(after['windows'][1]['items'])==30
        for platform in before:assert before[platform][0]==after[platform][0]
        links={item['id']:item['link'] for item in after['android'][1]['items']}
        for item in before['android'][1]['items']:assert links[item['id']]==item['link']
        assert config_before==pathlib.Path('/etc/rovunet/config.json').read_bytes()
        for path,data in old_files.items():assert (old/path).read_bytes()==data
        assert pathlib.Path('/usr/local/bin/xray').read_bytes()==legacy
        for p in processes:assert p.poll() is None
        backups=list(pathlib.Path('/etc/rovunet/setup-backups').glob('*.backup'));assert len(backups)==1
        run('/usr/local/bin/rovunet','backup-check',str(backups[0]))
        new_files=snapshot(old)
        run('/usr/local/bin/rovunet','complete-setup')
        assert snapshot(old)==new_files
        assert len(list(pathlib.Path('/etc/rovunet/setup-backups').glob('*.backup')))==1
        calls=(state/'calls').read_text().splitlines()
        for name in old_names:
            assert 'start rovunet-core-'+name+'.service' not in calls
            assert 'stop rovunet-core-'+name+'.service' not in calls
            assert 'restart rovunet-core-'+name+'.service' not in calls
        print(json.dumps({'actual_panel_cli':True,'actual_xray_and_awg_processes':True,'systemd':'simulated','host_modified':False,
            'android_profiles':30,'windows_profiles':30,'old_urls_profiles_credentials_core_files':'preserved',
            'legacy_runtime_and_processes':'preserved','private_backup_validation':'PASS','idempotent_retry':'PASS'},indent=2))
    finally:
        pids=[p.pid for p in processes]
        pids += [int(p.read_text()) for p in state.glob('*.pid')]
        for pid in set(pids):
            try:os.kill(pid,signal.SIGTERM)
            except ProcessLookupError:pass
        time.sleep(.5)
        for pid in set(pids):
            try:os.kill(pid,signal.SIGKILL)
            except ProcessLookupError:pass

def main():
    p=argparse.ArgumentParser()
    for name in ('panel','xray','engine','tool','sing-box','mita','work'):p.add_argument('--'+name,required=True)
    p.add_argument('--fixture-test-binary')
    p.add_argument('--child',action='store_true');a=p.parse_args()
    for name in ('panel','xray','engine','tool','sing_box','mita','work'):setattr(a,name,str(pathlib.Path(getattr(a,name)).resolve()))
    for name in ('panel','xray','engine','tool','sing_box','mita'):
        if not pathlib.Path(getattr(a,name)).is_file():raise SystemExit('Missing input binary: '+name)
    if os.geteuid()!=0:raise SystemExit('This test needs root for isolated namespaces')
    if a.child:return child(a)
    root=pathlib.Path(a.work);root.mkdir(parents=True,exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='complete-',dir=root) as directory:
        work=pathlib.Path(directory);fixture=work/'fixture';fixture.mkdir()
        run('openssl','req','-x509','-newkey','rsa:2048','-nodes','-keyout',str(work/'ca.key'),'-out',str(work/'ca.crt'),'-subj','/CN=Rovunet-test-only-CA','-days','2')
        run('openssl','req','-newkey','rsa:2048','-nodes','-keyout',str(work/'leaf.key'),'-out',str(work/'leaf.csr'),'-subj','/CN=n.example.com')
        (work/'ext').write_text('subjectAltName=DNS:n.example.com,DNS:www.cloudflare.com\nextendedKeyUsage=serverAuth\nbasicConstraints=CA:FALSE\n')
        run('openssl','x509','-req','-in',str(work/'leaf.csr'),'-CA',str(work/'ca.crt'),'-CAkey',str(work/'ca.key'),'-CAcreateserial','-out',str(work/'leaf.crt'),'-days','1','-extfile',str(work/'ext'))
        env=dict(os.environ,ROVUNET_TEST_COMPLETE_FIXTURE=str(fixture),ROVUNET_TEST_COMPLETE_CERT=str(work/'leaf.crt'),ROVUNET_TEST_COMPLETE_KEY=str(work/'leaf.key'))
        if a.fixture_test_binary:
            run(str(pathlib.Path(a.fixture_test_binary).resolve()),'-test.run','^TestExportCompleteCLIFixture$',env=env)
        else:run('go','test','-count=1','-run','^TestExportCompleteCLIFixture$','./internal/handlers',env=env)
        argv=[sys.executable,str(pathlib.Path(__file__).resolve()),'--child','--work',str(work)]
        for name in ('panel','xray','engine','tool','sing_box','mita'):argv+=['--'+name.replace('_','-'),getattr(a,name)]
        env=dict(os.environ,**{'ROVUNET_COMPLETE_PARENT_'+n.upper():os.readlink('/proc/self/ns/'+n) for n in ('mnt','net','pid')})
        result=subprocess.run(['unshare','--mount','--net','--pid','--mount-proc','--kill-child','--fork']+argv,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,env=env)
        (root/'complete-setup-integration.log').write_bytes(result.stdout)
        print(result.stdout.decode())
        if result.returncode:
            for file in sorted((work/'systemd').glob('*.log')):print(file.name+'\n'+file.read_text(errors='replace')[-4000:])
            if (work/'first-setup.log').exists():print((work/'first-setup.log').read_text())
            raise SystemExit(result.returncode)

if __name__=='__main__':main()
