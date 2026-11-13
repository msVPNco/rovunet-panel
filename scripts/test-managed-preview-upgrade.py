#!/usr/bin/env python3
"""Exercise the private upgrade transaction in temporary paths, without systemd."""
import copy
import base64
import importlib.util
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location('upgrade', pathlib.Path(__file__).with_name('upgrade-managed-preview.py'))
upgrade = importlib.util.module_from_spec(spec)
spec.loader.exec_module(upgrade)

TOKEN = '12' * 16
ARCHIVED_TOKEN = '34' * 16
OLD, NEW = b'fixture-previous-panel', b'fixture-new-panel'
COMMIT = 'abcdef0123456789abcdef0123456789abcdef0123'


def feed(count):
    return {'version': 1, 'name': 'fixture', 'items': [
        {'id': 'path-' + str(index), 'link': 'rovunet://fixture-' + str(index)} for index in range(count)]}


class FakeHost:
    def __init__(self, root):
        self.root, self.paths = pathlib.Path(root), upgrade.Paths(pathlib.Path(root) / 'host')
        self.mode, self.busy, self.fail_resume = '', False, False
        self.calls, self.refreshes = [], 0
        self.service_snapshot = {'rovunet-core-managed-awg.service': {
            'ActiveState': 'active', 'MainPID': '12345', 'InvocationID': 'unchanged'}}

    def version(self, binary):
        return (upgrade.PREVIOUS_VERSION if binary.read_bytes() == OLD else
                'rovunet v' + upgrade.TARGET_VERSION + ' (' + COMMIT[:7] + ')')

    def timer(self, name):
        return json.loads((self.root / 'timers.json').read_text())[name]

    def set_timer(self, name, active):
        if name not in upgrade.TIMERS:
            raise AssertionError('upgrade tried to change a network service')
        self.calls.append((name, active))
        if active and self.fail_resume:
            raise upgrade.UpgradeError('fixture timer start failed')
        path = self.root / 'timers.json'
        states = json.loads(path.read_text())
        states[name]['ActiveState'] = 'active' if active else 'inactive'
        upgrade.write_json(path, states)

    def wait_idle(self, timer):
        if self.busy:
            raise upgrade.UpgradeError('fixture job is still running')

    def services(self):
        return copy.deepcopy(self.service_snapshot)

    def backup(self, old_binary, new_binary, directory):
        (directory / 'panel.backup').write_bytes(b'fixture-private-backup')

    def refresh(self, binary, directory):
        self.refreshes += 1
        root = self.paths.subscriptions / 'sub'
        upgrade.write_json(root / (TOKEN + '.json'), feed(24))
        if self.mode == 'crash':
            os._exit(86)  # deliberately bypass finally, as a killed upgrade would
        if self.mode == 'raise':
            raise upgrade.UpgradeError('fixture failed after one feed changed')
        upgrade.write_json(root / (TOKEN + '.windows.json'), feed(22))
        if self.mode == 'config':
            config = json.loads(self.paths.config.read_text())
            config['users'][0]['password'] = 'fixture-new-admin-password'
            upgrade.write_json(self.paths.config, config)
        elif self.mode == 'core':
            (self.paths.cores / 'managed-awg' / 'state.json').write_bytes(b'fixture-new-admin-core-key')
        elif self.mode == 'delete':
            (root / (TOKEN + '.json')).unlink()
        elif self.mode == 'duplicate':
            bad = feed(2)
            bad['items'][1]['id'] = bad['items'][0]['id']
            upgrade.write_json(root / (TOKEN + '.json'), bad)
        elif self.mode == 'service':
            self.service_snapshot[next(iter(self.service_snapshot))]['MainPID'] = '54321'


class Fixture:
    def __init__(self, root, create=True):
        self.root, self.package = pathlib.Path(root), pathlib.Path(root) / 'package'
        self.host = FakeHost(root)
        self.paths = self.host.paths
        if create:
            self.package.mkdir()
            self.paths.backups.mkdir(parents=True)
            self.paths.cores.mkdir(parents=True)
            (self.paths.cores / 'managed-awg').mkdir()
            (self.paths.cores / 'managed-awg' / 'state.json').write_bytes(b'fixture-private-core-key')
            upgrade.atomic(self.paths.target, OLD, 0o755)
            config = {'server_id': 'ab' * 16, 'users': [{
                'username': 'fixture', 'password': 'fixture-private-password', 'sub_token': TOKEN,
                'account_id': 'cd' * 16, 'period_id': 'ef' * 16}],
                'archived': [{'username': 'retired', 'sub_token': ARCHIVED_TOKEN,
                              'account_id': '56' * 16, 'period_id': '78' * 16}],
                'tunnels': [{'transport': 'naive', 'enabled': True, 'domain': 'example.test'}]}
            upgrade.write_json(self.paths.config, config)
            upgrade.write_json(self.paths.state, {'backup': str(self.paths.backups / 'original-preview-backup'),
                                                  'preview_sha256': upgrade.digest(self.paths.target)})
            upgrade.write_json(self.root / 'timers.json', {name: {
                'ActiveState': 'active', 'UnitFileState': 'enabled'} for name in upgrade.TIMERS})
            for suffix in ('.json', '.windows.json', '.status.json', '.windows.status.json'):
                upgrade.write_json(self.paths.subscriptions / 'sub' / (TOKEN + suffix),
                                   feed(20) if 'status' not in suffix else {'state': 'active'})
            upgrade.write_json(self.paths.subscriptions / 'sub' / (ARCHIVED_TOKEN + '.status.json'), {'state': 'expired'})
            upgrade.atomic(self.package / 'rovunet-linux-amd64', NEW, 0o755)
            manifest = {'commit': COMMIT, 'panel_sha256': upgrade.digest(self.package / 'rovunet-linux-amd64'),
                        'previous_sha256': upgrade.digest(self.paths.target)}
            upgrade.write_json(self.package / 'UPGRADE-MANIFEST.json', manifest)
            self.checksums()
        self.operation = upgrade.Upgrade(self.package, self.paths, self.host)

    def checksums(self):
        files = sorted(path for path in self.package.iterdir() if path.name != 'SHA256SUMS')
        (self.package / 'SHA256SUMS').write_text(''.join(
            upgrade.digest(path) + '  ' + path.name + '\n' for path in files))

    def feeds(self):
        return {path.name: path.read_bytes() for path in (self.paths.subscriptions / 'sub').iterdir()}

    def receipt(self):
        return json.loads(self.paths.receipt.read_text())


class UpgradeTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.fixture = Fixture(pathlib.Path(temporary.name))
        self.f = self.fixture

    def test_apply_keeps_config_core_processes_and_urls_and_is_idempotent(self):
        config, cores = self.f.paths.config.read_bytes(), upgrade.core_fingerprint(self.f.paths.cores)
        names, markers = set(self.f.feeds()), json.loads(self.f.paths.state.read_text())
        states = json.loads((self.f.root / 'timers.json').read_text())
        result = self.f.operation.apply()
        self.assertEqual(self.f.paths.config.read_bytes(), config)
        self.assertEqual(upgrade.core_fingerprint(self.f.paths.cores), cores)
        self.assertEqual(set(self.f.feeds()), names)
        self.assertEqual(self.f.paths.target.read_bytes(), NEW)
        self.assertEqual(json.loads(self.f.paths.state.read_text())['backup'], markers['backup'])
        self.assertEqual(self.f.receipt()['phase'], 'applied')
        self.assertEqual(json.loads((self.f.root / 'timers.json').read_text()), states)
        self.assertTrue(result['service_processes_unchanged'])
        self.assertEqual(result['android_profiles_for_first_unmetered_account'], 24)
        self.assertEqual(result['windows_profiles_for_first_unmetered_account'], 22)
        self.assertNotIn(TOKEN, json.dumps(result))
        self.assertNotIn('fixture-private-password', json.dumps(result))
        count = len(list(self.f.paths.backups.iterdir()))
        self.assertTrue(self.f.operation.apply()['already_installed'])
        self.assertEqual(len(list(self.f.paths.backups.iterdir())), count)
        self.assertEqual(self.f.host.refreshes, 1)
        for call in self.f.host.calls:
            self.assertIn(call[0], upgrade.TIMERS)

    def test_failure_after_partial_refresh_restores_feeds_binary_and_timers(self):
        before = self.f.feeds()
        self.f.host.mode = 'raise'
        with self.assertRaises(upgrade.UpgradeError):
            self.f.operation.apply()
        self.assertEqual(self.f.paths.target.read_bytes(), OLD)
        self.assertEqual(self.f.feeds(), before)
        self.assertTrue(self.f.receipt()['feed_snapshot_restored'])
        self.assertEqual(self.f.receipt()['phase'], 'reverted')
        self.assertTrue(all(self.f.host.timer(name)['ActiveState'] == 'active' for name in upgrade.TIMERS))

    def test_changed_config_is_retained_and_old_feeds_are_not_restored(self):
        self.f.host.mode = 'config'
        with self.assertRaises(upgrade.UpgradeError):
            self.f.operation.apply()
        self.assertEqual(self.f.paths.target.read_bytes(), OLD)
        self.assertEqual(json.loads(self.f.paths.config.read_text())['users'][0]['password'], 'fixture-new-admin-password')
        self.assertFalse(self.f.receipt()['feed_snapshot_restored'])
        self.assertEqual(len(json.loads(self.f.feeds()[TOKEN + '.json'])['items']), 24)

    def test_changed_core_is_retained(self):
        self.f.host.mode = 'core'
        with self.assertRaises(upgrade.UpgradeError):
            self.f.operation.apply()
        self.assertEqual(self.f.paths.target.read_bytes(), OLD)
        self.assertEqual((self.f.paths.cores / 'managed-awg' / 'state.json').read_bytes(), b'fixture-new-admin-core-key')
        self.assertFalse(self.f.receipt()['feed_snapshot_restored'])

    def test_service_change_is_reported_without_restarting_it(self):
        self.f.host.mode = 'service'
        with self.assertRaises(upgrade.UpgradeError):
            self.f.operation.apply()
        self.assertEqual(self.f.paths.target.read_bytes(), OLD)
        self.assertEqual(self.f.host.service_snapshot[next(iter(self.f.host.service_snapshot))]['MainPID'], '54321')

    def test_missing_or_duplicate_feed_rolls_back(self):
        before = self.f.feeds()
        for mode in ('delete', 'duplicate'):
            self.f.host.mode = mode
            with self.assertRaises(upgrade.UpgradeError):
                self.f.operation.apply()
            self.assertEqual(self.f.feeds(), before)
            self.assertEqual(self.f.paths.target.read_bytes(), OLD)

    def test_foreign_binary_is_rejected_before_pausing_timers(self):
        self.f.paths.target.write_bytes(b'another-admin-binary')
        with self.assertRaises(upgrade.UpgradeError):
            self.f.operation.apply()
        self.assertEqual(self.f.paths.target.read_bytes(), b'another-admin-binary')
        self.assertEqual(self.f.host.calls, [])

    def test_archived_identity_is_checked_before_refresh_can_mint_it(self):
        config = json.loads(self.f.paths.config.read_text())
        del config['archived'][0]['period_id']
        upgrade.write_json(self.f.paths.config, config)
        with self.assertRaises(upgrade.UpgradeError):
            self.f.operation.apply()
        self.assertEqual(self.f.host.calls, [])
        self.assertFalse(self.f.paths.receipt.exists())

    def test_busy_scheduled_job_defers_upgrade_and_restores_timers(self):
        self.f.host.busy = True
        with self.assertRaises(upgrade.UpgradeError):
            self.f.operation.apply()
        self.assertEqual(self.f.paths.target.read_bytes(), OLD)
        self.assertTrue(all(self.f.host.timer(name)['ActiveState'] == 'active' for name in upgrade.TIMERS))

    def test_initial_inactive_disabled_timer_is_preserved(self):
        states = json.loads((self.f.root / 'timers.json').read_text())
        states[upgrade.TIMERS[0]] = {'ActiveState': 'inactive', 'UnitFileState': 'disabled'}
        upgrade.write_json(self.f.root / 'timers.json', states)
        self.f.operation.apply()
        self.assertEqual(json.loads((self.f.root / 'timers.json').read_text()), states)

    def test_timer_restore_failure_is_recoverable_after_success(self):
        self.f.host.fail_resume = True
        with self.assertRaises(upgrade.UpgradeError):
            self.f.operation.apply()
        self.assertEqual(self.f.receipt()['phase'], 'applied-restoring-timers')
        self.f.host.fail_resume = False
        result = self.f.operation.apply()
        self.assertTrue(result['already_installed'])
        self.assertEqual(self.f.host.refreshes, 1)
        self.assertEqual(self.f.receipt()['phase'], 'applied')
        self.assertTrue(all(self.f.host.timer(name)['ActiveState'] == 'active' for name in upgrade.TIMERS))

    def test_timer_restore_failure_is_recoverable_after_failed_refresh(self):
        before = self.f.feeds()
        self.f.host.mode, self.f.host.fail_resume = 'raise', True
        with self.assertRaises(upgrade.UpgradeError):
            self.f.operation.apply()
        self.assertEqual(self.f.receipt()['phase'], 'reverted-restoring-timers')
        self.f.host.mode, self.f.host.fail_resume = '', False
        self.f.operation.recover_pending()
        self.assertEqual(self.f.receipt()['phase'], 'reverted')
        self.assertEqual(self.f.feeds(), before)
        self.assertTrue(all(self.f.host.timer(name)['ActiveState'] == 'active' for name in upgrade.TIMERS))

    def test_killed_process_recovers_private_journal_before_retry(self):
        before = self.f.feeds()
        child = subprocess.run([sys.executable, __file__, '--crash-fixture', str(self.f.root)],
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=20)
        self.assertEqual(child.returncode, 86, 'crash fixture did not exit at the intended boundary')
        self.assertEqual(self.f.receipt()['phase'], 'applying')
        self.assertEqual(self.f.paths.target.read_bytes(), NEW)
        self.assertTrue(all(self.f.host.timer(name)['ActiveState'] == 'inactive' for name in upgrade.TIMERS))
        self.f.operation.recover_pending()
        self.assertEqual(self.f.paths.target.read_bytes(), OLD)
        self.assertEqual(self.f.feeds(), before)
        self.assertTrue(all(self.f.host.timer(name)['ActiveState'] == 'active' for name in upgrade.TIMERS))
        self.assertTrue(self.f.operation.apply()['configuration_unchanged'])

    def test_revert_keeps_refreshed_feeds_and_later_account_or_core_edits(self):
        self.f.operation.apply()
        feeds = self.f.feeds()
        config = json.loads(self.f.paths.config.read_text())
        config['users'][0]['password'] = 'fixture-later-admin-password'
        upgrade.write_json(self.f.paths.config, config)
        (self.f.paths.cores / 'managed-awg' / 'state.json').write_bytes(b'fixture-later-core-key')
        result = self.f.operation.revert()
        self.assertTrue(result['previous_panel_restored'])
        self.assertEqual(self.f.paths.target.read_bytes(), OLD)
        self.assertEqual(self.f.feeds(), feeds)
        self.assertEqual(json.loads(self.f.paths.config.read_text())['users'][0]['password'], 'fixture-later-admin-password')
        self.assertEqual((self.f.paths.cores / 'managed-awg' / 'state.json').read_bytes(), b'fixture-later-core-key')
        self.assertTrue(self.f.operation.revert()['already_reverted'])

    def test_interrupted_revert_recovers_without_restoring_old_feeds(self):
        self.f.operation.apply()
        feeds = self.f.feeds()
        receipt = self.f.receipt()
        receipt['phase'] = 'reverting'
        receipt['revert_timers'] = {name: self.f.host.timer(name) for name in upgrade.TIMERS}
        upgrade.write_json(self.f.paths.receipt, receipt)
        self.f.operation.pause(receipt['revert_timers'])
        self.f.operation.recover_pending()
        self.assertEqual(self.f.paths.target.read_bytes(), OLD)
        self.assertEqual(self.f.feeds(), feeds)
        self.assertEqual(self.f.receipt()['phase'], 'reverted')

    def test_package_corruption_is_rejected_before_host_changes(self):
        (self.f.package / 'rovunet-linux-amd64').write_bytes(b'corrupt')
        with self.assertRaises(upgrade.UpgradeError):
            self.f.operation.apply()
        self.assertEqual(self.f.paths.target.read_bytes(), OLD)
        self.assertEqual(self.f.host.calls, [])

    def names_fixture(self, fixture, tamper=''):
        labels = {'path-0': 'Approved · SOCKS5'}
        upgrade.write_json(fixture.package / 'profile-display-names.json', labels)
        manifest = json.loads((fixture.package / 'UPGRADE-MANIFEST.json').read_text())
        manifest['display_names_only'] = True
        upgrade.write_json(fixture.package / 'UPGRADE-MANIFEST.json', manifest)
        fixture.operation.manifest = manifest
        fixture.checksums()
        for suffix in ('.json', '.windows.json'):
            source = feed(2)
            for i, item in enumerate(source['items']):
                fields = [''] * 88
                fields[0], fields[1], fields[2] = '33', 'xray', 'previous-' + str(i)
                fields[3], fields[12], fields[84], fields[85] = 'fixture-endpoint', 'fixture-credential', 'fixture-embedded-profile', 'fixture-collection'
                item['link'] = 'rovunet://' + base64.b64encode('|'.join(fields).encode()).decode()
            upgrade.write_json(fixture.paths.subscriptions / 'sub' / (TOKEN + suffix), source)

        def refresh(binary, directory):
            for suffix in ('.json', '.windows.json'):
                path = fixture.paths.subscriptions / 'sub' / (TOKEN + suffix)
                document = json.loads(path.read_text())
                for item in document['items']:
                    fields = base64.b64decode(item['link'].split('://', 1)[1]).decode().split('|')
                    fields[2] = labels.get(item['id'], fields[2])
                    if tamper.startswith('field-'):
                        fields[int(tamper.split('-')[1])] = 'unexpected-change'
                    if tamper == 'unapproved-name':
                        fields[2] = 'unapproved'
                    item['link'] = 'rovunet://' + base64.b64encode('|'.join(fields).encode()).decode()
                if tamper == 'item-id':
                    document['items'][0]['id'] = 'different-id'
                elif tamper == 'order':
                    document['items'].reverse()
                elif tamper == 'count':
                    document['items'].pop()
                elif tamper == 'collection-name':
                    document['name'] = 'different-collection'
                upgrade.write_json(path, document)
        fixture.host.refresh = refresh

    def test_display_name_guard_accepts_only_approved_names_and_preserves_custom_names(self):
        self.names_fixture(self.f)
        result = self.f.operation.apply()
        self.assertTrue(result['display_names_only_verified'])
        self.assertEqual(result['profiles_checked'], 4)
        self.assertEqual(result['profile_names_changed'], 2)

    def test_public_promotion_allows_only_the_two_reviewed_windows_additions(self):
        self.names_fixture(self.f)
        manifest=self.f.operation.manifest
        manifest['approved_windows_core_additions']=['managed-awg','managed-ss2022']
        upgrade.write_json(self.f.package/'UPGRADE-MANIFEST.json',manifest)
        self.f.checksums()
        refresh=self.f.host.refresh
        def add_windows(binary,directory):
            refresh(binary,directory)
            path=self.f.paths.subscriptions/'sub'/(TOKEN+'.windows.json')
            document=json.loads(path.read_text())
            for profile_id,inner in [('managed-awg','wireguard://fixture'),('managed-ss2022','ss://fixture')]:
                document['items'].append({'id':profile_id,'link':inner})
            upgrade.write_json(path,document)
        self.f.host.refresh=add_windows
        self.assertTrue(self.f.operation.apply()['display_names_only_verified'])
        self.assertEqual(len(json.loads(self.f.feeds()[TOKEN+'.windows.json'])['items']),4)

    def test_display_name_guard_rolls_back_any_connection_identity_or_unapproved_name_change(self):
        for tamper in ('field-0', 'field-3', 'field-12', 'field-84', 'field-85', 'unapproved-name', 'item-id', 'order', 'count', 'collection-name'):
            with self.subTest(tamper=tamper), tempfile.TemporaryDirectory() as root:
                fixture = Fixture(pathlib.Path(root))
                self.names_fixture(fixture, tamper)
                before = fixture.feeds()
                with self.assertRaises(upgrade.UpgradeError):
                    fixture.operation.apply()
                self.assertEqual(fixture.paths.target.read_bytes(), OLD)
                self.assertEqual(fixture.feeds(), before)
                self.assertEqual(fixture.receipt()['phase'], 'reverted')


if __name__ == '__main__':
    if len(sys.argv) == 3 and sys.argv[1] == '--crash-fixture':
        fixture = Fixture(sys.argv[2], create=False)
        fixture.host.mode = 'crash'
        fixture.operation.apply()
    else:
        unittest.main(verbosity=2)
