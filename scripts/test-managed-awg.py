#!/usr/bin/env python3
"""Root-only integration test in disposable Linux network namespaces.

No host routes, firewall rules or existing interfaces are changed. The test
executes the production RunAWG/SyncAWG functions and official pinned binaries.
"""
import argparse, hashlib, json, os, pathlib, signal, subprocess, tempfile, time, uuid

parser=argparse.ArgumentParser()
for flag in ('panel','helper','engine','tool','work'): parser.add_argument('--'+flag,required=True)
a=parser.parse_args()
if os.geteuid()!=0: raise SystemExit('run inside a disposable Linux test host as root')
tag='rv'+uuid.uuid4().hex[:7]
names={k:tag+k for k in ('s','c','t')}
created=[]; processes=[]; logs=[]
def run(*args, ns=None, check=True, **kwargs):
    command=(['ip','netns','exec',names[ns]] if ns else [])+list(args)
    return subprocess.run(command,check=check,stdout=subprocess.PIPE,stderr=subprocess.PIPE,**kwargs)
def spawn(args,ns,**kwargs):
    p=subprocess.Popen(['ip','netns','exec',names[ns]]+args,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,**kwargs)
    processes.append(p);return p
def stop(p):
    if p.poll() is None:
        p.send_signal(signal.SIGTERM)
        try:p.wait(4)
        except subprocess.TimeoutExpired:p.kill();p.wait()
def link(left,right,ln,rn,lip,rip):
    run('ip','link','add',ln,'type','veth','peer','name',rn)
    run('ip','link','set',ln,'netns',names[left]);run('ip','link','set',rn,'netns',names[right])
    for ns,dev,ip in ((left,ln,lip),(right,rn,rip)):
        run('ip','addr','add',ip,'dev',dev,ns=ns);run('ip','link','set',dev,'up',ns=ns)
def fetch(address, expected=True):
    p=run('curl','--silent','--fail','--max-time','4','--interface',address,'http://192.0.2.2:18080/payload',ns='c',check=False)
    if expected and (p.returncode!=0 or hashlib.sha256(p.stdout).digest()!=hashlib.sha256(payload).digest()):raise RuntimeError('AWG data round trip failed')
    if not expected and p.returncode==0:raise RuntimeError('revoked AWG peer still passed new data')
try:
    with tempfile.TemporaryDirectory(prefix='awg-',dir=a.work) as tmp:
        tmp=pathlib.Path(tmp)
        for ns in names.values():run('ip','netns','add',ns);created.append(ns)
        for ns in names:run('ip','link','set','lo','up',ns=ns)
        link('s','c','si','ci','198.51.100.1/30','198.51.100.2/30')
        link('s','t','so','to','192.0.2.1/30','192.0.2.2/30')
        core_root=tmp/'core';core_root.mkdir();core=tag+'a';directory=core_root/core
        run(a.panel,'core-plan','--protocol','amneziawg','--host','198.51.100.1','--port','51840','--devices','2','--subnet','10.203.72.0/24','--egress','so','--output',str(directory))
        env=dict(os.environ,TEST_CORE_ROOT=str(core_root),TEST_AWG_GO=a.engine,TEST_AWG_TOOL=a.tool)
        server=spawn([a.helper,'run',core],ns='s',env=env)
        time.sleep(.5)
        if server.poll() is not None:raise RuntimeError('managed AWG server exited during initialization')
        payload=b'rovunet-awg-namespace-payload\n'*8192
        target=tmp/'http';target.mkdir();(target/'payload').write_bytes(payload)
        spawn(['python3','-m','http.server','18080','--bind','192.0.2.2','--directory',str(target)],ns='t')
        clients=[]
        for n,file in enumerate(sorted(directory.glob('device-*.conf')),1):
            iface=tag+str(n);text=file.read_text();address=None;lines=[]
            for line in text.splitlines():
                k,sep,v=line.partition('=');k=k.strip().lower()
                if k=='address':address=v.strip().split('/')[0]
                if k not in ('address','dns','mtu'):lines.append(line)
            conf=tmp/(iface+'.conf');conf.write_text('\n'.join(lines)+'\n');conf.chmod(0o600)
            spawn([a.engine,'-f',iface],ns='c')
            for attempt in range(40):
                if run(a.tool,'show',iface,'listen-port',ns='c',check=False).returncode==0:break
                time.sleep(.05)
            run(a.tool,'setconf',iface,str(conf),ns='c')
            run('ip','addr','add',address+'/32','dev',iface,ns='c')
            run('ip','link','set',iface,'mtu','1280','up',ns='c')
            table=str(200+n)
            run('ip','route','add','192.0.2.2/32','dev',iface,'table',table,ns='c')
            run('ip','rule','add','from',address,'table',table,ns='c')
            clients.append(address)
        if len(clients)!=2:raise RuntimeError('expected exactly two generated client files')
        time.sleep(.2)
        for address in clients:fetch(address)
        print('PASS: two isolated peers passed payload through userspace AWG, forwarding and scoped NAT')
        manifest=directory/'server.json';doc=json.loads(manifest.read_text());doc['peers']=doc['peers'][1:];manifest.write_text(json.dumps(doc));manifest.chmod(0o600)
        run(a.helper,'sync',core,ns='s',env=env)
        fetch(clients[0],expected=False);fetch(clients[1])
        if server.poll() is not None:raise RuntimeError('peer update restarted or stopped the server')
        print('PASS: peer removal blocked its traffic while the other peer and engine stayed active')
        stop(server)
        rules=run('iptables','-t','nat','-S',ns='s').stdout
        if ('rovunet-awg-'+core).encode() in rules:raise RuntimeError('scoped NAT rule remained after clean stop')
        server=spawn([a.helper,'run',core],ns='s',env=env);time.sleep(.5)
        if server.poll() is not None:raise RuntimeError('managed AWG server exited after restart')
        recovery=time.monotonic()
        # A peer can retain its old session until the engine's rekey timer.
        # Measure recovery instead of calling a listener a successful reboot.
        while True:
            try:fetch(clients[1]);break
            except RuntimeError:
                if server.poll() is not None or time.monotonic()-recovery>190:raise
                time.sleep(.2)
        print(f'PASS: clean stop removed scoped rules; restart recovered peer traffic after {time.monotonic()-recovery:.1f}s')
finally:
    for p in reversed(processes):stop(p)
    for ns in reversed(created):run('ip','netns','delete',ns,check=False)
