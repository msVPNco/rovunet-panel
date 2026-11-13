#!/usr/bin/env python3
"""Fault-test bootstrap only inside private mount/network/PID namespaces."""
import argparse
import importlib.util
import json
import os
import pathlib
import pty
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import time

ROOT = pathlib.Path(__file__).resolve().parents[1]


def run(*args, **kwargs):
    return subprocess.run(list(map(str, args)), check=True, capture_output=True, **kwargs)


def inside(work):
    # Refuse accidental direct invocation of the internal entry point.
    for name in ('mnt','net','pid'):
        parent=os.environ.get('ROVUNET_LAB_PARENT_'+name.upper())
        if not parent or os.readlink('/proc/self/ns/'+name)==parent:
            raise RuntimeError('Private mount, network and PID namespaces are required')
    run('mount', '--make-rprivate', '/')
    target = work/'target'
    target.mkdir()
    run('mount', '--bind', target, '/usr/local/bin')
    dest = pathlib.Path('/usr/local/bin')
    fake = work/'fakebin'
    fake.mkdir()
    assets = work/'assets'
    assets.mkdir()
    private = work/'ephemeral-test-key.pem'
    run('openssl', 'genpkey', '-algorithm', 'Ed25519', '-out', private)
    private.chmod(0o600)
    spec = importlib.util.spec_from_file_location('release_signer', ROOT/'scripts/sign-release.py')
    signer = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(signer)
    # The candidate records its actual executable path. It must not be executed
    # at all during verification, including failed/truncated downloads.
    fixture = work/'fixture.c'
    fixture.write_text('#include <stdio.h>\n#include <stdlib.h>\nint main(int argc,char**argv){FILE*f=fopen(getenv("EXEC_MARKER"),"a");if(!f)return 2;fprintf(f,"%s\\n",argv[0]);fclose(f);return 0;}\n')
    run('cc', '-o', work/'candidate', fixture)
    binary = (work/'candidate').read_bytes()
    name = 'rovunet-linux-amd64'
    if os.uname().machine != 'x86_64':
        raise RuntimeError('This bootstrap fixture currently claims Linux/amd64 only')
    original = (ROOT/'install.sh').read_text()
    key = signer.key_identity(private)
    ring = json.dumps({'schema': 1, 'keys': [dict(key, purpose='test')]}, separators=(',', ':'))
    lab, replacements = re.subn(r"^TRUST_ROOT_JSON='[^']*'$", "TRUST_ROOT_JSON='"+ring+"'", original, count=1, flags=re.M)
    if replacements != 1:
        raise RuntimeError('Expected exactly one compiled trust root')
    lab = lab.replace('TRUST_ENVIRONMENT=production', 'TRUST_ENVIRONMENT=test')
    if lab == original:
        raise RuntimeError('Private lab trust fixture was not prepared')
    installer = work/'installer-lab.sh'
    installer.write_text(lab)
    # The downloaded key is NEVER used; the private lab copy has a preselected
    # ephemeral test authority. The production copy rejects that authority.
    (fake/'curl').write_text('''#!/usr/bin/python3
import os,pathlib,sys,time,urllib.parse
args=sys.argv[1:]; dest=pathlib.Path(args[args.index('--output')+1]); url=args[-1]
name=pathlib.PurePosixPath(urllib.parse.urlparse(url).path).name
name={'RELEASE-MANIFEST.json':'RELEASE-MANIFEST.json','RELEASE-MANIFEST.sig':'RELEASE-MANIFEST.sig'}.get(name,name)
case=os.environ['BOOTSTRAP_CASE']; src=pathlib.Path(os.environ['BOOTSTRAP_ASSETS'])/name
if case in ['404','504']: sys.exit(22)
if case=='interrupt':
 pathlib.Path(os.environ['EXEC_MARKER']+'.waiting').touch(); time.sleep(15)
if not src.exists(): sys.exit(22)
data=src.read_bytes()
if case=='partial' and name.startswith('rovunet-linux-'): data=data[:len(data)//2]
if case=='no-space' and name.startswith('rovunet-linux-'): data=b'X'*(4*1024*1024)
dest.write_bytes(data)
''')
    (fake/'curl').chmod(0o755)
    env = dict(os.environ, PATH=str(fake)+':/usr/bin:/bin', ROVUNET_RELEASE_TAG='v1.21.2',
               BOOTSTRAP_ASSETS=str(assets), EXEC_MARKER=str(work/'executed'))
    old = b'healthy previous panel\n'
    results = []

    def reset():
        for p in dest.iterdir():
            if p.is_dir():
                if not p.name.startswith('.rovunet-install.'):
                    raise RuntimeError('Unexpected object in private mount')
                shutil.rmtree(p)
            else:
                p.unlink()
        (dest/'rovunet').write_bytes(old)
        (dest/'rovunet').chmod(0o755)
        for p in [work/'executed', work/'executed.waiting']:
            p.unlink(missing_ok=True)
        (assets/name).write_bytes(binary)
        signer.sign(assets, private, 'v1.21.2', 'a'*40, test=True)

    for case in ['production-rejects-test-key', '404', '504', 'missing-manifest',
                 'missing-signature', 'tampered-manifest', 'bad-signature',
                 'wrong-architecture', 'wrong-elf', 'partial', 'interrupt', 'no-space']:
        reset()
        script = installer
        mode = case
        if case == 'production-rejects-test-key':
            script = work/'production.sh'
            script.write_text(lab.replace('TRUST_ENVIRONMENT=test', 'TRUST_ENVIRONMENT=production'))
        elif case == 'missing-manifest':
            (assets/'RELEASE-MANIFEST.json').unlink()
        elif case == 'missing-signature':
            (assets/'RELEASE-MANIFEST.sig').unlink()
        elif case == 'tampered-manifest':
            with (assets/'RELEASE-MANIFEST.json').open('ab') as f:
                f.write(b'\n')
        elif case == 'bad-signature':
            (assets/'RELEASE-MANIFEST.sig').write_bytes(b'\x00'*64)
        elif case == 'wrong-architecture':
            m = json.loads((assets/'RELEASE-MANIFEST.json').read_text())
            m['assets'][0]['arch'] = 'arm64'
            (assets/'RELEASE-MANIFEST.json').write_text(json.dumps(m))
            (assets/'RELEASE-MANIFEST.sig').write_bytes(signer.openssl('pkeyutl', '-sign', '-rawin', '-inkey', private, '-in', assets/'RELEASE-MANIFEST.json'))
        elif case == 'wrong-elf':
            b = bytearray(binary)
            b[18:20] = (183).to_bytes(2, 'little')
            (assets/name).write_bytes(b)
            signer.sign(assets, private, 'v1.21.2', 'a'*40, test=True)
        if case == 'no-space':
            run('mount', '-t', 'tmpfs', '-o', 'size=2M', 'tmpfs', dest)
            (dest/'rovunet').write_bytes(old)
        current = dict(env, BOOTSTRAP_CASE=mode)
        if case == 'interrupt':
            proc = subprocess.Popen(['bash', str(script), '--no-run'], env=current, stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
            deadline = time.monotonic()+3
            while not (work/'executed.waiting').exists() and time.monotonic()<deadline:
                time.sleep(.01)
            if not (work/'executed.waiting').exists():
                raise RuntimeError('Interrupt fixture did not reach its controlled download')
            os.killpg(proc.pid, signal.SIGTERM)
            proc.communicate(timeout=5)
            code = proc.returncode
        else:
            proc = subprocess.run(['bash', str(script), '--no-run'], env=current, capture_output=True, timeout=10)
            code = proc.returncode
        if not code or (dest/'rovunet').read_bytes() != old or (work/'executed').exists():
            raise RuntimeError('Bootstrap fault did not preserve the healthy panel: '+case)
        if case == 'no-space':
            run('umount', dest)
        results.append(dict(case=case, passed=True, candidate_executed=False))

    reset()
    result = subprocess.run(['bash', str(installer), '--no-run'], env=dict(env, BOOTSTRAP_CASE='ok'), capture_output=True, timeout=10)
    if result.returncode or (dest/'rovunet').read_bytes() != binary or (dest/'rovunet.bak').read_bytes() != old or (work/'executed').exists():
        raise RuntimeError('Verified staging failed')
    # A repeat with pending setup must preserve the original recovery binary.
    result = subprocess.run(['bash', str(installer), '--no-run'], env=dict(env, BOOTSTRAP_CASE='ok'), capture_output=True, timeout=10)
    if result.returncode == 0 or (dest/'rovunet.bak').read_bytes() != old:
        raise RuntimeError('Repeated pending install overwrote recovery')
    results.append(dict(case='verified-stage-and-pending-repeat', passed=True))
    reset()
    # Give the private bootstrap a controlling terminal and observe only the
    # final installed path executing. This is not a full panel install test.
    pid, master = pty.fork()
    if pid == 0:
        os.execve('/bin/bash', ['bash', str(installer)], dict(env, BOOTSTRAP_CASE='ok'))
    try:
        while True:
            if not os.read(master, 4096):
                break
    except OSError:
        pass
    os.close(master)
    _, status = os.waitpid(pid, 0)
    if os.waitstatus_to_exitcode(status) or (work/'executed').read_text().splitlines() != ['/usr/local/bin/rovunet']:
        raise RuntimeError('Bootstrap executed a staging file or failed its permanent handoff')
    results.append(dict(case='permanent-path-exec-only', passed=True))
    print(json.dumps(dict(scope='Linux/amd64; private mount/network/PID namespaces; synthetic signer and executable; not production setup', checks=results), indent=2))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--work', required=True, type=pathlib.Path)
    parser.add_argument('--inside', action='store_true', help=argparse.SUPPRESS)
    args = parser.parse_args()
    if os.geteuid() != 0:
        parser.error('Use a dedicated Linux lab with root and namespace support')
    if args.inside:
        inside(args.work)
    else:
        args.work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(prefix='bootstrap-', dir=args.work) as d:
            env=dict(os.environ,**{'ROVUNET_LAB_PARENT_'+n.upper():os.readlink('/proc/self/ns/'+n) for n in ('mnt','net','pid')})
            subprocess.run(['unshare', '--mount', '--net', '--pid', '--fork', '--kill-child','--mount-proc',
                            sys.executable, str(pathlib.Path(__file__).resolve()), '--inside', '--work', d], check=True,env=env)


if __name__ == '__main__':
    main()
