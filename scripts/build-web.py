#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Rebuild reviewed frontend assets or verify the committed asset manifest."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parent.parent
LOCK = ROOT / "web-build.lock.json"
DEST = ROOT / "internal/webui/static"
MANIFEST = ROOT / "internal/webui/assets-manifest.json"
EXPECTED_REPOSITORY = "https://github.com/MagicalWig34653/db-manager-web.git"
ROOT_FILES = {"index.html", "LICENSE.txt", "THIRD-PARTY-NOTICES.txt"}
ASSET_NAME = re.compile(r"assets/[A-Za-z0-9._-]+-[A-Za-z0-9_-]{8,}\.(js|css)\Z")


def digest(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def load_lock() -> dict:
    lock = json.loads(LOCK.read_text())
    if lock.get("schema_version") != 1 or lock.get("repository") != EXPECTED_REPOSITORY:
        raise ValueError("Unsupported source repository or lock version")
    if not re.fullmatch(r"[0-9a-f]{40}", lock.get("commit", "")):
        raise ValueError("The frontend source must be pinned to one full Git commit")
    if not re.fullmatch(r"[0-9a-f]{64}", lock.get("contract_sha256", "")) or not re.fullmatch(r"[0-9a-f]{64}", lock.get("npm_lock_sha256", "")):
        raise ValueError("Missing contract SHA-256")
    if lock.get("contract_version") != "0.1.0" or lock.get("build_mode") != "production":
        raise ValueError("Unsupported API contract")
    if digest((ROOT / "contracts/api-v0.1.0.json").read_bytes()) != lock["contract_sha256"]:
        raise ValueError("The main repository contract does not match the frontend lock")
    for tool in ("node", "npm"):
        if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", lock.get("toolchain", {}).get(tool, "")):
            raise ValueError("Toolchain versions must be pinned exactly")
    return lock


def artifact(path: str, content: bytes) -> dict:
    if path not in ROOT_FILES and not ASSET_NAME.fullmatch(path):
        raise ValueError(f"Unexpected frontend artifact: {path}")
    if b"/Users/" in content or b"sourceMappingURL=" in content:
        raise ValueError(f"Private path or source map marker in artifact: {path}")
    return {"path": path, "sha256": digest(content), "size": len(content)}


def inventory(folder: Path) -> list[dict]:
    files = []
    if not folder.is_dir() or folder.is_symlink():
        raise ValueError("Asset directory is missing or is a symlink")
    for file in sorted(folder.rglob("*")):
        if file.is_symlink():
            raise ValueError("Symlinks are not distributable assets")
        if file.is_file():
            files.append(artifact(file.relative_to(folder).as_posix(), file.read_bytes()))
    names = {file["path"] for file in files}
    if not ROOT_FILES.issubset(names) or not any(name.endswith(".js") for name in names):
        raise ValueError("Required HTML, JavaScript or license artifacts are missing")
    return files


def expected_manifest(lock: dict, files: list[dict]) -> dict:
    return {"schema_version": 1, "source": lock, "files": files}


def check(lock: dict) -> None:
    manifest = json.loads(MANIFEST.read_text())
    if manifest != expected_manifest(lock, inventory(DEST)):
        raise ValueError("Embedded frontend provenance or artifact checksums are stale")
    print(f"Verified {len(manifest['files'])} embedded assets and pinned provenance.")


def run(command: list[str], cwd: Path, env: dict[str, str], capture: bool = False) -> str:
    result = subprocess.run(command, cwd=cwd, env=env, check=True, text=True,
                            stdout=subprocess.PIPE if capture else None)
    return result.stdout.strip() if capture else ""


def rebuild(lock: dict) -> None:
    with tempfile.TemporaryDirectory(prefix="db-manager-web-build-") as temporary:
        scratch = Path(temporary)
        npm_user = scratch / "npm-user.conf"
        npm_global = scratch / "npm-global.conf"
        npm_user.write_text("")
        npm_global.write_text("")
        env = {key: value for key, value in os.environ.items()
               if key in {"PATH", "HOME", "TMPDIR", "SYSTEMROOT", "SystemRoot"}}
        env.update({"GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull,
                    "NPM_CONFIG_USERCONFIG": str(npm_user),
                    "NPM_CONFIG_GLOBALCONFIG": str(npm_global),
                    "NODE_ENV": "development"})
        for tool in ("node", "npm"):
            version = run([tool, "--version"], ROOT, env, capture=True).removeprefix("v")
            if version != lock["toolchain"][tool]:
                raise ValueError(f"Expected {tool} {lock['toolchain'][tool]}, found {version}")
        checkout = scratch / "source"
        checkout.mkdir()
        run(["git", "init", "--quiet", "--initial-branch=build"], checkout, env)
        run(["git", "remote", "add", "origin", lock["repository"]], checkout, env)
        run(["git", "fetch", "--quiet", "--depth=1", "origin", lock["commit"]], checkout, env)
        run(["git", "checkout", "--quiet", "--detach", "FETCH_HEAD"], checkout, env)
        if run(["git", "rev-parse", "HEAD"], checkout, env, capture=True) != lock["commit"]:
            raise ValueError("Fetched source commit differs from the lock")
        contract = checkout / "src/lib/contract/api-v0.1.0.json"
        if digest(contract.read_bytes()) != lock["contract_sha256"]:
            raise ValueError("Frontend API contract differs from the lock")
        if digest((checkout / "package-lock.json").read_bytes()) != lock["npm_lock_sha256"]:
            raise ValueError("Frontend npm lockfile differs from the pinned checksum")
        run(["npm", "ci", "--no-audit", "--no-fund"], checkout, env)
        run(["npm", "run", "build"], checkout, {**env, "NODE_ENV": "production"})
        output = checkout / "dist"
        files = inventory(output)
        if "GNU AFFERO GENERAL PUBLIC LICENSE" not in (output / "LICENSE.txt").read_text():
            raise ValueError("The frontend AGPL license is missing")
        if "shadcn-svelte" not in (output / "THIRD-PARTY-NOTICES.txt").read_text():
            raise ValueError("Original component license notices are missing")
        for built, original in [("LICENSE.txt", "LICENSE"),
                                ("THIRD-PARTY-NOTICES.txt", "src/third-party-notices.txt")]:
            if (output / built).read_bytes() != (checkout / original).read_bytes():
                raise ValueError("Original license notices were altered during the build")
        if DEST.is_symlink() or MANIFEST.is_symlink():
            raise ValueError("Generated destinations must not be symlinks")
        staged = scratch / "assets"
        shutil.copytree(output, staged)
        DEST.parent.mkdir(parents=True, exist_ok=True)
        if DEST.exists():
            shutil.rmtree(DEST)
        shutil.copytree(staged, DEST)
        MANIFEST.write_text(json.dumps(expected_manifest(lock, files), indent=2, sort_keys=True) + "\n")
    check(lock)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="Verify vendored files without Node or network")
    arguments = parser.parse_args()
    try:
        lock = load_lock()
        if arguments.check:
            check(lock)
        else:
            rebuild(lock)
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"Frontend build failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
