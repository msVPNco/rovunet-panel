#!/usr/bin/env python3
"""Report open production gates; publication requires each to be verified."""
import argparse
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
REQUIRED = {'P0-PRODUCTION-TRUST','P0-UPSTREAM-PROVENANCE',
            'P0-RECOVERY-ACCEPTANCE','P0-CLIENT-CONTRACT'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--report-only', action='store_true')
    args = parser.parse_args()
    doc = json.loads((ROOT/'docs/release-gates.json').read_text())
    rows = doc.get('gates', [])
    if doc.get('schema') != 1 or len(rows) != len(REQUIRED) or {r['id'] for r in rows} != REQUIRED:
        parser.exit(1,'Invalid production gate inventory\n')
    for row in rows:
        if not row.get('reason') or not (ROOT/row.get('evidence','missing')).is_file():
            parser.exit(1,'A production gate lacks its review evidence\n')
    pending = [r for r in rows if r['status'] != 'verified']
    for row in pending:
        print(row['id']+': '+row['reason'])
    if pending and not args.report_only:
        parser.exit(1,'Production publication stopped: open P0 acceptance gates remain.\n')
    if not pending and doc.get('publication') != 'ready':
        parser.exit(1,'Production publication has not been marked ready after review.\n')
    print(f'Production gates: {len(rows)-len(pending)} verified, {len(pending)} open.')


if __name__ == '__main__':
    main()
