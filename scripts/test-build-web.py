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

    def test_pwa_files_are_accepted_only_as_a_complete_exact_set(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            self.make_assets(root)
            for name in build_web.PWA_FILES:
                (root / name).write_bytes(b"synthetic")
            names = {f["path"] for f in build_web.inventory(root)}
            self.assertTrue(build_web.PWA_FILES <= names)
            (root / "icon-192.png").unlink()
            with self.assertRaisesRegex(ValueError, "Incomplete PWA file set: icon-192.png"):
                build_web.inventory(root)
        for name in ["sw.js.map", "icon-unlisted.png", "assets/sw.js", "manifest.json", "robots.txt"]:
            with self.subTest(name=name), self.assertRaises(ValueError):
                build_web.artifact(name, b"synthetic")
        with self.assertRaises(ValueError):
            build_web.artifact("sw.js", b"//# sourceMappingURL=sw.js.map")

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
        self.assertEqual([(c["family"], c["version"]) for c in lock["contracts"]][:5], list(build_web.LEGACY_CONTRACTS))
        self.assertEqual([(c["family"], c["version"]) for c in lock["contracts"]][5:], [("create", "0.6.0"), ("overview", "0.7.0"), ("object-ops", "0.8.0"), ("columns", "0.9.0"), ("export", "0.11.0")])

    def test_missing_or_empty_contract_list_is_rejected(self):
        for mutate in [lambda l: l.pop("contracts"), lambda l: l.update(contracts=[]), lambda l: l.update(contracts={})]:
            with self.subTest(), self.assertRaisesRegex(ValueError, "contracts list"):
                self.load_with(mutate)

    def test_each_frozen_contract_is_required_and_must_match(self):
        for index, entry in enumerate(build_web.load_lock()["contracts"][:5]):
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

    def test_additional_contracts_must_match_root_bytes_and_reach_the_provenance(self):
        wrong = lambda l: l["contracts"][-1].update(sha256="0" * 64)
        with self.assertRaisesRegex(ValueError, "export contract does not match"):
            self.load_with(wrong)
        lock = build_web.load_lock()
        source = build_web.manifest_source(lock)
        self.assertEqual(source["schema_version"], 2)
        self.assertEqual([c["version"] for c in source["additional_contracts"]], ["0.6.0", "0.7.0", "0.8.0", "0.9.0", "0.11.0"])
        self.assertEqual(len(lock["contracts"]), 10)

    def test_web_contract_paths_follow_the_vendored_layout(self):
        self.assertEqual(build_web.web_contract_path("0.1.0"), "src/lib/contract/api-v0.1.0.json")
        self.assertEqual(build_web.web_contract_path("0.5.0"), "src/lib/contract/v5/api-v0.5.0.json")
        self.assertEqual(build_web.web_contract_path("0.12.0"), "src/lib/contract/v12/api-v0.12.0.json")

    def test_manifest_source_of_the_current_pin_matches_the_committed_manifest(self):
        committed = json.loads(build_web.MANIFEST.read_text())["source"]
        self.assertEqual(build_web.manifest_source(build_web.load_lock()), committed)

    def test_dockerfile_source_label_matches_the_pinned_commit(self):
        label = 'org.dbmanager.frontend.source="https://github.com/MagicalWig34653/db-manager-web/tree/' + build_web.load_lock()["commit"] + '"'
        self.assertIn(label, (build_web.ROOT / "Dockerfile").read_text())


    COMMIT = "a" * 40

    def make_build(self, root, shell=None, extra=None):
        link = "https://github.com/MagicalWig34653/db-manager-web/tree/" + self.COMMIT
        files = {
            "index.html": '<html><head><script type="module" crossorigin src="/assets/index-abcdefgh.js"></script>'
                          '<link rel="stylesheet" crossorigin href="/assets/index-abcdefgh.css"></head></html>',
            "LICENSE.txt": "GNU AFFERO GENERAL PUBLIC LICENSE",
            "THIRD-PARTY-NOTICES.txt": "shadcn-svelte",
            "assets/index-abcdefgh.js": 'import("./feature-abcdefgh.js");const l="' + link + '";',
            "assets/index-abcdefgh.css": "body{}",
            "assets/feature-abcdefgh.js": "export default 1;",
        }
        files.update(extra or {})
        names = sorted(["index.html", *(build_web.PWA_FILES - {"sw.js"}), *[n for n in files if n.startswith("assets/")]])
        files["sw.js"] = "SHELL = JSON.parse('" + json.dumps(shell if shell is not None else names) + "')"
        for pwa in build_web.PWA_FILES - {"sw.js"}:
            files[pwa] = "x"
        for name, content in files.items():
            target = root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(content)
        return build_web.inventory(root)

    def verify_with(self, **kwargs):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            files = self.make_build(root, **kwargs)
            build_web.verify_build(root, files, self.COMMIT)

    def test_consistent_build_structure_is_accepted(self):
        self.verify_with()

    def test_orphan_chunks_are_rejected(self):
        with self.assertRaisesRegex(ValueError, "Unreferenced"):
            self.verify_with(extra={"assets/orphan-abcdefgh.js": "export default 2;"})

    def test_chunk_limit_is_enforced(self):
        chunks = {f"assets/c{i:02d}-abcdefgh.js": "x" for i in range(build_web.MAX_CHUNKS)}
        entry = 'import("./feature-abcdefgh.js");const l="https://github.com/MagicalWig34653/db-manager-web/tree/' + self.COMMIT + '";' + "".join(f'import("./c{i:02d}-abcdefgh.js");' for i in range(build_web.MAX_CHUNKS))
        with self.assertRaisesRegex(ValueError, "Too many"):
            self.verify_with(extra={**chunks, "assets/index-abcdefgh.js": entry})

    def test_source_link_must_be_in_the_entry_script_for_the_pinned_commit(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            files = self.make_build(root)
            with self.assertRaisesRegex(ValueError, "source link"):
                build_web.verify_build(root, files, "b" * 40)
        with self.assertRaisesRegex(ValueError, "source link"):
            self.verify_with(extra={"assets/index-abcdefgh.js": 'import("./feature-abcdefgh.js");'})

    def test_entry_files_must_be_unique_and_present(self):
        extra_script = '<script type="module" src="/assets/index-abcdefgh.js"></script><script type="module" src="/assets/feature-abcdefgh.js"></script>'
        with self.assertRaisesRegex(ValueError, "exactly one"):
            self.verify_with(extra={"index.html": extra_script + '<link rel="stylesheet" href="/assets/index-abcdefgh.css">'})
        with self.assertRaisesRegex(ValueError, "missing"):
            self.verify_with(extra={"index.html": '<script type="module" src="/assets/gone-abcdefgh.js"></script><link rel="stylesheet" href="/assets/index-abcdefgh.css">'})

    def test_service_worker_shell_list_must_equal_the_inventory(self):
        full = sorted(["index.html", *(build_web.PWA_FILES - {"sw.js"}), "assets/index-abcdefgh.js", "assets/index-abcdefgh.css", "assets/feature-abcdefgh.js"])
        for shell in (full[:-1], full + ["assets/extra-abcdefgh.js"], full + [full[0]]):
            with self.subTest(shell=len(shell)), self.assertRaisesRegex(ValueError, "shell list"):
                self.verify_with(shell=shell)

    def test_embedded_build_passes_the_structural_checks(self):
        build_web.verify_build(build_web.DEST, build_web.inventory(build_web.DEST), build_web.load_lock()["commit"])


if __name__ == "__main__":
    unittest.main()
