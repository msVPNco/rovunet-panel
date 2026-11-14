#!/usr/bin/env python3
"""Deterministically assemble a patch release from authenticated prior assets.

Panel executables are built separately. This script never signs or publishes.
"""
import argparse,base64,gzip,hashlib,importlib.util,io,json,re,shutil,subprocess,tarfile
from pathlib import Path

ROOT=Path(__file__).resolve().parents[1]
REPLACED={'rovunet-linux-amd64','rovunet-linux-arm64','install.sh','rovunet-panel-source.tar.gz','TEST-SCOPE.txt','SHA256SUMS','RELEASE-MANIFEST.json','RELEASE-MANIFEST.sig'}

def load_script(name):
 spec=importlib.util.spec_from_file_location(name.replace('-','_'),ROOT/'scripts'/name)
 module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module);return module

def source_archive(tree):
 data=io.BytesIO()
 with tarfile.open(fileobj=data,mode='w',format=tarfile.USTAR_FORMAT) as archive:
  for entry in sorted(tree,key=lambda x:x['path']):
   if entry['type']!='blob':continue
   path=entry['path'];mode=entry['mode']
   if not path or path.startswith('/') or '..' in Path(path).parts:raise ValueError('unsafe source path')
   p=ROOT/path
   raw=p.read_bytes() if mode!='120000' else str(p.readlink()).encode()
   blob=hashlib.sha1(b'blob '+str(len(raw)).encode()+b'\0'+raw).hexdigest()
   if blob!=entry['sha']:raise ValueError('Source differs from exact Git tree: '+path)
   item=tarfile.TarInfo('rovunet-panel/'+path);item.uid=item.gid=0;item.mtime=0;item.uname=item.gname=''
   if mode=='120000':item.type=tarfile.SYMTYPE;item.linkname=raw.decode();item.mode=0o777;archive.addfile(item)
   else:item.mode=0o755 if mode=='100755' else 0o644;item.size=len(raw);archive.addfile(item,io.BytesIO(raw))
 return gzip.compress(data.getvalue(),mtime=0)

def git_tree():
 raw=subprocess.run(['git','ls-tree','-rz','HEAD'],cwd=ROOT,capture_output=True,check=True).stdout
 result=[]
 for row in raw.split(b'\0'):
  if not row:continue
  meta,path=row.split(b'\t',1);mode,kind,sha=meta.decode().split();result.append(dict(path=path.decode(),mode=mode,type=kind,sha=sha))
 return result

def assemble(previous,output,commit,tag,tree):
 verifier=load_script('verify-release.py');signer=load_script('sign-release.py')
 old=json.loads((previous/'RELEASE-MANIFEST.json').read_bytes());ring=json.loads((ROOT/'internal/releaseauth/production-keys.json').read_bytes())
 verifier.verify(previous,old['tag'],old['source_commit'],ring)
 output.mkdir(parents=True,exist_ok=True)
 if (output/'RELEASE-MANIFEST.json').exists() or (output/'RELEASE-MANIFEST.sig').exists():raise ValueError('Refusing to mutate a prepared release')
 for item in previous.iterdir():
  if item.is_file() and item.name not in REPLACED:shutil.copyfile(item,output/item.name)
 for name in ['rovunet-linux-amd64','rovunet-linux-arm64']:
  if not(output/name).is_file():raise ValueError('Missing separately built panel: '+name)
 shutil.copyfile(ROOT/'install.sh',output/'install.sh')
 (output/'rovunet-panel-source.tar.gz').write_bytes(source_archive(tree))
 shutil.copyfile(ROOT/'docs/releases/v2.0.2-test-scope.txt',output/'TEST-SCOPE.txt')
 (output/'SHA256SUMS').write_text(''.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+p.name+'\n' for p in sorted(output.iterdir()) if p.is_file()),encoding='utf-8')
 entries=[dict(name=p.name,size=p.stat().st_size,sha256=hashlib.sha256(p.read_bytes()).hexdigest(),**signer.metadata(p.name,commit)) for p in sorted(output.iterdir()) if p.is_file()]
 if len(entries)!=38:raise ValueError('Unexpected patch payload count')
 manifest=dict(schema=1,environment='production',key_id=ring['keys'][0]['id'],repository='msVPNco/rovunet-panel',tag=tag,source_commit=commit,assets=entries)
 (output/'RELEASE-MANIFEST.json').write_text(json.dumps(manifest,sort_keys=True,separators=(',',':'))+'\n',encoding='utf-8')
 return manifest

if __name__=='__main__':
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--previous',type=Path,required=True);parser.add_argument('--output',type=Path,required=True);parser.add_argument('--commit',required=True);parser.add_argument('--tag',required=True);parser.add_argument('--tree',type=Path);args=parser.parse_args()
 if not re.fullmatch('[0-9a-f]{40}',args.commit) or args.tag!='v2.0.2':raise SystemExit('Unexpected source/tag')
 tree=json.loads(args.tree.read_bytes())['tree'] if args.tree else git_tree()
 manifest=assemble(args.previous,args.output,args.commit,args.tag,tree)
 print(json.dumps({'assembled':len(manifest['assets']),'source_commit':args.commit,'signed':False}))
