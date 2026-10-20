#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Check bootstrap documentation without third-party dependencies."""
from pathlib import Path
import re
import sys

root = Path(__file__).resolve().parent.parent
ignored = {'.git', '.build', 'node_modules', '.worktrees', '.claude', '.idea'}
errors = []
files = [p for p in root.rglob('*') if p.is_file() and not ignored.intersection(p.relative_to(root).parts)]
for path in files:
    if path.suffix != '.md':
        continue
    content = path.read_text()
    if re.search(r'/Users/[^/\s]+/|gh[pousr]_[A-Za-z0-9]{20,}|-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY', content):
        errors.append(f'{path.relative_to(root)}: private path or credential marker')
    for link in re.findall(r'\]\(([^)]+)\)', content):
        if link.startswith(('https://', 'http://', 'mailto:', '#')):
            continue
        target = link.split('#', 1)[0]
        if target and not (path.parent / target).exists():
            errors.append(f'{path.relative_to(root)}: missing link {link}')
required = ['LICENSE', 'README.md', 'AGENTS.md', 'CLAUDE.md', 'CLAUDE_STARTPROMPT.md',
            'EINRICHTUNG.md', 'docs/STATUS.md', 'docs/ARBEITSPLAN.md',
            'docs/ENTWICKLUNGSABLAUF.md', 'docs/LIZENZEN.md']
for name in required:
    if not (root / name).is_file():
        errors.append(f'missing required file: {name}')
if (root / 'LICENSE').is_file():
    license_text = (root / 'LICENSE').read_text()
    if 'GNU AFFERO GENERAL PUBLIC LICENSE' not in license_text or 'Version 3, 19 November 2007' not in license_text:
        errors.append('LICENSE: expected AGPLv3 text')
    if len(license_text) < 30000:
        errors.append('LICENSE: incomplete license text')
for name in ['README.md', 'AGENTS.md', 'docs/LIZENZEN.md']:
    if (root / name).exists() and 'AGPL-3.0-only' not in (root / name).read_text():
        errors.append(f'{name}: missing AGPL-3.0-only declaration')
for error in errors:
    print(error, file=sys.stderr)
print(f'{len(files)} files checked; {len(errors)} errors')
sys.exit(bool(errors))
