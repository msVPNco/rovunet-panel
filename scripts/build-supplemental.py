#!/usr/bin/env python3
"""Build the reviewed AnyTLS/TUIC/Mieru runtimes from locked public sources."""
import hashlib,json,os,pathlib,shutil,subprocess,sys,tarfile,tempfile,urllib.request
ROOT=pathlib.Path(__file__).resolve().parents[1]
locks=ROOT/'third_party/supplemental';lock=json.loads((locks/'runtime-lock.json').read_text())
out=pathlib.Path(sys.argv[1]).resolve();out.mkdir(parents=True,exist_ok=True)
assert subprocess.check_output(['go','version'],text=True).split()[2]==lock['go'],'Review compiler before changing the runtime pins'
for item in lock['runtimes']:
 engine=item['engine']
 with tempfile.TemporaryDirectory(prefix='supplemental-',dir=out) as tmp:
  work=pathlib.Path(tmp);archive=work/'source.tar.gz'
  cached=pathlib.Path(os.environ.get('ROVUNET_SOURCE_ARCHIVES',''))/(('sing-box' if engine=='sing-box' else 'mieru')+'-'+item['tag']+'-source.tar.gz')
  if os.environ.get('ROVUNET_SOURCE_ARCHIVES') and cached.is_file():shutil.copyfile(cached,archive)
  else:urllib.request.urlretrieve('https://codeload.github.com/'+item['repository']+'/tar.gz/refs/tags/'+item['tag'],archive)
  assert hashlib.sha256(archive.read_bytes()).hexdigest()==item['archive_sha256'],'Upstream source digest changed'
  with tarfile.open(archive) as t:t.extractall(work,filter='data')
  source=next(p for p in work.iterdir() if p.is_dir())
  for f in ['go.mod','go.sum']:shutil.copyfile(locks/(engine+'.'+f),source/f)
  for arch in ['amd64','arm64']:
   target=out/(engine+'-linux-'+arch)
   args=['go','build','-mod=readonly','-trimpath']
   if engine=='sing-box':args+=['-tags','with_quic,with_utls','-ldflags','-X github.com/sagernet/sing-box/constant.Version=1.14.4-rovunet.1']
   args+=['-o',str(target),'./cmd/'+('sing-box' if engine=='sing-box' else 'mita')]
   subprocess.run(args,cwd=source,env=dict(os.environ,GOOS='linux',GOARCH=arch,CGO_ENABLED='0',GOTOOLCHAIN='local',GOTELEMETRY='off'),check=True)
   assert hashlib.sha256(target.read_bytes()).hexdigest()==item['arch_sha256'][arch],'Rebuilt output differs from accepted bytes; repeat acceptance before publication'
  (source/'ROVUNET-MODIFICATIONS.txt').write_text('Upstream '+item['repository']+' '+item['tag']+'; Go module dependency fixes in go.mod/go.sum. No upstream source edits. sing-box build tags: with_quic,with_utls. Go '+lock['go']+'. No upstream endorsement is implied.\n')
  with tarfile.open(out/(engine+'-source.tar.gz'),'w:gz') as t:t.add(source,arcname=engine)
  shutil.copyfile(source/'LICENSE',out/('LICENSE-'+engine+'.txt'))
shutil.copyfile(locks/'runtime-lock.json',out/'supplemental-runtime-lock.json')
