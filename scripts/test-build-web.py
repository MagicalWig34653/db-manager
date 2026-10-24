#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Exercise frontend artifact and offline-integrity boundaries."""
import hashlib
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

    def load_with(self, mutate):
        with tempfile.TemporaryDirectory() as temporary:
            lockfile = Path(temporary) / "lock.json"
            lock = json.loads(build_web.LOCK.read_text())
            mutate(lock)
            lockfile.write_text(json.dumps(lock))
            with patch.object(build_web, "LOCK", lockfile):
                return build_web.load_lock()

    def test_lock_pins_the_five_frozen_contracts_in_order(self):
        lock = build_web.load_lock()
        self.assertEqual(lock["schema_version"], 2)
        self.assertEqual([(c["family"], c["version"]) for c in lock["contracts"]], list(build_web.LEGACY_CONTRACTS))

    def test_missing_or_empty_contract_list_is_rejected(self):
        for mutate in [lambda l: l.pop("contracts"), lambda l: l.update(contracts=[]), lambda l: l.update(contracts={})]:
            with self.subTest(), self.assertRaisesRegex(ValueError, "contracts list"):
                self.load_with(mutate)

    def test_each_frozen_contract_is_required_and_must_match(self):
        for index, entry in enumerate(build_web.load_lock()["contracts"]):
            with self.subTest(family=entry["family"], case="missing"), self.assertRaisesRegex(ValueError, "Missing frozen " + entry["family"]):
                self.load_with(lambda l: l["contracts"].pop(index))
            with self.subTest(family=entry["family"], case="wrong sha"), self.assertRaisesRegex(ValueError, entry["family"] + " contract does not match"):
                self.load_with(lambda l: l["contracts"][index].update(sha256="0" * 64))

    def test_duplicate_versions_and_families_are_rejected(self):
        with self.assertRaisesRegex(ValueError, "Duplicate contract version"):
            self.load_with(lambda l: l["contracts"].append({**l["contracts"][-1], "family": "other"}))
        with self.assertRaisesRegex(ValueError, "Duplicate table contract family"):
            self.load_with(lambda l: l["contracts"].append({**l["contracts"][1], "version": "0.9.0"}))

    def test_contract_order_is_enforced(self):
        def swap(lock):
            lock["contracts"][1], lock["contracts"][2] = lock["contracts"][2], lock["contracts"][1]
        with self.assertRaisesRegex(ValueError, "ascending"):
            self.load_with(swap)

    def test_malformed_entries_are_rejected(self):
        cases = [lambda l: l["contracts"][0].update(extra=1),
                 lambda l: l["contracts"][0].pop("family"),
                 lambda l: l["contracts"][0].update(version="1.0.0"),
                 lambda l: l["contracts"][0].update(sha256="ABC"),
                 lambda l: l["contracts"][0].update(family="Bad Name"),
                 lambda l: l["contracts"].append("api-v0.6.0")]
        for index, mutate in enumerate(cases):
            with self.subTest(case=index), self.assertRaises(ValueError):
                self.load_with(mutate)

    def test_legacy_flat_keys_and_old_schema_are_rejected(self):
        with self.assertRaisesRegex(ValueError, "lock version"):
            self.load_with(lambda l: l.update(schema_version=1))
        with self.assertRaisesRegex(ValueError, "lock version"):
            self.load_with(lambda l: l.update(table_contract_sha256="0" * 64))

    def test_unpinned_extra_contract_must_exist_and_match_root_bytes(self):
        extra = {"family": "create", "version": "0.6.0", "sha256": "0" * 64}
        with self.assertRaisesRegex(ValueError, "create contract does not match"):
            self.load_with(lambda l: l["contracts"].append(extra))
        real = hashlib.sha256((build_web.ROOT / "contracts/api-v0.6.0.json").read_bytes()).hexdigest()
        lock = self.load_with(lambda l: l["contracts"].append({**extra, "sha256": real}))
        self.assertEqual(len(lock["contracts"]), 6)
        source = build_web.manifest_source(lock)
        self.assertEqual(source["schema_version"], 2)
        self.assertEqual(source["additional_contracts"], [{**extra, "sha256": real}])

    def test_web_contract_paths_follow_the_vendored_layout(self):
        self.assertEqual(build_web.web_contract_path("0.1.0"), "src/lib/contract/api-v0.1.0.json")
        self.assertEqual(build_web.web_contract_path("0.5.0"), "src/lib/contract/v5/api-v0.5.0.json")
        self.assertEqual(build_web.web_contract_path("0.12.0"), "src/lib/contract/v12/api-v0.12.0.json")

    def test_manifest_source_of_the_current_pin_matches_the_committed_manifest(self):
        committed = json.loads(build_web.MANIFEST.read_text())["source"]
        self.assertEqual(build_web.manifest_source(build_web.load_lock()), committed)


if __name__ == "__main__":
    unittest.main()
