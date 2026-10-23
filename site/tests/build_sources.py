#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Exercise roadmap extraction against actual immutable project-status profiles."""
import importlib.util
from pathlib import Path
import subprocess
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('pages_build', ROOT / 'site/build.py')
build = importlib.util.module_from_spec(spec)
spec.loader.exec_module(build)


def git_status(commit):
    return subprocess.check_output(['git', 'show', f'{commit}:docs/STATUS.md'], cwd=ROOT, text=True)


class AcceptedSourceTests(unittest.TestCase):
    def test_actual_legacy_status_profile(self):
        source = git_status('8708c3ed1dcc6b45c9a96f46e1e498795bacc05b')
        self.assertIn('## Abgenommen und integriert', source)
        accepted = build.accepted_results(source)
        self.assertIn('| B1 |', accepted)
        self.assertNotIn('| B4a |', accepted)
        self.assertNotIn('## Freigegebene Verträge', accepted)

    def test_actual_current_status_profile(self):
        source = git_status('1b02ea30447f6ff100ef00a24b7e174bb8618cd0')
        self.assertIn('## Abgenommene Inkremente und historische Nachweise', source)
        accepted = build.accepted_results(source)
        self.assertIn('| B4a |', accepted)
        self.assertIn('API0.6', accepted)
        self.assertNotIn('## Freigegebene Verträge', accepted)

    def test_current_working_tree_and_heading_changes_preserve_source_table(self):
        source = (ROOT / 'docs/STATUS.md').read_text()
        accepted = build.accepted_results(source)
        self.assertIn('| B4a |', accepted)
        for heading in ['Abgenommen und integriert', 'Verifizierte Projektergebnisse']:
            renamed = source.replace('Abgenommene Inkremente und historische Nachweise', heading)
            self.assertEqual(build.accepted_results(renamed), accepted)
        self.assertTrue((ROOT / 'docs/ARBEITSPLAN.md').read_text().startswith('# Arbeitsplan'))
        self.assertTrue((ROOT / 'docs/TEAM.md').read_text().startswith('# Entwicklungsteam'))

    def test_missing_or_ambiguous_source_is_rejected(self):
        with self.assertRaisesRegex(ValueError, 'no accepted-results table'):
            build.accepted_results('# Status\n\n## Andere Ergebnisse\nKeine Abnahmeevidenz.\n')
        source = (ROOT / 'docs/STATUS.md').read_text()
        with self.assertRaisesRegex(ValueError, 'ambiguous accepted-results tables'):
            build.accepted_results(source + '\n## Zweite Ergebnistabelle\n' + build.accepted_results(source))


if __name__ == '__main__':
    unittest.main()
