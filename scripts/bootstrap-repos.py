#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Install prepared repository templates; optionally commit and publish them."""
import argparse
import json
from pathlib import Path
import shutil
import subprocess


def run(args, cwd=None, capture=False):
    return subprocess.run(args, cwd=cwd, check=True, text=True,
                          stdout=subprocess.PIPE if capture else None).stdout


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--workspace', type=Path, required=True,
                        help='Parent of GoLand, PhpstormProjects and Xcode')
    parser.add_argument('--apply', action='store_true', help='Install templates and initialize Git')
    parser.add_argument('--commit', action='store_true', help='Commit the reviewed bootstrap using existing Git identity')
    parser.add_argument('--publish', action='store_true', help='Create public GitHub repos and push; requires --commit')
    parser.add_argument('--owner', default='MagicalWig34653')
    args = parser.parse_args()
    if args.commit and not args.apply:
        parser.error('--commit requires --apply')
    if args.publish and not args.commit:
        parser.error('--publish requires --commit')
    root = Path(__file__).resolve().parent.parent
    workspace = args.workspace.expanduser().resolve()
    destinations = {'db-manager': workspace / 'GoLand/db-manager',
                    'db-manager-web': workspace / 'PhpstormProjects/db-manager-web',
                    'db-manager-apple': workspace / 'Xcode/db-manager-apple'}
    if destinations['db-manager'].resolve() != root:
        parser.error('Run the script from the intended main repository; workspace does not match')
    manifest = json.loads((root / 'bootstrap/manifest.json').read_text())
    for entry in manifest['main_files']:
        path = Path(entry)
        if path.is_absolute() or '..' in path.parts or not (root / path).is_file() or (root / path).is_symlink():
            raise SystemExit(f'Invalid or missing manifest file: {entry}')
    plans = []
    allowed_by_repo = {}
    for name, target in destinations.items():
        if name == 'db-manager':
            allowed = set(manifest['main_files'])
        else:
            allowed = {str(p.relative_to(root / 'bootstrap/repos' / name))
                       for p in (root / 'bootstrap/repos' / name).rglob('*') if p.is_file()}
        allowed_by_repo[name] = allowed
        if name != 'db-manager':
            source = root / 'bootstrap/repos' / name
            for file in source.rglob('*'):
                if file.is_symlink():
                    raise SystemExit('Symlinks are not allowed in templates')
                if not file.is_file():
                    continue
                dest = target / file.relative_to(source)
                if dest.exists() and (not dest.is_file() or dest.read_bytes() != file.read_bytes()):
                    raise SystemExit(f'Refusing to overwrite differing file: {dest}')
                if any(parent.is_symlink() for parent in [dest, *dest.parents]):
                    raise SystemExit(f'Refusing symlink destination: {dest}')
                plans.append((file, dest))
        if name != 'db-manager' and target.exists():
            for existing in target.rglob('*'):
                relative = existing.relative_to(target)
                if '.git' in relative.parts:
                    continue
                if existing.is_symlink():
                    raise SystemExit(f'Refusing existing symlink: {existing}')
                if existing.is_file() and str(relative) not in allowed and existing.name != '.DS_Store':
                    raise SystemExit(f'Unknown existing file; inspect manually: {existing}')
        if (target / '.git').exists():
            has_commit = subprocess.run(['git', 'rev-parse', '--verify', 'HEAD'], cwd=target,
                                        capture_output=True).returncode == 0
            if has_commit and run(['git', 'status', '--porcelain'], target, capture=True).strip():
                raise SystemExit(f'Existing committed repository has changes; integrate manually: {target}')
            if not has_commit:
                staged = run(['git', 'diff', '--cached', '--name-only', '-z'], target, capture=True)
                if any(path and path not in allowed for path in staged.split('\0')):
                    raise SystemExit(f'Unknown staged files; inspect manually: {target}')
            if run(['git', 'symbolic-ref', '--short', 'HEAD'], target, capture=True).strip() != 'main':
                raise SystemExit(f'Existing repository is not on main: {target}')
        print(f'{name}: {target}')
    print(f'{len(plans)} client files to install. No changes without --apply.')
    if not args.apply:
        return
    if args.publish:
        login = run(['gh', 'api', 'user', '--jq', '.login'], capture=True).strip()
        if login.lower() != args.owner.lower():
            raise SystemExit('Authenticated GitHub user differs from owner; review manually')
        # Verify no conflicting repositories; non-404 API errors must stop the operation.
        for name in destinations:
            result = subprocess.run(['gh', 'api', f'repos/{args.owner}/{name}', '--silent'],
                                    capture_output=True, text=True)
            if result.returncode == 0:
                raise SystemExit(f'Remote already exists; inspect and integrate manually: {name}')
            if 'HTTP 404' not in result.stderr:
                raise SystemExit(f'Cannot safely verify remote availability: {result.stderr}')
    for source, dest in plans:
        dest.parent.mkdir(parents=True, exist_ok=True)
        if not dest.exists():
            shutil.copyfile(source, dest)
    for name, target in destinations.items():
        if not (target / '.git').exists():
            run(['git', 'init', '-b', 'main'], target)
        run(['python3', 'scripts/check-docs.py'], target)
        if args.commit:
            identity = run(['git', 'var', 'GIT_AUTHOR_IDENT'], target, capture=True).strip()
            print(f'Existing public commit identity: {identity}')
            run(['git', 'add', '--', *sorted(allowed_by_repo[name])], target)
            if subprocess.run(['git', 'diff', '--cached', '--quiet'], cwd=target).returncode:
                run(['git', 'commit', '-m', 'chore: bootstrap AGPL project and agent documentation'], target)
        if args.publish:
            run(['gh', 'repo', 'create', f'{args.owner}/{name}', '--public',
                 '--source', str(target), '--remote', 'origin', '--push'], target)
    print('Bootstrap steps finished. Update STATUS and ARBEITSPLAN with actual Git/remote/check results.')


if __name__ == '__main__':
    main()
