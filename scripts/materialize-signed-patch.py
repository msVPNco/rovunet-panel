#!/usr/bin/env python3
"""Materialize only a locally approved, production-signed v2.0.2 request.

No signing key is present in CI. Every recreated or reused byte must match the
owner's already signed manifest before any release is published.
"""
import argparse,base64,hashlib,importlib.util,json,os,re,subprocess,tempfile
from pathlib import Path

ROOT=Path(__file__).resolve().parents[1]
KEY='885d8aba22365ce42e20527bfa00e82aed3cba2f69501f146ed0496a8bc3dcbe'
REPO='msVPNco/rovunet-panel'

def unique(pairs):
 result={}
 for key,value in pairs:
  if key in result:raise ValueError('duplicate JSON key')
  result[key]=value
 return result

def run(*args,**kw):return subprocess.run(list(map(str,args)),check=True,**kw)
def gh_json(*args):return json.loads(run('gh',*args,capture_output=True).stdout,object_pairs_hook=unique)
def request(path):
 if path.stat().st_size>2*1024*1024:raise ValueError('request too large')
 r=json.loads(path.read_bytes(),object_pairs_hook=unique)
 if set(r)!={'schema','tag','previous_tag','source_commit','manifest_base64','signature_base64','local_acceptance_sha256','publish_latest'} or r['schema']!=1 or r['tag']!='v2.0.2' or r['previous_tag']!='v2.0.1' or r['publish_latest'] is not True or not re.fullmatch('[0-9a-f]{40}',r['source_commit']) or not re.fullmatch('[0-9a-f]{64}',r['local_acceptance_sha256']):raise ValueError('unapproved release identity')
 raw=base64.b64decode(r['manifest_base64'],validate=True);sig=base64.b64decode(r['signature_base64'],validate=True)
 if not 0<len(raw)<=1048576 or len(sig)!=64:raise ValueError('invalid signed wire size')
 m=json.loads(raw,object_pairs_hook=unique)
 if set(m)!={'schema','environment','key_id','repository','tag','source_commit','assets'} or m['schema']!=1 or m['environment']!='production' or m['key_id']!=KEY or m['repository']!=REPO or m['tag']!=r['tag'] or m['source_commit']!=r['source_commit'] or len(m['assets'])!=38:raise ValueError('manifest/request binding mismatch')
 names=set()
 for a in m['assets']:
  if not re.fullmatch('[A-Za-z0-9][A-Za-z0-9._-]{0,159}',a['name']) or a['name'] in names or type(a['size']) is not int or not 0<a['size']<=536870912 or not re.fullmatch('[0-9a-f]{64}',a['sha256']):raise ValueError('unsafe signed asset entry')
  names.add(a['name'])
 ring=json.loads((ROOT/'internal/releaseauth/production-keys.json').read_bytes());key=next(k for k in ring['keys'] if k['id']==KEY and k['purpose']=='production');der=base64.b64decode(key['spki'],validate=True)
 if hashlib.sha256(der).hexdigest()!=KEY:raise ValueError('production root fingerprint mismatch')
 with tempfile.TemporaryDirectory(prefix='signed-wire-') as name:
  d=Path(name);(d/'public.der').write_bytes(der);(d/'manifest').write_bytes(raw);(d/'sig').write_bytes(sig)
  run('openssl','pkeyutl','-verify','-rawin','-pubin','-keyform','DER','-inkey',d/'public.der','-in',d/'manifest','-sigfile',d/'sig',stdout=subprocess.DEVNULL)
 return r,m,raw,sig

def source_ci(r):
 runs=gh_json('api',f'repos/{REPO}/actions/runs?head_sha={r["source_commit"]}&event=push&per_page=30')['workflow_runs']
 if not any(x['head_sha']==r['source_commit'] and x['path']=='.github/workflows/ci.yml' and x['conclusion']=='success' for x in runs):raise ValueError('Exact source CI must pass before publication')

def assemble(r,raw,sig):
 source_ci(r)
 if run('git','rev-parse','HEAD',capture_output=True,cwd=ROOT).stdout.decode().strip()!=r['source_commit']:raise ValueError('checkout differs from approved source')
 temp=Path(os.environ['RUNNER_TEMP']);previous=temp/'previous-authenticated';out=temp/'approved-patch';previous.mkdir();out.mkdir()
 run('gh','release','download',r['previous_tag'],'--repo',REPO,'--dir',previous)
 flags='-s -w -X github.com/msVPNco/rovunet-panel/internal/version.Version=2.0.2 -X github.com/msVPNco/rovunet-panel/internal/version.Commit='+r['source_commit']
 for arch in ['amd64','arm64']:
  env=os.environ.copy();env.update(GOOS='linux',GOARCH=arch,CGO_ENABLED='0',GOAMD64='v1',GOARM64='v8.0',GOTOOLCHAIN='local',GOTELEMETRY='off',GOENV='off')
  run('go','build','-buildvcs=false','-trimpath','-ldflags',flags,'-o',out/('rovunet-linux-'+arch),'.',env=env,cwd=ROOT)
 run('python3',ROOT/'scripts/assemble-patch-assets.py','--previous',previous,'--output',out,'--commit',r['source_commit'],'--tag',r['tag'])
 actual=(out/'RELEASE-MANIFEST.json').read_bytes()
 if actual!=raw:
  expected=json.loads(raw);built=json.loads(actual);aa={x['name']:x for x in expected['assets']};bb={x['name']:x for x in built['assets']}
  different=[name for name in sorted(set(aa)|set(bb)) if aa.get(name)!=bb.get(name)]
  raise ValueError('CI bytes differ from local signed candidate; nothing published: '+','.join(different))
 (out/'RELEASE-MANIFEST.sig').write_bytes(sig)
 run('python3',ROOT/'scripts/verify-release.py','--assets',out,'--tag',r['tag'],'--commit',r['source_commit'])

def publish(r,raw):
 source_ci(r);out=Path(os.environ['RUNNER_TEMP'])/'approved-patch'
 run('python3',ROOT/'scripts/verify-release.py','--assets',out,'--tag',r['tag'],'--commit',r['source_commit'])
 latest=gh_json('api',f'repos/{REPO}/releases/latest')
 if latest['tag_name']!=r['previous_tag']:raise ValueError('Public Latest changed; do not supersede it automatically')
 view=subprocess.run(['gh','release','view',r['tag'],'--repo',REPO,'--json','isDraft,targetCommitish'],capture_output=True)
 if view.returncode==0:
  current=json.loads(view.stdout)
  if not current['isDraft'] or current['targetCommitish']!=r['source_commit']:raise ValueError('Refusing to change an existing public/different release')
 else:run('gh','release','create',r['tag'],'--repo',REPO,'--draft','--target',r['source_commit'],'--title','Rovunet Panel v2.0.2 — Slipstream certificates and installation recovery','--notes-file',ROOT/'docs/releases/v2.0.2.md')
 current=gh_json('api',f'repos/{REPO}/releases/tags/{r["tag"]}')
 if not current['draft']:raise ValueError('Draft required before upload')
 files={p.name:p for p in out.iterdir() if p.is_file()}
 for asset in current['assets']:
  p=files.get(asset['name'])
  if p is None or asset['size']!=p.stat().st_size or asset.get('digest')!='sha256:'+hashlib.sha256(p.read_bytes()).hexdigest():raise ValueError('Existing draft asset differs; no overwrite permitted')
 existing={a['name'] for a in current['assets']}
 missing=[str(p) for name,p in sorted(files.items()) if name not in existing]
 if missing:run('gh','release','upload',r['tag'],'--repo',REPO,*missing)
 current=gh_json('api',f'repos/{REPO}/releases/tags/{r["tag"]}')
 if len(current['assets'])!=40:raise ValueError('Incomplete upload')
 for asset in current['assets']:
  p=files.get(asset['name'])
  if p is None or asset['state']!='uploaded' or asset['size']!=p.stat().st_size or asset.get('digest')!='sha256:'+hashlib.sha256(p.read_bytes()).hexdigest():raise ValueError('GitHub asset digest mismatch')
 run('gh','release','edit',r['tag'],'--repo',REPO,'--draft=false','--latest')
 tag=gh_json('api',f'repos/{REPO}/git/ref/tags/{r["tag"]}')
 if tag['object']['type']!='commit' or tag['object']['sha']!=r['source_commit']:raise ValueError('Published tag source differs')
 print(json.dumps({'published':r['tag'],'source_commit':r['source_commit'],'assets':40,'all_github_digests_matched':True,'local_acceptance_sha256':r['local_acceptance_sha256']}))

if __name__=='__main__':
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('mode',choices=['identity','assemble','publish']);parser.add_argument('request',type=Path);args=parser.parse_args()
 r,m,raw,sig=request(args.request)
 if os.environ.get('GITHUB_REPOSITORY',REPO)!=REPO:raise SystemExit('Unexpected repository')
 if args.mode=='identity':source_ci(r);print('Approved production signature and source CI verified')
 elif args.mode=='assemble':assemble(r,raw,sig)
 else:publish(r,raw)
