#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Exercise frontend artifact and offline-integrity boundaries."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("build_web", Path(__file__).with_name("build-web.py"))
build_web = importlib.util.module_from_spec(spec)
spec.loader.exec_module(build_web)


class ArtifactTests(unittest.TestCase):
    def make_assets(self, root):
        for name, content in {
            "index.html": b"<!doctype html><html lang='de'></html>",
            "LICENSE.txt": b"GNU AFFERO GENERAL PUBLIC LICENSE",
            "THIRD-PARTY-NOTICES.txt": b"Original shadcn-svelte notices",
            "assets/index-abcdefgh.js": b"const synthetic = 1;",
        }.items():
            target = root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(content)

    def test_manifest_verifies_offline_without_subprocess(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            self.make_assets(root)
            manifest = root / "provenance.json"
            lock = build_web.load_lock()
            manifest.write_text(json.dumps(build_web.expected_manifest(lock, build_web.inventory(root))))
            # The manifest is intentionally outside the asset inventory.
            assets = root / "dist"
            assets.mkdir()
            for name in build_web.ROOT_FILES | {"assets/index-abcdefgh.js"}:
                source = root / name
                target = assets / name
                target.parent.mkdir(parents=True, exist_ok=True)
                source.rename(target)
            with patch.object(build_web, "DEST", assets), patch.object(build_web, "MANIFEST", manifest), patch.object(build_web, "run", side_effect=AssertionError("offline verification invoked a tool")):
                build_web.check(lock)
                (assets / "assets/index-abcdefgh.js").write_bytes(b"modified")
                with self.assertRaisesRegex(ValueError, "checksums"):
                    build_web.check(lock)

    def test_unknown_files_and_source_maps_are_rejected(self):
        for name in ["assets/index.map", "node_modules/a.js", ".env", "assets/unhashed.js", "credentials.json"]:
            with self.subTest(name=name), self.assertRaises(ValueError):
                build_web.artifact(name, b"synthetic")

    def test_private_paths_and_inline_maps_are_rejected(self):
        for content in [b"/Users/synthetic/path", b"//# sourceMappingURL=data:synthetic"]:
            with self.subTest(content=content), self.assertRaises(ValueError):
                build_web.artifact("assets/index-abcdefgh.js", content)

    def test_missing_notices_are_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            self.make_assets(root)
            (root / "THIRD-PARTY-NOTICES.txt").unlink()
            with self.assertRaisesRegex(ValueError, "license"):
                build_web.inventory(root)

    def test_symlinks_cannot_enter_the_build(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            self.make_assets(root)
            (root / "assets/linked-abcdefgh.js").symlink_to(root / "index.html")
            with self.assertRaisesRegex(ValueError, "Symlinks"):
                build_web.inventory(root)

    def test_non_commit_source_references_are_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            lockfile = Path(temporary) / "lock.json"
            lock = json.loads(build_web.LOCK.read_text())
            lock["commit"] = "main"
            lockfile.write_text(json.dumps(lock))
            with patch.object(build_web, "LOCK", lockfile), self.assertRaisesRegex(ValueError, "full Git commit"):
                build_web.load_lock()

    def test_missing_table_contract_pin_is_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            lockfile = Path(temporary) / "lock.json"
            lock = json.loads(build_web.LOCK.read_text())
            del lock["table_contract_version"]
            lockfile.write_text(json.dumps(lock))
            with patch.object(build_web, "LOCK", lockfile), self.assertRaisesRegex(ValueError, "table contract"):
                build_web.load_lock()

    def test_mismatched_table_contract_bytes_are_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            lockfile = Path(temporary) / "lock.json"
            lock = json.loads(build_web.LOCK.read_text())
            lock["table_contract_sha256"] = "0" * 64
            lockfile.write_text(json.dumps(lock))
            with patch.object(build_web, "LOCK", lockfile), self.assertRaisesRegex(ValueError, "table contract"):
                build_web.load_lock()

    def test_all_additive_contract_pins_are_required_and_match(self):
        for family in ["mutation", "insert", "structure"]:
            for field, value in [("_contract_version", None), ("_contract_sha256", "0" * 64)]:
                with self.subTest(family=family, field=field), tempfile.TemporaryDirectory() as temporary:
                    lockfile = Path(temporary) / "lock.json"
                    lock = json.loads(build_web.LOCK.read_text())
                    if value is None:
                        del lock[family + field]
                    else:
                        lock[family + field] = value
                    lockfile.write_text(json.dumps(lock))
                    with patch.object(build_web, "LOCK", lockfile), self.assertRaisesRegex(ValueError, family + " contract"):
                        build_web.load_lock()


if __name__ == "__main__":
    unittest.main()
