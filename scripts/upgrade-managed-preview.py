#!/usr/bin/env python3
"""Upgrade the audited preview panel while keeping its running engines and URLs."""
import argparse
import base64
import contextlib
import datetime
import fcntl
import hashlib
import json
import os
import pathlib
import platform
import re
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.parse

TIMERS = ('rovunet-user-expiry.timer', 'rovunet-core-certs.timer')
PREVIOUS_VERSION = 'rovunet v1.20.1-managed-preview.2 (037e829)'
PREVIOUS_SHA256 = '1999b9ab312f4008ff2bb9697794bb1947d1c9d15d8748b8aec929cd3fa6c2fd'
TARGET_VERSION = '1.20.1-managed-preview.3'
FEED_NAME = re.compile(r'[0-9a-fA-F]{32}(?:\.windows)?(?:\.status)?\.json\Z')


class UpgradeError(Exception):
    pass


def same_standard_core(fields, link, profile_id, name):
    """Only lossless encoding of the two unprotected generated core items."""
    try:
        if len(fields) != 88 or fields[:2] != ['33', 'xray']:
            return False
        if any(fields[i] != '0' for i in (31, 33, 34, 36)) or any(fields[i] for i in (32, 35, 37)):
            return False
        inner = base64.b64decode(fields[84], validate=True).decode('utf-8').strip()
        uri = urllib.parse.urlsplit(link)
        if urllib.parse.unquote(uri.fragment) != name:
            return False
        if profile_id == 'managed-ss2022':
            return inner.startswith('ss://') and uri.scheme == 'ss' and inner.split('#', 1)[0] == link.split('#', 1)[0]
        if profile_id != 'managed-awg' or uri.scheme != 'wireguard' or uri.path or not inner.startswith('[Interface]'):
            return False
        values = {}; section = ''
        for line in inner.splitlines():
            line = line.strip()
            if not line:
                continue
            if line in ('[Interface]', '[Peer]'):
                if line == section or line == '[Interface]' and section:
                    return False
                section = line; continue
            key, value = line.split('=', 1)
            key = key.strip().lower()
            if not section or key in values:
                return False
            values[key] = value.strip()
        endpoint = urllib.parse.urlsplit('wireguard://' + values.pop('endpoint'))
        secret = values.pop('privatekey')
        if (endpoint.hostname, endpoint.port) != (uri.hostname, uri.port) or urllib.parse.unquote(uri.username or '') != secret or uri.password is not None:
            return False
        pairs = urllib.parse.parse_qsl(uri.query, keep_blank_values=True, strict_parsing=True)
        return len(pairs) == len(dict(pairs)) and dict(pairs) == values
    except (ValueError, KeyError, IndexError, UnicodeError):
        return False


def digest(path):
    with path.open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def atomic(path, data, mode=0o600):
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix='.' + path.name + '-', dir=path.parent)
    try:
        with os.fdopen(descriptor, 'wb') as output:
            os.fchmod(output.fileno(), mode)
            output.write(data)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, path)
        parent = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(parent)
        finally:
            os.close(parent)
    finally:
        pathlib.Path(temporary).unlink(missing_ok=True)


def write_json(path, value):
    atomic(path, (json.dumps(value, indent=2) + '\n').encode())


@contextlib.contextmanager
def lock_file(path):
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open('a+b') as lock:
        os.chmod(path, 0o600)
        fcntl.flock(lock, fcntl.LOCK_EX)
        try:
            yield
        finally:
            fcntl.flock(lock, fcntl.LOCK_UN)


class Paths:
    def __init__(self, root=pathlib.Path('/')):
        self.config = root / 'etc/rovunet/config.json'
        self.cores = root / 'etc/rovunet/core'
        self.state = root / 'etc/rovunet/managed-preview.json'
        self.receipt = root / 'etc/rovunet/managed-preview-upgrade.json'
        self.lock = root / 'etc/rovunet/.preview-upgrade.lock'
        self.subscriptions = root / 'var/lib/rovunet/subscriptions'
        self.target = root / 'usr/local/bin/rovunet'
        self.backups = root / 'root'


class Host:
    def command(self, args, timeout=15):
        result = subprocess.run([str(x) for x in args], stdout=subprocess.PIPE,
                                stderr=subprocess.STDOUT, timeout=timeout)
        if result.returncode:
            raise UpgradeError('A required command failed; no private command output was printed.')
        return result.stdout.decode(errors='replace').strip()

    def version(self, binary):
        return self.command([binary, '--version'])

    def timer(self, name):
        text = self.command(['systemctl', 'show', name, '--property=ActiveState,UnitFileState'])
        values = dict(line.split('=', 1) for line in text.splitlines() if '=' in line)
        if not values.get('UnitFileState'):
            raise UpgradeError('A required preview timer is missing.')
        return values

    def set_timer(self, name, active):
        if name not in TIMERS:
            raise UpgradeError('Refused to change a non-timer service.')
        self.command(['systemctl', 'start' if active else 'stop', name])

    def wait_idle(self, timer):
        service = timer.removesuffix('.timer') + '.service'
        deadline = time.monotonic() + 45
        while self.command(['systemctl', 'show', service, '--property=ActiveState', '--value']) in (
                'active', 'activating', 'deactivating', 'reloading'):
            if time.monotonic() >= deadline:
                raise UpgradeError('A scheduled job is still running; upgrade was deferred.')
            time.sleep(0.2)

    def services(self):
        listing = self.command(['systemctl', 'list-units', '--all', '--type=service',
                                'rovunet*', '--no-legend', '--plain', '--no-pager'])
        units = sorted(set(re.findall(r'\b(rovunet[\w.@:-]*\.service)\b', listing)))
        ignored = {timer.removesuffix('.timer') + '.service' for timer in TIMERS}
        snapshot = {}
        for unit in units:
            if unit in ignored:
                continue
            text = self.command(['systemctl', 'show', unit,
                                 '--property=Id,ActiveState,SubState,MainPID,InvocationID,UnitFileState'])
            snapshot[unit] = dict(line.split('=', 1) for line in text.splitlines() if '=' in line)
        return snapshot

    def backup(self, old_binary, new_binary, directory):
        with (directory / 'backup-create.log').open('wb') as log:
            for args in ([old_binary, 'backup', '--file', directory / 'panel.backup'],
                         [new_binary, 'backup-check', directory / 'panel.backup']):
                result = subprocess.run([str(x) for x in args], stdout=log, stderr=log, timeout=90)
                if result.returncode:
                    raise UpgradeError('Private backup creation or validation failed; panel was kept.')

    def refresh(self, binary, directory):
        with (directory / 'subscription-refresh.log').open('wb') as log:
            result = subprocess.run([str(binary), 'subscriptions-refresh'], stdout=log,
                                    stderr=log, timeout=90)
            if result.returncode:
                raise UpgradeError('Subscription refresh failed; see the private upgrade log.')


def core_fingerprint(root):
    values = {}
    if not root.is_dir():
        raise UpgradeError('Managed core directory is missing.')
    for path in sorted(root.rglob('*')):
        if path.is_symlink():
            raise UpgradeError('Unexpected core symlink requires inspection.')
        if path.is_file():
            values[path.relative_to(root).as_posix()] = digest(path)
    return values


def check_identities(config):
    if not re.fullmatch(r'[0-9a-fA-F]{32}', config.get('server_id', '')):
        raise UpgradeError('Existing server identity needs migration before this upgrade.')
    for user in config.get('users', []):
        for key in ('sub_token', 'account_id', 'period_id'):
            if not re.fullmatch(r'[0-9a-fA-F]{32}', user.get(key, '')):
                raise UpgradeError('An existing account identity needs migration before this upgrade.')
    for user in config.get('archived', []):
        for key in ('account_id', 'period_id'):
            if not re.fullmatch(r'[0-9a-fA-F]{32}', user.get(key, '')):
                raise UpgradeError('An archived account identity needs migration before this upgrade.')
    if not any(t.get('enabled') and t.get('transport') == 'naive' and t.get('domain')
               for t in config.get('tunnels', [])):
        raise UpgradeError('No active subscription host is configured.')


def expired(user):
    text = user.get('expires_at')
    try:
        return bool(text and datetime.datetime.fromisoformat(text.replace('Z', '+00:00')) <=
                    datetime.datetime.now(datetime.timezone.utc))
    except (ValueError, TypeError):
        return False  # match the panel: an unreadable expiry does not retire an account


class Upgrade:
    def __init__(self, package, paths=None, host=None):
        self.package = pathlib.Path(package).resolve()
        self.paths = paths or Paths()
        self.host = host or Host()
        self.manifest = json.loads((self.package / 'UPGRADE-MANIFEST.json').read_text())
        self.binary = self.package / 'rovunet-linux-amd64'

    def verify_package(self):
        for line in (self.package / 'SHA256SUMS').read_text().splitlines():
            expected, name = line.split('  ', 1)
            path = self.package / name
            if not path.resolve().is_relative_to(self.package) or path.is_symlink() or digest(path) != expected:
                raise UpgradeError('Package checksum verification failed.')
        if digest(self.binary) != self.manifest['panel_sha256']:
            raise UpgradeError('Panel binary checksum mismatch.')
        expected_version = 'rovunet v' + TARGET_VERSION + ' (' + self.manifest['commit'][:7] + ')'
        if self.host.version(self.binary) != expected_version:
            raise UpgradeError('Package binary version differs from its manifest.')

    def marker(self):
        if not self.paths.state.is_file():
            raise UpgradeError('The audited preview deployment marker is missing.')
        return json.loads(self.paths.state.read_text())

    def backup_path(self, receipt):
        path = pathlib.Path(receipt['backup'])
        if path.parent != self.paths.backups or not path.name.startswith('rovunet-preview-upgrade.') or not path.is_dir():
            raise UpgradeError('Invalid private upgrade backup path.')
        return path

    def pause(self, states):
        for timer in states:
            self.host.set_timer(timer, False)
            self.host.wait_idle(timer)

    def resume(self, states):
        failed = False
        for timer, previous in states.items():
            try:
                self.host.set_timer(timer, previous['ActiveState'] == 'active')
                if self.host.timer(timer) != previous:
                    failed = True
            except Exception:
                failed = True
        if failed:
            raise UpgradeError('Timer state could not be restored; rerun apply or revert to recover the private receipt.')

    def resume_receipt(self, receipt, states):
        self.resume(states)
        completed = {'applied-restoring-timers': 'applied', 'reverted-restoring-timers': 'reverted'}
        if receipt.get('phase') in completed:
            receipt['phase'] = completed[receipt['phase']]
            write_json(self.paths.receipt, receipt)

    def snapshot_feeds(self, directory):
        target = directory / 'subscriptions-sub'
        target.mkdir(mode=0o700)
        with lock_file(self.paths.subscriptions / '.write.lock'):
            source = self.paths.subscriptions / 'sub'
            for path in source.iterdir():
                if path.is_file() and not path.is_symlink() and FEED_NAME.fullmatch(path.name):
                    shutil.copyfile(path, target / path.name)
        return sorted(path.name for path in target.iterdir())

    def restore_feeds(self, directory):
        source = directory / 'subscriptions-sub'
        if not source.is_dir():
            return
        target = self.paths.subscriptions / 'sub'
        with lock_file(self.paths.subscriptions / '.write.lock'):
            names = {path.name for path in source.iterdir()}
            for path in source.iterdir():
                atomic(target / path.name, path.read_bytes(), 0o644)
            for path in target.iterdir():
                if FEED_NAME.fullmatch(path.name) and path.name not in names:
                    path.unlink()

    def restore_attempt(self, receipt, restore_feeds=True):
        directory = self.backup_path(receipt)
        marker = self.marker()
        if marker.get('backup') != receipt['installation_backup']:
            raise UpgradeError('Deployment changed; automatic recovery will not overwrite it.')
        current = digest(self.paths.target)
        if current not in (receipt['previous_sha256'], receipt['new_sha256']):
            raise UpgradeError('Panel changed after this upgrade; automatic recovery was refused.')
        previous_binary = directory / 'rovunet.previous'
        if digest(previous_binary) != receipt['previous_sha256']:
            raise UpgradeError('Previous private panel backup failed verification.')
        unchanged = (receipt.get('config_sha256') == digest(self.paths.config) and
                     receipt.get('core_files') == core_fingerprint(self.paths.cores))
        atomic(self.paths.target, previous_binary.read_bytes(), 0o755)
        marker['preview_sha256'] = receipt['previous_sha256']
        write_json(self.paths.state, marker)
        if restore_feeds and unchanged:
            self.restore_feeds(directory)
        receipt['feed_snapshot_restored'] = bool(restore_feeds and unchanged)
        receipt['phase'] = 'reverted-restoring-timers'
        write_json(self.paths.receipt, receipt)

    def recover_pending(self):
        if not self.paths.receipt.is_file():
            return
        receipt = json.loads(self.paths.receipt.read_text())
        if receipt.get('phase') not in ('preparing', 'applying', 'reverting',
                                       'applied-restoring-timers', 'reverted-restoring-timers'):
            return
        if self.marker().get('backup') != receipt.get('installation_backup'):
            return  # a later fresh installation superseded the private receipt
        if receipt['phase'] in ('preparing', 'applying', 'reverting'):
            self.restore_attempt(receipt, restore_feeds=receipt['phase'] != 'reverting')
        self.resume_receipt(receipt, receipt.get('revert_timers', receipt['timers']))

    def check_feeds(self, config, previous_names):
        root = self.paths.subscriptions / 'sub'
        for user in config.get('users', []):
            if expired(user):
                continue
            for suffix in ('.json', '.windows.json'):
                name = user['sub_token'] + suffix
                if name not in previous_names:
                    continue
                path = root / name
                if not path.is_file():
                    raise UpgradeError('A previously active subscription disappeared during refresh.')
                feed = json.loads(path.read_text())
                items = feed.get('items', [])
                if feed.get('version') != 1 or not items or len({x['id'] for x in items}) != len(items):
                    raise UpgradeError('A refreshed subscription failed validation.')

    def check_display_names_only(self, config, receipt):
        if not self.manifest.get('display_names_only'):
            return
        directory = self.backup_path(receipt) / 'subscriptions-sub'
        labels = json.loads((self.package / 'profile-display-names.json').read_text())
        checked = changed = 0
        for user in config.get('users', []):
            if expired(user):
                continue
            for suffix in ('.json', '.windows.json'):
                name = user['sub_token'] + suffix
                if name not in receipt['previous_feeds']:
                    continue
                before = json.loads((directory / name).read_text())
                after = json.loads((self.paths.subscriptions / 'sub' / name).read_text())
                if (before.get('version'), before.get('name')) != (after.get('version'), after.get('name')):
                    raise UpgradeError('Subscription identity changed during the display-name update.')
                old_items, new_items = before['items'], after['items']
                old_ids = [x['id'] for x in old_items]
                allowed = set(self.manifest.get('approved_windows_core_additions', [])) if suffix == '.windows.json' else set()
                added = [x for x in new_items if x['id'] not in old_ids]
                if any(x['id'] not in allowed for x in added) or len({x['id'] for x in new_items}) != len(new_items):
                    raise UpgradeError('Unexpected subscription additions during preview promotion.')
                for item in added:
                    # Old Windows 0.1.0 skips unsupported standard entries but
                    # rejects an unsupported native xray envelope atomically.
                    expected = 'wireguard://' if item['id'] == 'managed-awg' else 'ss://'
                    if not isinstance(item.get('link'), str) or not item['link'].startswith(expected):
                        raise UpgradeError('An approved added Windows profile has an invalid compatible encoding.')
                new_items = [x for x in new_items if x['id'] in old_ids]
                if old_ids != [x['id'] for x in new_items]:
                    raise UpgradeError('Subscription membership or order changed during the display-name update.')
                for old, new in zip(old_items, new_items):
                    try:
                        def fields(item):
                            prefix, payload = item['link'].split('://', 1)
                            if prefix != 'rovunet':
                                raise ValueError()
                            return base64.b64decode(payload, validate=True).decode('utf-8').split('|')
                        a = fields(old)
                        profile_id = re.sub(r'-ip[0-9]+x*$', '', old['id'])
                        expected = labels.get(old['id'], labels.get(profile_id, a[2]))
                        encodings = self.manifest.get('approved_standard_core_encodings', [])
                        if profile_id in encodings and not new['link'].startswith('rovunet://'):
                            if not same_standard_core(a, new['link'], profile_id, expected):
                                raise ValueError()
                            checked += 1
                            changed += a[2] != expected
                            continue
                        b = fields(new)
                        valid = (len(a) == len(b) == 88 and b[2] == expected and
                                 a[:2] == b[:2] and a[3:] == b[3:])
                    except (ValueError, KeyError, IndexError, UnicodeError):
                        valid = False
                    if not valid:
                        raise UpgradeError('A profile changed beyond its approved display name; previous panel and feeds will be restored.')
                    checked += 1
                    changed += a[2] != b[2]
        receipt['display_names_only_verified'] = True
        receipt['profiles_checked'] = checked
        receipt['profile_names_changed'] = changed

    def status(self):
        marker = self.marker()
        if marker.get('preview_sha256') != digest(self.paths.target):
            raise UpgradeError('Panel checksum and deployment marker differ.')
        config = json.loads(self.paths.config.read_text())
        summary = {'version': self.host.version(self.paths.target),
                   'active_accounts': len(config.get('users', [])), 'private_urls_printed': False}
        if self.paths.receipt.is_file():
            receipt = json.loads(self.paths.receipt.read_text())
            if receipt.get('installation_backup') == marker.get('backup'):
                summary['upgrade_phase'] = receipt.get('phase')
                for key in ('display_names_only_verified', 'profiles_checked', 'profile_names_changed'):
                    if key in receipt:
                        summary[key] = receipt[key]
        summary['timers'] = {timer: self.host.timer(timer) for timer in TIMERS}
        for user in config.get('users', []):
            if user.get('quota_gb', 0) or expired(user):
                continue
            root = self.paths.subscriptions / 'sub'
            for client, suffix in (('android', '.json'), ('windows', '.windows.json')):
                path = root / (user['sub_token'] + suffix)
                if path.is_file():
                    summary[client + '_profiles_for_first_unmetered_account'] = len(json.loads(path.read_text()).get('items', []))
            break
        return summary

    def apply(self):
        self.verify_package()
        with lock_file(self.paths.lock):
            self.recover_pending()
            marker = self.marker()
            current = digest(self.paths.target)
            if current == self.manifest['panel_sha256'] and marker.get('preview_sha256') == current:
                return dict(self.status(), already_installed=True)
            if current != self.manifest['previous_sha256'] or marker.get('preview_sha256') != current:
                raise UpgradeError('Expected the audited previous preview; no binary was replaced.')
            if self.host.version(self.paths.target) != PREVIOUS_VERSION:
                raise UpgradeError('Installed version differs from the audited previous preview.')
            check_identities(json.loads(self.paths.config.read_text()))
            self.paths.backups.mkdir(parents=True, exist_ok=True)
            directory = pathlib.Path(tempfile.mkdtemp(prefix='rovunet-preview-upgrade.', dir=self.paths.backups))
            os.chmod(directory, 0o700)
            shutil.copy2(self.paths.target, directory / 'rovunet.previous')
            shutil.copyfile(self.paths.state, directory / 'managed-preview.before.json')
            if self.paths.receipt.is_file():
                shutil.copyfile(self.paths.receipt, directory / 'previous-upgrade-receipt.json')
            states = {timer: self.host.timer(timer) for timer in TIMERS}
            receipt = {'phase': 'preparing', 'backup': str(directory), 'timers': states,
                       'installation_backup': marker['backup'], 'previous_sha256': current,
                       'new_sha256': self.manifest['panel_sha256']}
            write_json(self.paths.receipt, receipt)
            failed = None
            try:
                self.pause(states)
                config = json.loads(self.paths.config.read_text())
                check_identities(config)
                receipt['config_sha256'] = digest(self.paths.config)
                receipt['core_files'] = core_fingerprint(self.paths.cores)
                receipt['services'] = self.host.services()
                shutil.copyfile(self.paths.config, directory / 'config.before.json')
                self.host.backup(self.paths.target, self.binary, directory)
                receipt['previous_feeds'] = self.snapshot_feeds(directory)
                receipt['phase'] = 'applying'
                write_json(self.paths.receipt, receipt)
                atomic(self.paths.target, self.binary.read_bytes(), 0o755)
                marker['preview_sha256'] = receipt['new_sha256']
                write_json(self.paths.state, marker)
                self.host.refresh(self.paths.target, directory)
                if digest(self.paths.config) != receipt['config_sha256'] or core_fingerprint(self.paths.cores) != receipt['core_files']:
                    raise UpgradeError('Account configuration or core files changed during upgrade.')
                if self.host.services() != receipt['services']:
                    raise UpgradeError('A service process or state changed during upgrade; inspect the private backup.')
                self.check_feeds(config, receipt['previous_feeds'])
                self.check_display_names_only(config, receipt)
                receipt['phase'] = 'applied-restoring-timers'
                write_json(self.paths.receipt, receipt)
            except BaseException as error:
                try:
                    self.restore_attempt(receipt)
                except Exception:
                    raise UpgradeError('Automatic panel recovery needs inspection; private receipt was kept.') from None
                failed = error
            finally:
                self.resume_receipt(receipt, states)
            if failed:
                if isinstance(failed, UpgradeError):
                    raise failed
                raise UpgradeError('Upgrade did not finish; the previous panel was restored. Inspect the private backup.') from None
            return dict(self.status(), configuration_unchanged=True, core_files_unchanged=True,
                        service_processes_unchanged=True, backup=str(directory))

    def revert(self):
        self.verify_package()
        with lock_file(self.paths.lock):
            self.recover_pending()
            receipt = json.loads(self.paths.receipt.read_text())
            if receipt.get('phase') == 'reverted':
                return dict(self.status(), already_reverted=True)
            if self.marker().get('backup') != receipt.get('installation_backup'):
                raise UpgradeError('This receipt belongs to another preview installation.')
            states = {timer: self.host.timer(timer) for timer in TIMERS}
            receipt['phase'], receipt['revert_timers'] = 'reverting', states
            write_json(self.paths.receipt, receipt)
            try:
                self.pause(states)
                self.restore_attempt(receipt, restore_feeds=False)
            finally:
                self.resume_receipt(receipt, states)
            return dict(self.status(), previous_panel_restored=True, core_files_retained=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('apply', 'status', 'revert'))
    args = parser.parse_args()
    os.umask(0o077)
    if os.geteuid() != 0 or sys.platform != 'linux' or platform.machine() != 'x86_64':
        raise UpgradeError('This private upgrade requires root on Linux x86_64.')
    upgrade = Upgrade(pathlib.Path(__file__).resolve().parent)
    if upgrade.manifest.get('previous_sha256') != PREVIOUS_SHA256:
        raise UpgradeError('Unexpected upgrade baseline in the manifest.')
    if upgrade.manifest.get('version') != TARGET_VERSION or not upgrade.manifest.get('display_names_only'):
        raise UpgradeError('Expected the private display-name-only upgrade manifest.')
    result = getattr(upgrade, args.action)()
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    try:
        main()
    except UpgradeError as error:
        print('ERROR: ' + str(error), file=sys.stderr)
        sys.exit(1)
    except (Exception, KeyboardInterrupt):
        print('ERROR: Upgrade stopped. Keep the private receipt and backup for inspection; no private error data was printed.', file=sys.stderr)
        sys.exit(1)
