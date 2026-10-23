#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Check every published page, local reference, policy and original notice."""
import hashlib
from html.parser import HTMLParser
import json
from pathlib import Path
from urllib.parse import unquote, urljoin, urlsplit

SITE = Path(__file__).resolve().parent
OUTPUT = SITE / 'dist/db-manager'


class Document(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.references = []
        self.ids = set()
        self.csp = None
        self.referrer = None
        self.language = None
        self.issues = []

    def handle_starttag(self, tag, attributes):
        attrs = dict(attributes)
        if 'id' in attrs:
            self.ids.add(attrs['id'])
        if tag == 'html':
            self.language = attrs.get('lang')
        if tag == 'meta' and attrs.get('http-equiv', '').lower() == 'content-security-policy':
            self.csp = attrs.get('content')
        if tag == 'meta' and attrs.get('name') == 'referrer':
            self.referrer = attrs.get('content')
        for key in ['href', 'src']:
            if key in attrs:
                self.references.append(attrs[key])
        if any(key.startswith('on') for key in attrs) or 'style' in attrs:
            self.issues.append('Inline handler/style')
        if tag == 'script' and not attrs.get('src'):
            self.issues.append('Inline script')
        if tag == 'img' and 'alt' not in attrs:
            self.issues.append('Missing alt')


def main():
    documents = {}
    issues = []
    for path in sorted(OUTPUT.rglob('*.html')):
        text = path.read_text()
        parser = Document()
        parser.feed(text)
        documents[path] = parser
        if parser.language != 'de' or parser.referrer != 'no-referrer':
            issues.append(f'{path.relative_to(OUTPUT)}: missing language/referrer')
        if not parser.csp or "script-src 'self'" not in parser.csp or 'unsafe-' in parser.csp:
            issues.append(f'{path.relative_to(OUTPUT)}: invalid CSP')
        issues.extend(f'{path.relative_to(OUTPUT)}: {issue}' for issue in parser.issues)
        if any(private in text for private in ['/Users/', '/home/runner/', '/private/var/folders/', 'DBM_CAPTURE_CONFIG']):
            issues.append(f'{path.relative_to(OUTPUT)}: private build context')
    references = 0
    for path, doc in documents.items():
        url = '/db-manager/' + path.relative_to(OUTPUT).as_posix()
        for reference in doc.references:
            parsed = urlsplit(reference)
            if parsed.scheme or parsed.netloc:
                if parsed.scheme not in {'https', 'http', 'mailto'}:
                    issues.append(f'{path.relative_to(OUTPUT)}: unsafe scheme')
                continue
            location = urlsplit(urljoin(url, reference))
            if not location.path.startswith('/db-manager/'):
                issues.append(f'{path.relative_to(OUTPUT)}: reference escapes basepath: {reference}')
                continue
            target = OUTPUT / unquote(location.path.removeprefix('/db-manager/'))
            if target.is_dir():
                target /= 'index.html'
            references += 1
            if not target.is_file():
                issues.append(f'{path.relative_to(OUTPUT)}: missing {reference}')
            elif location.fragment and target in documents and unquote(location.fragment) not in documents[target].ids:
                issues.append(f'{path.relative_to(OUTPUT)}: missing anchor {reference}')
    notices = 0
    for inventory in ['python-inventory.json', 'npm-inventory.json']:
        for dependency in json.loads((SITE / 'licenses' / inventory).read_text()):
            for notice in dependency['notices']:
                data = (SITE / notice['path']).read_bytes()
                assert hashlib.sha256(data).hexdigest() == notice['sha256'], notice['path']
                assert (OUTPUT / notice['path']).read_bytes() == data, notice['path']
                notices += 1
    for item, digest in json.loads((SITE / 'assets/screenshots/provenance.json').read_text())['files'].items():
        assert hashlib.sha256((OUTPUT / 'assets/screenshots' / item).read_bytes()).hexdigest() == digest
    assert (OUTPUT / 'licenses/AGPL-3.0-only.txt').read_bytes() == (SITE.parent / 'LICENSE').read_bytes()
    scripts = sorted(p.relative_to(OUTPUT).as_posix() for p in OUTPUT.rglob('*.js'))
    assert scripts == ['assets/site.js'], scripts
    assert not list(OUTPUT.rglob('*.map')), 'Source maps in output'
    if issues:
        print('\n'.join(issues))
        raise SystemExit(f'{len(issues)} site check errors')
    print(f'{len(documents)} pages, {references} local references, {notices} original notices: zero errors')


if __name__ == '__main__':
    main()
