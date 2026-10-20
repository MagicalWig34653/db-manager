#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Create a container-readable token inside a private directory without displaying it."""
import os
from pathlib import Path
import secrets

path = Path('secrets/api_token')
path.parent.mkdir(mode=0o700, exist_ok=True)
if path.parent.is_symlink() or path.parent.stat().st_mode & 0o077:
    raise SystemExit('Token directory must be private (mode 0700), without symlinks.')
try:
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o444)
except FileExistsError:
    raise SystemExit('Token exists; no change.')
os.fchmod(descriptor, 0o444)
with os.fdopen(descriptor, 'w') as output:
    output.write(secrets.token_urlsafe(48) + '\n')
print('Development token created in secrets/api_token (not displayed).')
