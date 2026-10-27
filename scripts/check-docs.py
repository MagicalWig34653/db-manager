#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Check bootstrap documentation without third-party dependencies."""
from pathlib import Path
import re
import sys
import unicodedata

root = Path(__file__).resolve().parent.parent
ignored = {'.git', '.build', 'node_modules', '.worktrees', '.claude', '.idea', '.generated', '.venv', 'dist'}
errors = []
PRIVATE_MARKERS = ('/Users/', '/home/runner/', '/private/var/folders/', 'DBM_CAPTURE_CONFIG')


def slugify(text):
    # Mirrors the Python-Markdown toc slug used by the MkDocs site build (umlauts lose their dots).
    text = re.sub(r'`([^`]*)`|\[([^\]]*)\]\([^)]*\)', lambda m: m.group(1) or m.group(2), text)
    text = unicodedata.normalize('NFKD', text).encode('ascii', 'ignore').decode('ascii')
    return re.sub(r'[-\s]+', '-', re.sub(r'[^\w\s-]', '', text.strip().lower())).strip('-')


def heading_slugs(path):
    slugs, seen, fenced = set(), {}, False
    for line in path.read_text().splitlines():
        if line.lstrip().startswith(('```', '~~~')):
            fenced = not fenced
        match = None if fenced else re.match(r'#{1,6}\s+(.*?)\s*#*\s*$', line)
        if match:
            slug = slugify(match.group(1))
            count = seen.get(slug, 0)
            seen[slug] = count + 1
            slugs.add(slug if not count else f'{slug}_{count}')
    return slugs


files = [p for p in root.rglob('*') if p.is_file() and not ignored.intersection(p.relative_to(root).parts)]
for path in files:
    if path.suffix != '.md':
        continue
    content = path.read_text()
    if re.search(r'/Users/[^/\s]+/|gh[pousr]_[A-Za-z0-9]{20,}|-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY', content):
        errors.append(f'{path.relative_to(root)}: private path or credential marker')
    in_docs = path.relative_to(root).parts[0] == 'docs'
    if in_docs and any(marker in content for marker in PRIVATE_MARKERS):
        errors.append(f'{path.relative_to(root)}: private build context marker (use a paraphrase, not a literal path)')
    for link in re.findall(r'\]\(([^)]+)\)', content):
        if link.startswith(('https://', 'http://', 'mailto:')):
            continue
        target, _, fragment = link.partition('#')
        if target and not (path.parent / target).exists():
            errors.append(f'{path.relative_to(root)}: missing link {link}')
            continue
        if in_docs and fragment:
            if not fragment.isascii():
                errors.append(f'{path.relative_to(root)}: non-ASCII anchor {link}')
                continue
            dest = path.parent / target if target else path
            if dest.suffix == '.md' and fragment not in heading_slugs(dest):
                errors.append(f'{path.relative_to(root)}: missing anchor {link}')
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
