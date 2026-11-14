#!/usr/bin/env python3
"""Install the signed local review bundle on an operator-controlled Linux server."""
import hashlib
import json
import os
import pathlib
import platform
import shutil
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent
TAG = 'v2.0.0'
PUBLIC_DER = bytes.fromhex('302a300506032b6570032100')  # prefix checked below
SPKI = 'MCowBQYDK2VwAyEAMfWBf2jAnb20LJvMoOfN1R/4OP0tSjPec+GWU+QXdqs='

def main():
    import base64
    if os.geteuid() != 0 or sys.argv[1:]:
        sys.exit('Run as root: python3 install-local-review.py')
    arch = {'x86_64': 'amd64', 'aarch64': 'arm64'}[platform.machine()]
    dest = pathlib.Path('/usr/local/bin')
    dest.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(dir=dest, prefix='.review-') as scratch:
        stage = pathlib.Path(scratch)
        public = base64.b64decode(SPKI, validate=True)
        assert public.startswith(PUBLIC_DER)
        (stage/'root.der').write_bytes(public)
        for name in ('RELEASE-MANIFEST.json', 'RELEASE-MANIFEST.sig'):
            shutil.copyfile(ROOT/name, stage/name)
        subprocess.run(['openssl', 'pkeyutl', '-verify', '-rawin', '-pubin', '-keyform', 'DER',
                        '-inkey', str(stage/'root.der'), '-in', str(stage/'RELEASE-MANIFEST.json'),
                        '-sigfile', str(stage/'RELEASE-MANIFEST.sig')], check=True)
        m = json.loads((stage/'RELEASE-MANIFEST.json').read_bytes())
        if (m['environment'], m['repository'], m['tag'], m['key_id']) != (
                'production', 'msVPNco/rovunet-panel', TAG, hashlib.sha256(public).hexdigest()):
            sys.exit('Review signature identity mismatch')
        entries = {a['name']: a for a in m['assets']}
        def verified(name):
            asset = entries[name]
            local = stage/name
            shutil.copyfile(ROOT/name, local)
            with local.open('rb') as f:
                if local.stat().st_size != asset['size'] or hashlib.file_digest(f, 'sha256').hexdigest() != asset['sha256']:
                    sys.exit('Release verification failed for '+name)
            return local
        bootstrap = verified('install.sh')
        panel = verified('rovunet-linux-'+arch)
        panel.chmod(0o755)
        configured=pathlib.Path('/etc/rovunet/config.json').exists()
        if configured and not (dest/'rovunet').is_file():sys.exit('Existing configuration needs its installed panel for upgrade')
        if not configured and any(pathlib.Path('/etc/rovunet').glob('config.json.*')):sys.exit('Configuration missing while recovery state exists')
        # Resolve and verify all dependencies before creating any runtime.
        assets = {}
        for prog in ('xray','amneziawg-go','awg','caddy-naive','slipstream-server','dnstt-server','vaydns-server','sing-box','mita'):
            target = {'amneziawg-go':'rovunet-amneziawg-go','awg':'rovunet-awg','sing-box':'rovunet-sing-box','mita':'rovunet-mita'}.get(prog, prog)
            assets[target] = verified(prog+'-linux-'+arch)
        assets['rovunet-awg-provenance.json'] = verified('awg-provenance-linux-'+arch+'.json')
        for prog, local in assets.items():
            local.chmod(0o644 if prog.endswith('.json') else 0o755)
            try:
                os.link(local, dest/prog)
            except FileExistsError:
                if (dest/prog).is_symlink() or not (dest/prog).is_file():
                    sys.exit('Nonregular installed runtime '+prog)
        # The bootstrap independently authenticates the release and drives the
        # same transaction as menu 8. Only the exact signed files above are served.
        shim = stage/'curl'
        shim.write_text("""#!/usr/bin/python3
import pathlib,sys,shutil
a=sys.argv[1:];url=a[-1];prefix='https://github.com/msVPNco/rovunet-panel/releases/download/v2.0.0/'
if not url.startswith(prefix):sys.exit(1)
name=url[len(prefix):]
if name not in ['RELEASE-MANIFEST.json','RELEASE-MANIFEST.sig','rovunet-linux-amd64','rovunet-linux-arm64']:sys.exit(1)
shutil.copyfile(pathlib.Path(__file__).parent/name,a[a.index('--output')+1])
""")
        shim.chmod(0o755)
        env=dict(os.environ,PATH=str(stage)+':/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin',
                 ROVUNET_REPO='msVPNco/rovunet-panel',ROVUNET_RELEASE_TAG=TAG)
        # Restrict the local transport override to verification. The interactive
        # panel and all of its subprocesses receive the operator's original PATH.
        configured=pathlib.Path('/etc/rovunet/config.json').exists()
        if configured:
            subprocess.run([str(panel),'upgrade-installed',str(dest/'rovunet')],check=True)
        else:
            subprocess.run(['bash',str(bootstrap),'--no-run'],env=env,check=True)
            subprocess.run([str(dest/'rovunet'),'install'],check=True)

if __name__=='__main__':
    main()

