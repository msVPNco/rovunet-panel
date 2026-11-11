#!/usr/bin/env python3
"""Exercise the actual release tag step with isolated GitHub API responses."""
import os
import pathlib
import subprocess
import tempfile
import unittest

source = pathlib.Path(__file__).resolve().parents[1]
script = []
inside = running = False
for line in (source / '.github/workflows/release.yml').read_text().splitlines():
    if line == '      - name: Pick the release tag':
        inside = True
    elif inside and line == '        run: |':
        running = True
    elif running:
        if line.strip() and not line.startswith('          '):
            break
        script.append(line[10:] if line.strip() else '')
assert script, 'release tag step was not found'


class ReleaseTagTests(unittest.TestCase):
    def run_case(self, response, tag=''):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / 'Makefile').write_text('print-version:\n\t@echo 1.21.0\n')
            gh = root / 'gh'
            gh.write_text('''#!/usr/bin/env bash
case "$MOCK_RESPONSE" in
  absent) printf '{"message":"Not Found","status":"404"}\\n'; exit 1 ;;
  published) printf '{"draft":false}\\n' ;;
  own-draft) printf '{"draft":true,"target_commitish":"%s"}\\n' "$GITHUB_SHA" ;;
  other-draft) printf '{"draft":true,"target_commitish":"another-source"}\\n' ;;
esac
''')
            gh.chmod(0o755)
            # Keep the fixture self-contained on hosts without the jq CLI.
            jq = root / 'jq'
            jq.write_text('''#!/usr/bin/env python3
import json,sys
value=json.load(sys.stdin).get(sys.argv[-1].lstrip('.'))
print(json.dumps(value) if isinstance(value,(bool,type(None))) else value)
''')
            jq.chmod(0o755)
            output = root / 'output'
            environment = dict(os.environ, PATH=str(root) + os.pathsep + os.environ['PATH'],
                               MOCK_RESPONSE=response, GITHUB_REF_TYPE='tag' if tag else 'branch',
                               GITHUB_REF_NAME=tag or 'main', GITHUB_SHA='ab' * 20,
                               GITHUB_REPOSITORY='example/panel', GITHUB_OUTPUT=str(output))
            result = subprocess.run(['bash', '-e'], input='\n'.join(script), cwd=root,
                                    env=environment, text=True, stdout=subprocess.PIPE,
                                    stderr=subprocess.PIPE, timeout=15)
            values = dict(line.split('=', 1) for line in output.read_text().splitlines()) if output.exists() else {}
            return result.returncode, values

    def test_404_with_json_body_still_builds_new_release(self):
        code, values = self.run_case('absent')
        self.assertEqual(code, 0)
        self.assertEqual(values.get('tag'), 'v1.21.0')
        self.assertNotEqual(values.get('skip'), 'true')

    def test_published_release_is_not_overwritten(self):
        code, values = self.run_case('published')
        self.assertEqual(code, 0)
        self.assertEqual(values.get('skip'), 'true')

    def test_own_interrupted_draft_can_resume(self):
        code, values = self.run_case('own-draft')
        self.assertEqual(code, 0)
        self.assertNotEqual(values.get('skip'), 'true')

    def test_another_sources_draft_is_rejected(self):
        self.assertNotEqual(self.run_case('other-draft')[0], 0)

    def test_matching_tag_is_accepted(self):
        self.assertEqual(self.run_case('absent', 'v1.21.0')[0], 0)

    def test_tag_for_another_version_is_rejected(self):
        self.assertNotEqual(self.run_case('absent', 'v0.0.1')[0], 0)


if __name__ == '__main__':
    unittest.main(verbosity=2)
