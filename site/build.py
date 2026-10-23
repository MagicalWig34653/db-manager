#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Stage canonical sources and build a static MkDocs site; no network access."""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
from urllib.parse import quote, unquote, urlsplit

SITE = Path(__file__).resolve().parent
ROOT = SITE.parent
STAGE = SITE / '.generated'
REPO = 'https://github.com/MagicalWig34653/db-manager'


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def accepted_results(status):
    """Read the unique canonical outcome table, independent of section wording."""
    header = re.compile(r'^\|[ \t]*Paket[ \t]*\|[ \t]*Tatsächliches Ergebnis[ \t]*\|[ \t]*Nachweis[ \t]*\|[ \t]*$', re.M)
    sections = re.finditer(r'^##[ \t]+[^\n]+\n(.*?)(?=^##[ \t]+|\Z)', status, re.M | re.S)
    candidates = [section.group(1).strip() for section in sections if header.search(section.group(1))]
    if not candidates:
        raise ValueError('Canonical status has no accepted-results table (Paket / Tatsächliches Ergebnis / Nachweis)')
    if len(candidates) != 1:
        raise ValueError('Canonical status has ambiguous accepted-results tables')
    return candidates[0]


def main():
    commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    shutil.rmtree(STAGE, ignore_errors=True)
    STAGE.mkdir()
    sources = [ROOT / 'README.md', ROOT / 'EINRICHTUNG.md', *sorted((ROOT / 'docs').rglob('*.md')), *sorted((ROOT / 'contracts').glob('*.md'))]
    mapping = {}
    for source in sources:
        relative = source.relative_to(ROOT)
        target = relative
        if relative == Path('README.md'):
            target = Path('projekt.md')
        elif relative.name == 'README.md':
            target = relative.with_name('index.md')
        mapping[source.resolve()] = target

    def rewrite(text, source, target):
        def link(match):
            label, destination = match.group(1), match.group(2)
            parts = urlsplit(destination)
            if parts.scheme or parts.netloc or destination.startswith('#'):
                return match.group(0)
            resolved = (source.parent / unquote(parts.path)).resolve()
            fragment = ('#' + parts.fragment) if parts.fragment else ''
            if resolved in mapping:
                path = Path(os.path.relpath(mapping[resolved], target.parent)).as_posix()
                return f'[{label}]({quote(path, safe="/.")}{fragment})'
            try:
                root_path = resolved.relative_to(ROOT).as_posix()
            except ValueError:
                raise ValueError('Canonical link escapes repository') from None
            kind = 'tree' if resolved.is_dir() else 'blob'
            return f'[{label}]({REPO}/{kind}/{commit}/{quote(root_path, safe="/.")}{fragment})'
        # Markdown is rendered by MkDocs/Python-Markdown. Only repository link
        # destinations are adapted to the staged source layout here.
        return re.sub(r'(?<!!)\[([^\]\n]+)\]\(([^\s)]+)\)', link, text)

    source_hashes = {}
    for source, target in mapping.items():
        path = STAGE / target
        path.parent.mkdir(parents=True, exist_ok=True)
        relative = source.relative_to(ROOT).as_posix()
        front = f'---\ncanonical_source: {REPO}/blob/{commit}/{quote(relative, safe="/.")}\nsource_path: {relative}\n---\n\n'
        path.write_text(front + rewrite(source.read_text(), source, target))
        source_hashes[relative] = digest(source)
    for source in (SITE / 'pages').glob('*.md.in'):
        shutil.copy2(source, STAGE / source.name.removesuffix('.in'))
    status = (ROOT / 'docs/STATUS.md').read_text()
    accepted = accepted_results(status)
    roadmap = '''# Roadmap mit belegtem Stand

Diese Übersicht zeigt abgenommene Ergebnisse aus dem aktuellen kanonischen Status.
Sie ist keine Termin- oder Prozentplanung. Der [Arbeitsplan](docs/ARBEITSPLAN.md)
enthält laufende Pakete, Dateibesitz und Reviewgates; die
[Paritätsmatrix](docs/PARITAET.md) beschreibt die noch offenen Produktziele.

## Bereits abgenommen und integriert

'''
    # Link paths in the extracted section are relative to docs/STATUS.md.
    roadmap += rewrite(accepted, ROOT / 'docs/STATUS.md', Path('roadmap.md'))
    roadmap += '''
## Nächste Schritte bleiben einzelne Freigaben

- Strukturmetadaten sind lesend freigegeben. Weitere Schemafunktionen erfordern
  eigene Verträge, Integritätsprüfungen und eine getrennte Clientabnahme.
- Import/Export, Verlauf/Favoriten und weitere native Netzwerkadapter sind
  Paritätsziele; verfügbare Teilfunktionen und Produktbindung stehen im Status.
- Der isolierte Tresorkern ersetzt keinen produktiven, nicht zurückrollbaren
  Anchor. Appintegration, signierte Plattform und tatsächliche Speicherung von
  Zugangsdaten sind getrennte Gates.
- Native Geräte-, Signing- und OS-UI-Nachweise bleiben unabhängig von einem
  erfolgreichen Simulator- oder Plattformbuild.

## Den vollständigen Kontext nachlesen

[Tatsächlicher Status](docs/STATUS.md) · [Arbeitsplan](docs/ARBEITSPLAN.md) ·
[Paritätsziele](docs/PARITAET.md) · [Entscheidungen](docs/decisions/ADR-0001-projektgedaechtnis.md)
'''
    (STAGE / 'roadmap.md').write_text(roadmap + f'\nBuildquelle: [{commit[:12]}]({REPO}/tree/{commit}). Historische Nachweise sind keine neue Freigabe.\n')
    shutil.copytree(SITE / 'assets', STAGE / 'assets')
    shutil.copytree(SITE / 'licenses', STAGE / 'licenses')
    shutil.copy2(ROOT / 'LICENSE', STAGE / 'licenses/AGPL-3.0-only.txt')
    (STAGE / 'lizenzen.md').write_text((SITE / 'LIZENZEN.md').read_text() + f'\n\nWebsite- und Dokumentationsquelle: [{commit[:12]}]({REPO}/tree/{commit}).\n\n[Maschinenlesbare Buildherkunft](build-info.json).\n')
    info = {'schema_version': 1, 'repository': REPO, 'commit': commit, 'canonical_sources': source_hashes,
            'screenshots': json.loads((SITE / 'assets/screenshots/provenance.json').read_text())}
    (STAGE / 'build-info.json').write_text(json.dumps(info, indent=2, sort_keys=True) + '\n')
    subprocess.run([str(SITE / '.venv/bin/python'), '-m', 'mkdocs', 'build', '--clean', '--config-file', str(SITE / 'mkdocs.yml')], check=True, cwd=SITE)
    # The search index is consumed by our text-node-only client. No bundled
    # theme, Lunr runtime, worker or third-party JavaScript is shipped.
    for path in (SITE / 'dist/db-manager/search').iterdir():
        if path.name != 'search_index.json':
            if path.is_dir():
                shutil.rmtree(path)
            else:
                path.unlink()
    (SITE / 'dist/db-manager/.nojekyll').touch()
    print(f'Built {len(sources)} canonical documents at /db-manager/ from {commit[:12]}')


if __name__ == '__main__':
    main()
