#!/usr/bin/env python3
"""Bridge absolute ledgers to the old panel while its accounting timers are off.

The old collector will count the current engine epoch once on its next reset.
Project only earlier epochs for those running cores, and all totals for stopped
cores. A private journal allows a failed/interrupted rollback to be retried.
"""
import argparse
import base64
import json
import os
import pathlib
import subprocess


def atomic(path, data):
    temporary = path.with_name(path.name + '.tmp')
    with temporary.open('wb') as output:
        os.chmod(temporary, 0o600)
        output.write(data)
        output.flush()
        os.fsync(output.fileno())
    os.replace(temporary, path)
    descriptor = os.open(path.parent, os.O_RDONLY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def engine_epoch(name):
    unit = 'rovunet-core-' + name + '.service'
    active = subprocess.run(['systemctl', 'is-active', unit], capture_output=True, text=True)
    if active.returncode or active.stdout.strip() != 'active':
        return ''
    result = subprocess.run(['systemctl', 'show', unit, '--property=InvocationID', '--value'],
                            capture_output=True, text=True, check=True)
    epoch = result.stdout.strip()
    if not epoch:
        raise ValueError('Cannot identify the running engine epoch')
    return epoch


def checked_counts(values):
    if not isinstance(values, dict) or any(type(v) is not int or not 0 <= v <= 2**63-1 for v in values.values()):
        raise ValueError('Invalid usage counts; old panel was not installed')
    return values


def restore(usage, backup):
    plan_path = backup / 'rollback-usage-plan.json'
    if not plan_path.exists():
        return
    plan = json.loads(plan_path.read_text())
    archive = usage / plan['archive']
    for name in plan['sources']:
        original, saved = usage / name, archive / name
        if saved.exists():
            if original.exists():
                raise ValueError('Usage changed during rollback; preserve both copies for review')
            os.replace(saved, original)
    projection = usage / 'preview-history.json'
    if plan['previous'] is None:
        projection.unlink(missing_ok=True)
    else:
        atomic(projection, base64.b64decode(plan['previous']))
    (backup / 'rollback-usage-ready').unlink(missing_ok=True)


def prepare(usage, backup, epoch=engine_epoch):
    if (backup / 'rollback-usage-ready').exists():
        return
    restore(usage, backup)
    usage.mkdir(parents=True, exist_ok=True)
    projection = usage / 'preview-history.json'
    previous = projection.read_bytes() if projection.exists() else None
    totals = checked_counts(json.loads(previous)) if previous is not None else {}
    sources = sorted(usage.glob('xray-*.source'))
    for source in sources:
        record = json.loads(source.read_text())
        counts = checked_counts(record['totals'])
        last = checked_counts(record['last'])
        if not record.get('epoch'):
            raise ValueError('Missing engine epoch in usage ledger')
        contribution = dict(counts)
        if epoch(source.name[5:-7]) == record['epoch']:
            for direction, value in last.items():
                user = direction.split('>>>')[0]
                contribution[user] = contribution.get(user, 0) - value
        for user, value in contribution.items():
            if value < 0 or totals.get(user, 0) + value > 2**63-1:
                raise ValueError('Invalid usage bridge; preserved private ledgers')
            totals[user] = totals.get(user, 0) + value
    archive = usage / ('.preview-rollback-' + backup.name)
    archive.mkdir(mode=0o700, exist_ok=True)
    plan = dict(archive=archive.name, sources=[p.name for p in sources],
                previous=base64.b64encode(previous).decode() if previous is not None else None)
    atomic(backup / 'rollback-usage-plan.json', json.dumps(plan).encode())
    try:
        for source in sources:
            os.replace(source, archive / source.name)
        atomic(projection, json.dumps(totals).encode())
        atomic(backup / 'rollback-usage-ready', b'ready\n')
    except BaseException:
        restore(usage, backup)
        raise


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['prepare', 'restore'])
    parser.add_argument('--usage', type=pathlib.Path, required=True)
    parser.add_argument('--backup', type=pathlib.Path, required=True)
    arguments = parser.parse_args()
    globals()[arguments.action](arguments.usage, arguments.backup)
