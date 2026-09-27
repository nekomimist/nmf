import hashlib
import json
import os
from pathlib import Path
import struct
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import release


class ReleaseTest(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.root = Path(directory.name)
        (self.root / "VERSION").write_text("0.1.0\n")
        self.changelog = "# Changelog\n\n## Unreleased\n\n## 0.1.0 - 2026-09-16\n\n- Initial release.\n"
        (self.root / "CHANGELOG.md").write_text(self.changelog)

    def test_semver_and_tag_validation(self):
        for version in ("0.1.0", "1.0.0-rc.1", "0.2.0-0", "1.2.3-1a.0+build.001", "1.2.3+build-1"):
            with self.subTest(version=version):
                (self.root / "VERSION").write_text(version + "\n")
                (self.root / "CHANGELOG.md").write_text(self.changelog.replace("0.1.0", version))
                self.assertEqual(release.release_notes(self.root, "v" + version), "- Initial release.\n")
        for version in ("v0.1.0", "0.1", "01.2.3", "0.1.0-01", "0.1.0-rc..1", "0.1.0+", "0.1.0;echo bad"):
            with self.subTest(version=version):
                (self.root / "VERSION").write_text(version)
                with self.assertRaises(ValueError):
                    release.read_version(self.root)
        (self.root / "VERSION").write_text("0.1.0\n")
        for tag in ("0.1.0", "v0.2.0", "v0.1.0-rc.1", "v0.1.0;echo bad"):
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                release.release_notes(self.root, tag)

    def test_changelog_must_be_ready_for_this_release(self):
        invalid = [
            self.changelog.replace("Unreleased", "Pending"),
            self.changelog.replace("0.1.0", "0.0.9"),
            self.changelog.replace("2026-09-16", "2026-02-30"),
            self.changelog.replace("## Unreleased\n", "## Unreleased\n\n- Forgotten change.\n"),
            self.changelog.replace("- Initial release.", ""),
            self.changelog + "\n## 0.1.0 - 2026-09-15\n\n- Duplicate.\n",
        ]
        for changelog in invalid:
            with self.subTest(changelog=changelog):
                (self.root / "CHANGELOG.md").write_text(changelog)
                with self.assertRaises(ValueError):
                    release.release_notes(self.root, "v0.1.0")
        (self.root / "CHANGELOG.md").write_text(self.changelog + "\n## 0.0.9 - 2026-09-15\n\n- Older.\n")
        self.assertEqual(release.release_notes(self.root, "v0.1.0"), "- Initial release.\n")

    def test_license_collection_includes_nested_notices_and_rejects_missing_licenses(self):
        module = self.root / "module"
        (module / "font").mkdir(parents=True)
        (module / "LICENSE").write_text("module license")
        (module / "font/LICENSE_Font.txt").write_text("font license")
        (module / "font/NOTICE").write_text("font notice")
        package = {"Module": {"Path": "example.com/library", "Version": "v1.2.3", "Dir": str(module)}}
        stream = json.dumps({"Standard": True}) + "\n" + json.dumps(package) + "\n" + json.dumps(package)
        with patch.object(release, "go_output", side_effect=[stream, str(self.root / "go")]), patch.object(
            release.subprocess, "check_output", return_value=f'.{{\n    .lib_dir = "{self.root}/zig",\n}}\n'
        ):
            files = release.dependency_licenses(self.root, "arm64")
        self.assertEqual(files["licenses/example.com/library@v1.2.3/LICENSE"], module / "LICENSE")
        self.assertIn("licenses/example.com/library@v1.2.3/font/LICENSE_Font.txt", files)
        self.assertIn("licenses/example.com/library@v1.2.3/font/NOTICE", files)
        self.assertIn("licenses/go/LICENSE", files)
        self.assertIn("licenses/mingw-w64/COPYING", files)

        empty = self.root / "empty-module"
        empty.mkdir()
        package["Module"]["Dir"] = str(empty)
        with patch.object(release, "go_output", return_value=json.dumps(package)):
            with self.assertRaisesRegex(ValueError, "no license documents"):
                release.dependency_licenses(self.root, "amd64")

    def prepare_archive(self, arch):
        dist = self.root / "dist"
        dist.mkdir(exist_ok=True)
        binary = bytearray(128)
        binary[:2] = b"MZ"
        struct.pack_into("<I", binary, 0x3C, 64)
        # Keep the PE fixture independent of the production architecture map.
        executable, machine = {
            "amd64": ("nmf.exe", 0x8664),
            "arm64": ("nmf-arm64.exe", 0xAA64),
        }[arch]
        struct.pack_into("<4sH", binary, 64, b"PE\0\0", machine)
        (dist / executable).write_bytes(binary)
        for name in ("LICENSE", "THIRD_PARTY_LICENSES.txt", "README.md"):
            (self.root / name).write_text(name + " contents")
        # Nix store license documents have timestamps before ZIP's minimum year.
        os.utime(self.root / "LICENSE", (1, 1))
        return {"licenses/dependency/LICENSE": self.root / "LICENSE"}

    def test_zip_contents_architectures_checksums_and_rebuild(self):
        for arch in ("amd64", "arm64"):
            with self.subTest(arch=arch):
                licenses = self.prepare_archive(arch)
                archive = release.write_archive(self.root, arch, licenses)
                self.assertEqual(archive.name, f"nmf-v0.1.0-windows-{arch}.zip")
                with zipfile.ZipFile(archive) as zipped:
                    self.assertEqual(set(zipped.namelist()), {
                        "nmf.exe", "LICENSE", "THIRD_PARTY_LICENSES.txt", "README.md", "CHANGELOG.md",
                        "licenses/dependency/LICENSE",
                    })
                    self.assertIsNone(zipped.testzip())
                checksum = hashlib.sha256(archive.read_bytes()).hexdigest()
                self.assertEqual(archive.with_suffix(".zip.sha256").read_text(), f"{checksum}  {archive.name}\n")
                with zipfile.ZipFile(archive, "a") as zipped:
                    zipped.writestr("stale.txt", "from an earlier run")
                release.write_archive(self.root, arch, licenses)
                with zipfile.ZipFile(archive) as zipped:
                    self.assertNotIn("stale.txt", zipped.namelist())

    def test_packaging_rejects_wrong_architecture_and_missing_notices(self):
        licenses = self.prepare_archive("arm64")
        (self.root / "dist/nmf.exe").write_bytes((self.root / "dist/nmf-arm64.exe").read_bytes())
        with self.assertRaisesRegex(ValueError, "not a Windows amd64 executable"):
            release.write_archive(self.root, "amd64", licenses)
        for name in ("LICENSE", "THIRD_PARTY_LICENSES.txt"):
            with self.subTest(name=name):
                licenses = self.prepare_archive("arm64")
                (self.root / name).write_text("")
                with self.assertRaisesRegex(ValueError, "missing or empty release file"):
                    release.write_archive(self.root, "arm64", licenses)

    def prepare_verified_assets(self):
        for arch in ("amd64", "arm64"):
            licenses = self.prepare_archive(arch)
            licenses["licenses/example.com/dependency@v1.0.0/LICENSE"] = licenses.pop("licenses/dependency/LICENSE")
            for name in ("licenses/go/LICENSE", "licenses/go/PATENTS", "licenses/mingw-w64/COPYING"):
                licenses[name] = self.root / "LICENSE"
            release.write_archive(self.root, arch, licenses)
        return self.root / "dist"

    def replace_archive_entry(self, path, name, content):
        with zipfile.ZipFile(path) as archive:
            entries = {entry: archive.read(entry) for entry in archive.namelist()}
        if content is None:
            del entries[name]
        else:
            entries[name] = content
        with zipfile.ZipFile(path, "w") as archive:
            for entry, data in entries.items():
                archive.writestr(entry, data)
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        path.with_suffix(".zip.sha256").write_text(f"{digest}  {path.name}\n")

    def test_verify_assets_uses_explicit_tag_and_detects_corruption(self):
        directory = self.prepare_verified_assets()
        (self.root / "VERSION").write_text("9.9.9\n")
        hashes = release.verify_assets(directory, "v0.1.0")
        for arch in ("amd64", "arm64"):
            path = directory / f"nmf-v0.1.0-windows-{arch}.zip"
            self.assertEqual(hashes[arch], hashlib.sha256(path.read_bytes()).hexdigest())
        path = directory / "nmf-v0.1.0-windows-arm64.zip"
        original = path.read_bytes()
        path.write_bytes(original + b"changed after publication")
        with self.assertRaisesRegex(ValueError, "SHA-256 mismatch"):
            release.verify_assets(directory, "v0.1.0")
        path.write_bytes(original)
        path.with_suffix(".zip.sha256").write_text(f"{hashes['arm64']}  different.zip\n")
        with self.assertRaisesRegex(ValueError, "invalid checksum file"):
            release.verify_assets(directory, "v0.1.0")
        for tag in ("0.1.0", "v../0.1.0", "v0.1.0/extra"):
            with self.subTest(tag=tag), self.assertRaisesRegex(ValueError, "invalid release tag"):
                release.verify_assets(directory, tag)

    def test_verify_assets_checks_contents_even_with_matching_checksums(self):
        for name, content, error in (
            ("LICENSE", None, "missing release files"),
            ("licenses/go/PATENTS", None, "missing release files"),
            ("licenses/example.com/dependency@v1.0.0/LICENSE", None, "missing module license"),
            ("THIRD_PARTY_LICENSES.txt", b"", "empty release file"),
            ("CHANGELOG.md", self.changelog.replace("0.1.0", "0.0.9").encode(), "newest CHANGELOG"),
            ("nmf.exe", b"not a PE executable", "not a Windows executable"),
        ):
            with self.subTest(name=name):
                directory = self.prepare_verified_assets()
                path = directory / "nmf-v0.1.0-windows-amd64.zip"
                self.replace_archive_entry(path, name, content)
                with self.assertRaisesRegex(ValueError, error):
                    release.verify_assets(directory, "v0.1.0")
        directory = self.prepare_verified_assets()
        path = directory / "nmf-v0.1.0-windows-amd64.zip"
        with zipfile.ZipFile(directory / "nmf-v0.1.0-windows-arm64.zip") as archive:
            self.replace_archive_entry(path, "nmf.exe", archive.read("nmf.exe"))
        with self.assertRaisesRegex(ValueError, "not a Windows amd64 executable"):
            release.verify_assets(directory, "v0.1.0")

    def test_scoop_candidate_preserves_input_and_rejects_invalid_updates(self):
        directory = self.prepare_verified_assets()
        manifest_path = self.root / "nmf.json"
        manifest = {
            "version": "0.0.9",
            "homepage": "https://github.com/nekomimist/nmf",
            "architecture": {"64bit": {"url": "old64", "hash": "old"}, "arm64": {"url": "oldarm", "hash": "old"}},
            "bin": "nmf.exe", "autoupdate": {"hash": {"url": "$url.sha256"}},
        }
        raw = (json.dumps(manifest, indent=4) + "\n").replace("\n", "\r\n").encode()
        manifest_path.write_bytes(raw)
        command = [sys.executable, str(Path(release.__file__).resolve()), "verify", "v0.1.0", str(directory),
                   "--scoop-manifest", str(manifest_path)]
        result = subprocess.run(command, capture_output=True, check=True)
        self.assertEqual(manifest_path.read_bytes(), raw)
        self.assertIn(b"\r\n", result.stdout)
        self.assertNotIn(b"\n", result.stdout.replace(b"\r\n", b""))
        candidate = json.loads(result.stdout)
        self.assertEqual(candidate["version"], "0.1.0")
        for key, arch in (("64bit", "amd64"), ("arm64", "arm64")):
            filename = f"nmf-v0.1.0-windows-{arch}.zip"
            self.assertEqual(candidate["architecture"][key], {
                "url": f"https://github.com/nekomimist/nmf/releases/download/v0.1.0/{filename}",
                "hash": hashlib.sha256((directory / filename).read_bytes()).hexdigest(),
            })
        for key in ("homepage", "bin", "autoupdate"):
            self.assertEqual(candidate[key], manifest[key])
        hashes = release.verify_assets(directory, "v0.1.0")
        manifest_path.write_bytes(result.stdout)
        self.assertEqual(release.scoop_manifest(manifest_path, "v0.1.0", hashes), result.stdout)
        for tag, message in (("v0.0.9", "downgrade"), ("v0.2.0-rc.1", "stable releases")):
            with self.subTest(tag=tag), self.assertRaisesRegex(ValueError, message):
                release.scoop_manifest(manifest_path, tag, hashes)
        (directory / "nmf-v0.1.0-windows-arm64.zip").unlink()
        failed = subprocess.run(command, capture_output=True)
        self.assertNotEqual(failed.returncode, 0)
        self.assertEqual(failed.stdout, b"")
        self.assertEqual(manifest_path.read_bytes(), result.stdout)


if __name__ == "__main__":
    unittest.main()
