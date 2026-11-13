#!/usr/bin/env python3
"""Test the actual DNS router in an owned transient systemd lab unit.

The unit gets NoNewPrivileges=yes; its child runs in private mount/network/PID
namespaces with toy config and a loopback echo backend. It does not reboot the
host, alter real DNS/SSH/firewall, or claim tunnel/client acceptance.
"""
import argparse
import hashlib
import json
import os
import pathlib
import socket
import statistics
import struct
import subprocess
import sys
import tempfile
import threading
import time
import uuid


def query(txid):
    name=b''.join(bytes([len(p)])+p.encode() for p in 'healthy.t.example.test'.split('.'))+b'\0'
    return struct.pack('!6H',txid,0x100,1,0,0,0)+name+struct.pack('!HH',16,1)


def inside(args):
    for name in ['mnt','net','pid']:
        if os.environ.get('ROVUNET_LAB_PARENT_'+name.upper()) in [None,os.readlink('/proc/self/ns/'+name)]:
            raise RuntimeError('Private namespaces are required')
    subprocess.run(['mount','--make-rprivate','/'],check=True)
    for name,destination in [('etc','/etc'),('lib','/var/lib')]:
        p=args.work/name;p.mkdir(exist_ok=True)
        subprocess.run(['mount','--bind',str(p),destination],check=True)
    subprocess.run(['ip','link','set','lo','up'],check=True)
    root=pathlib.Path('/etc/rovunet');root.mkdir(exist_ok=True)
    cfg=dict(listen={'address':'127.0.0.1:53'},tunnels=[dict(tag='dnstt',transport='dnstt',backend='socks',domain='t.example.test',port=5310,enabled=True,dnstt={})],backends=[dict(tag='socks',type='socks',address='127.0.0.1:1080')],route={'mode':'multi'})
    (root/'config.json').write_text(json.dumps(cfg))
    backend=socket.socket(socket.AF_INET,socket.SOCK_DGRAM);backend.bind(('127.0.0.1',5310))
    def echo():
        while True:
            try:
                data,addr=backend.recvfrom(4096);data=bytearray(data);data[2]|=0x80;backend.sendto(data,addr)
            except OSError:
                return
    threading.Thread(target=echo,daemon=True).start()
    process=subprocess.Popen([str(args.panel),'dnsrouter','serve'],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    try:
        deadline=time.monotonic()+5
        while True:
            try:
                ready=socket.create_connection(('127.0.0.1',53),timeout=.1);ready.close();break
            except OSError:
                if process.poll() is not None or time.monotonic()>deadline:
                    raise RuntimeError('Actual router did not become ready')
                time.sleep(.01)
        status=pathlib.Path('/proc/'+str(process.pid)+'/status').read_text()
        no_new_privileges=any(line.split(':',1)[0]=='NoNewPrivs' and line.split(':',1)[1].strip()=='1' for line in status.splitlines())
        if not no_new_privileges:
            raise RuntimeError('The actual panel did not inherit NoNewPrivileges')
        durations=[]
        for protocol in ['udp','tcp']:
            if protocol=='udp':
                conn=socket.socket(socket.AF_INET,socket.SOCK_DGRAM);conn.settimeout(1);conn.connect(('127.0.0.1',53))
            else:
                conn=socket.create_connection(('127.0.0.1',53),timeout=1)
            try:
                for n in range(100):
                    data=query(n);start=time.monotonic()
                    if protocol=='udp':
                        conn.send(data);response=conn.recv(4096)
                    else:
                        conn.sendall(struct.pack('!H',len(data))+data)
                        def read_exact(size):
                            out=b''
                            while len(out)<size:
                                part=conn.recv(size-len(out))
                                if not part:raise RuntimeError('Truncated TCP response')
                                out+=part
                            return out
                        response=read_exact(struct.unpack('!H',read_exact(2))[0])
                    if response[:2]!=data[:2] or not response[2]&0x80:
                        raise RuntimeError('Router payload/identity mismatch')
                    durations.append((time.monotonic()-start)*1000)
            finally:
                conn.close()
        durations.sort()
        existing=json.loads((args.work/'results.json').read_text()) if (args.work/'results.json').exists() else []
        existing.append(dict(attempts=200,failures=0,udp_attempts=100,tcp_attempts=100,no_new_privileges=True,
                             p50_ms=statistics.median(durations),p95_ms=durations[189],
                             binary_sha256=hashlib.sha256(args.panel.read_bytes()).hexdigest(),
                             scope='actual router; synthetic loopback backend; private namespaces; not DNS tunnel payload or real-client proof'))
        (args.work/'results.json').write_text(json.dumps(existing,indent=2)+'\n')
    finally:
        process.terminate()
        try:process.communicate(timeout=5)
        except subprocess.TimeoutExpired:process.kill();process.communicate()
        backend.close()


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--panel',type=pathlib.Path,required=True)
    parser.add_argument('--work',type=pathlib.Path,required=True)
    parser.add_argument('--inside',action='store_true',help=argparse.SUPPRESS)
    args=parser.parse_args();args.panel=args.panel.resolve()
    if os.geteuid()!=0:parser.error('Use an authorized dedicated Linux lab with systemd and namespaces')
    if args.inside:
        inside(args);return
    args.work.mkdir(parents=True,exist_ok=True)
    unit='rovunet-lab-router-'+uuid.uuid4().hex[:12]
    with tempfile.TemporaryDirectory(prefix='router-systemd-',dir=args.work) as path:
        env=dict(os.environ,**{'ROVUNET_LAB_PARENT_'+n.upper():os.readlink('/proc/self/ns/'+n) for n in ('mnt','net','pid')})
        command=['systemd-run','--quiet','--unit='+unit,'--property=Type=oneshot','--property=RemainAfterExit=yes',
                 '--property=NoNewPrivileges=yes','--property=TimeoutStartSec=30s','--property=TimeoutStopSec=5s']
        for n in ['mnt','net','pid']:
            command+=['--setenv=ROVUNET_LAB_PARENT_'+n.upper()+'='+env['ROVUNET_LAB_PARENT_'+n.upper()]]
        command+=['/usr/bin/unshare','--mount','--net','--pid','--fork','--kill-child','--mount-proc',sys.executable,
                  str(pathlib.Path(__file__).resolve()),'--inside','--panel',str(args.panel),'--work',path]
        try:
            subprocess.run(command,check=True,capture_output=True)
            first=subprocess.check_output(['systemctl','show',unit,'--property=InvocationID','--value'],text=True).strip()
            subprocess.run(['systemctl','restart',unit],check=True,capture_output=True)
            second=subprocess.check_output(['systemctl','show',unit,'--property=InvocationID','--value'],text=True).strip()
            rows=json.loads((pathlib.Path(path)/'results.json').read_text())
            if len(rows)!=2 or not first or first==second:raise RuntimeError('Owned service restart was not observed')
            result=dict(systemd_no_new_privileges=True,restart_observed=True,tests=rows,host_reboot_tested=False,
                        live_host_service_changes='only this explicitly owned transient lab unit')
            (args.work/'router-systemd-results.json').write_text(json.dumps(result,indent=2)+'\n')
            print(json.dumps(result,indent=2))
        except subprocess.CalledProcessError as exc:
            print(subprocess.run(['journalctl','-u',unit,'-n','12','--no-pager'],capture_output=True,text=True).stdout,file=sys.stderr)
            raise RuntimeError('Owned systemd lab did not complete') from exc
        finally:
            subprocess.run(['systemctl','stop',unit],capture_output=True)
            subprocess.run(['systemctl','reset-failed',unit],capture_output=True)


if __name__=='__main__':main()
