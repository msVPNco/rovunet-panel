#!/usr/bin/env python3
"""Isolated rollback accounting tests; no real services/files are touched."""
import importlib.util
import json
import pathlib
import tempfile
import unittest
import sys

sys.dont_write_bytecode = True

spec = importlib.util.spec_from_file_location('bridge', pathlib.Path(__file__).with_name('preview-rollback-usage.py'))
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)


class UsageBridge(unittest.TestCase):
    def test_live_epoch_not_double_counted_and_stopped_core_history_retained(self):
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp)
            usage, backup = root/'usage', root/'backup'
            usage.mkdir(); backup.mkdir()
            (usage/'preview-history.json').write_text('{"alice":7}')
            def ledger(name, total, last):
                data = dict(epoch='running', totals={'alice':total}, last={'alice>>>uplink':last})
                (usage/f'xray-{name}.source').write_text(json.dumps(data))
            ledger('legacy', 150, 50)
            ledger('new', 40, 40)
            epochs = lambda name: 'running' if name == 'legacy' else ''
            bridge.prepare(usage, backup, epochs)
            self.assertEqual(json.loads((usage/'preview-history.json').read_text()), {'alice':147})
            # The old collector subsequently adds current-epoch 50 once.
            self.assertEqual(147 + 50, 7 + 150 + 40)
            bridge.prepare(usage, backup, epochs)
            self.assertEqual(json.loads((usage/'preview-history.json').read_text()), {'alice':147})
            bridge.restore(usage, backup)
            self.assertEqual(json.loads((usage/'preview-history.json').read_text()), {'alice':7})
            self.assertEqual(len(list(usage.glob('*.source'))), 2)
            # Recover an interrupted operation from its saved journal.
            bridge.prepare(usage, backup, epochs)
            (backup/'rollback-usage-ready').unlink()
            bridge.prepare(usage, backup, epochs)
            self.assertEqual(json.loads((usage/'preview-history.json').read_text()), {'alice':147})


if __name__ == '__main__':
    unittest.main()
