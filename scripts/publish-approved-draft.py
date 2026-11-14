#!/usr/bin/env python3
"""Publish only the existing approved v2.0.2 draft by its stable release ID.

GitHub's releases/tags GET excludes this unpublished draft. The exact signed
source publisher, its authority checks and every asset gate remain in force.
"""
import importlib.util,json,os,sys
from pathlib import Path
DRAFT_ID=445672321
TAG_PATH='repos/msVPNco/rovunet-panel/releases/tags/v2.0.2'
ID_PATH='repos/msVPNco/rovunet-panel/releases/'+str(DRAFT_ID)
def bind_draft_queries(original):
 def query(*args):
  if args==('api',TAG_PATH):
   release=original('api',ID_PATH)
   if release.get('id')!=DRAFT_ID or release.get('tag_name')!='v2.0.2':raise ValueError('Unexpected draft identity')
   return release
  return original(*args)
 return query

def main(request_path,source):
 spec=importlib.util.spec_from_file_location('approved_source_publisher',source/'scripts/materialize-signed-patch.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
 if os.environ.get('GITHUB_REPOSITORY')!='msVPNco/rovunet-panel':raise ValueError('Unexpected publication repository')
 r,manifest,raw,sig=m.request(request_path)
 if r['source_commit']!='efa697256880b0d2c0b1494513b51f4c72a7735b':raise ValueError('Unexpected approved source')
 if m.run('git','rev-parse','HEAD',capture_output=True,cwd=source).stdout.decode().strip()!=r['source_commit']:raise ValueError('Checkout differs from approved source')
 out=Path(os.environ['RUNNER_TEMP'])/'approved-patch'
 if (out/'RELEASE-MANIFEST.json').read_bytes()!=raw or (out/'RELEASE-MANIFEST.sig').read_bytes()!=sig:raise ValueError('Prepared release differs from exact approval')
 release=m.gh_json('api',ID_PATH)
 if release.get('id')!=DRAFT_ID or release.get('tag_name')!=r['tag'] or release.get('target_commitish')!=r['source_commit'] or release.get('draft') is not True or release.get('prerelease') is not False:raise ValueError('Refusing a published, foreign or differently targeted release')
 m.gh_json=bind_draft_queries(m.gh_json)
 # The original publisher rechecks exact source CI, production signature,
 # Latest lease, draft/target, all40 digests/sizes and the final source tag.
 m.publish(r,raw)
if __name__=='__main__':main(Path(sys.argv[1]),Path(sys.argv[2]))
