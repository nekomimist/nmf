#!/usr/bin/env python3
"""Prepare and verify NMF releases (standard library only)."""

import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import struct
import subprocess
import sys
import zipfile


ROOT = Path(__file__).resolve().parents[1]
NUMBER = r"(?:0|[1-9][0-9]*)"
IDENTIFIER = rf"(?:{NUMBER}|[0-9]*[A-Za-z-][0-9A-Za-z-]*)"
SEMVER = re.compile(
    rf"{NUMBER}\.{NUMBER}\.{NUMBER}"
    rf"(?:-{IDENTIFIER}(?:\.{IDENTIFIER})*)?"
    r"(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?"
)
LICENSE_NAME = re.compile(
    r"^(?:licen[cs]e|copying|notice|copyright|authors|patents)(?:$|[._-])", re.I
)
# This pinned module declares its license in its README, without a LICENSE file.
LICENSE_READMES = {"github.com/koron/gelatin": "README.mkd"}
ARCHITECTURES = {
    "amd64": ("nmf.exe", 0x8664),
    "arm64": ("nmf-arm64.exe", 0xAA64),
}
RELEASE_DOCUMENTS = ("LICENSE", "THIRD_PARTY_LICENSES.txt", "README.md", "CHANGELOG.md")


def read_version(root):
    version = (root / "VERSION").read_text(encoding="utf-8").strip()
    if not SEMVER.fullmatch(version):
        raise ValueError(f"VERSION is not a Semantic Version: {version!r}")
    return version


def release_notes(root, tag):
    version = read_version(root)
    if tag != f"v{version}":
        raise ValueError(f"tag {tag!r} does not match VERSION (v{version})")
    changelog = (root / "CHANGELOG.md").read_text(encoding="utf-8")
    return notes_for_version(version, changelog)


def notes_for_version(version, changelog):
    headings = list(re.finditer(r"^## (.+)$", changelog, re.M))
    if len(headings) < 2 or headings[0][1] != "Unreleased":
        raise ValueError("CHANGELOG.md must start with an Unreleased section")
    match = re.fullmatch(rf"{re.escape(version)} - (\d{{4}}-\d{{2}}-\d{{2}})", headings[1][1])
    if not match:
        raise ValueError(f"the newest CHANGELOG.md release must be {version} - YYYY-MM-DD")
    datetime.date.fromisoformat(match[1])
    if changelog[headings[0].end():headings[1].start()].strip():
        raise ValueError("move Unreleased entries into the release section before tagging")
    if any(h[1].startswith(f"{version} - ") for h in headings[2:]):
        raise ValueError(f"duplicate CHANGELOG.md release: {version}")
    end = headings[2].start() if len(headings) > 2 else len(changelog)
    notes = changelog[headings[1].end():end].strip()
    if not notes:
        raise ValueError(f"CHANGELOG.md release {version} has no notes")
    return notes + "\n"


def go_output(root, *args, env=None):
    return subprocess.check_output(["go", *args], cwd=root, env=env, text=True)


def dependency_licenses(root, arch):
    env = dict(os.environ, GOOS="windows", GOARCH=arch, CGO_ENABLED="1")
    stream = go_output(root, "list", "-deps", "-json", "-tags", "release,migrated_fynedo", ".", env=env)
    decoder = json.JSONDecoder()
    modules = {}
    while stream.strip():
        package, end = decoder.raw_decode(stream.lstrip())
        stream = stream.lstrip()[end:]
        module = package.get("Module")
        if module and not module.get("Main"):
            if module.get("Replace"):
                raise ValueError(f"release dependencies must not use replacements: {module['Path']}")
            modules[module["Path"]] = module

    files = {}
    for name, module in sorted(modules.items()):
        directory = Path(module["Dir"])
        notices = sorted(p for p in directory.rglob("*") if p.is_file() and LICENSE_NAME.match(p.name))
        if not notices and name in LICENSE_READMES:
            notices = [directory / LICENSE_READMES[name]]
        if not notices:
            raise ValueError(f"no license documents found for {name}; review its upstream notices")
        for notice in notices:
            archive_path = f"licenses/{name}@{module['Version']}/{notice.relative_to(directory).as_posix()}"
            files[archive_path] = notice

    go_license_dir = Path(os.environ.get("NMF_GO_LICENSE_DIR") or go_output(root, "env", "GOROOT").strip())
    files["licenses/go/LICENSE"] = go_license_dir / "LICENSE"
    files["licenses/go/PATENTS"] = go_license_dir / "PATENTS"
    # Zig's Windows C runtime is linked into the executable by the Makefile.
    # The pinned Zig CLI prints ZON, with a quoted lib_dir field.
    zig_env = subprocess.check_output(["zig", "env"], cwd=root, text=True)
    lib_dir = re.search(r'^\s*\.lib_dir = ("[^"\n]+"),$', zig_env, re.M)
    if not lib_dir:
        raise ValueError("could not locate the pinned Zig toolchain's license documents")
    files["licenses/mingw-w64/COPYING"] = Path(json.loads(lib_dir[1])) / "libc/mingw/COPYING"
    return files


def check_executable(binary, arch):
    with binary.open("rb") as stream:
        check_pe(stream, arch, binary)


def check_pe(stream, arch, label):
    if stream.read(2) != b"MZ":
        raise ValueError(f"{label} is not a Windows executable")
    stream.seek(0x3C)
    stream.seek(struct.unpack("<I", stream.read(4))[0])
    signature, machine = struct.unpack("<4sH", stream.read(6))
    if signature != b"PE\0\0" or machine != ARCHITECTURES[arch][1]:
        raise ValueError(f"{label} is not a Windows {arch} executable")


def write_archive(root, arch, licenses):
    version = read_version(root)
    binary = root / "dist" / ARCHITECTURES[arch][0]
    check_executable(binary, arch)
    files = {"nmf.exe": binary, **licenses}
    for name in RELEASE_DOCUMENTS:
        files[name] = root / name
    for source in files.values():
        if not source.is_file() or source.stat().st_size == 0:
            raise ValueError(f"missing or empty release file: {source}")

    destination = root / "dist" / f"nmf-v{version}-windows-{arch}.zip"
    # Opening with 'w' prevents files from an earlier run surviving in the ZIP.
    # Nix store files use epoch timestamps, earlier than ZIP's 1980 minimum.
    with zipfile.ZipFile(destination, "w", compression=zipfile.ZIP_DEFLATED, strict_timestamps=False) as archive:
        for name, source in sorted(files.items()):
            archive.write(source, name)
    with destination.open("rb") as stream:
        digest = hashlib.file_digest(stream, "sha256").hexdigest()
    destination.with_suffix(".zip.sha256").write_text(f"{digest}  {destination.name}\n", encoding="utf-8")
    return destination


def version_from_tag(tag):
    if not tag.startswith("v") or not SEMVER.fullmatch(tag[1:]):
        raise ValueError(f"invalid release tag: {tag!r}")
    return tag[1:]


def verify_assets(directory, tag):
    """Check both downloaded ZIPs without extracting or executing their contents."""
    version = version_from_tag(tag)
    hashes = {}
    for arch in ARCHITECTURES:
        path = directory / f"nmf-{tag}-windows-{arch}.zip"
        fields = path.with_suffix(".zip.sha256").read_text(encoding="utf-8").split()
        if len(fields) != 2 or not re.fullmatch(r"[0-9a-fA-F]{64}", fields[0]) or fields[1] != path.name:
            raise ValueError(f"invalid checksum file for {path.name}")
        with path.open("rb") as stream:
            digest = hashlib.file_digest(stream, "sha256").hexdigest()
        if digest != fields[0].lower():
            raise ValueError(f"SHA-256 mismatch: {path.name}")
        with zipfile.ZipFile(path) as archive:
            damaged = archive.testzip()
            if damaged:
                raise ValueError(f"damaged ZIP entry: {path.name}/{damaged}")
            names = set(archive.namelist())
            required = {"nmf.exe", *RELEASE_DOCUMENTS,
                        "licenses/go/LICENSE", "licenses/go/PATENTS", "licenses/mingw-w64/COPYING"}
            missing = required - names
            if missing:
                raise ValueError(f"missing release files in {path.name}: {sorted(missing)}")
            licenses = [name for name in names if name.startswith("licenses/") and not name.endswith("/")]
            if not any("@" in name for name in licenses):
                raise ValueError(f"missing module license documents in {path.name}")
            for name in required | set(licenses):
                if archive.getinfo(name).file_size == 0:
                    raise ValueError(f"empty release file: {path.name}/{name}")
            with archive.open("nmf.exe") as stream:
                check_pe(stream, arch, path.name)
            notes_for_version(version, archive.read("CHANGELOG.md").decode("utf-8"))
        hashes[arch] = digest
    return hashes


def scoop_manifest(manifest_path, tag, hashes):
    """Render a candidate manifest; never write to the bucket."""
    version = version_from_tag(tag)
    if "-" in version.split("+", 1)[0]:
        raise ValueError("Scoop only tracks stable releases")
    raw = manifest_path.read_bytes()
    manifest = json.loads(raw)
    if manifest.get("homepage") != "https://github.com/nekomimist/nmf":
        raise ValueError("expected the nmf Scoop manifest")
    current = manifest.get("version", "")
    if not SEMVER.fullmatch(current) or "-" in current.split("+", 1)[0]:
        raise ValueError(f"invalid stable Scoop version: {current!r}")
    current_core = tuple(map(int, current.split("+", 1)[0].split(".")))
    target_core = tuple(map(int, version.split("+", 1)[0].split(".")))
    if current_core > target_core:
        raise ValueError(f"refusing to downgrade Scoop from {current} to {version}")
    for key, arch in (("64bit", "amd64"), ("arm64", "arm64")):
        entry = manifest.get("architecture", {}).get(key)
        if not isinstance(entry, dict):
            raise ValueError(f"missing Scoop architecture: {key}")
        entry["url"] = f"https://github.com/nekomimist/nmf/releases/download/{tag}/nmf-{tag}-windows-{arch}.zip"
        entry["hash"] = hashes[arch]
    manifest["version"] = version
    text = json.dumps(manifest, indent=4, ensure_ascii=False) + "\n"
    if b"\r\n" in raw:
        text = text.replace("\n", "\r\n")
    return text.encode("utf-8")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    check = commands.add_parser("check", help="validate the tag and print its CHANGELOG release notes")
    check.add_argument("tag")
    package = commands.add_parser("package", help="ZIP an existing Windows build with license documents")
    package.add_argument("arch", choices=ARCHITECTURES)
    verify = commands.add_parser("verify", help="verify both ZIPs and checksums for an explicit tag")
    verify.add_argument("tag")
    verify.add_argument("directory", type=Path)
    verify.add_argument("--scoop-manifest", type=Path,
                        help="print an updated manifest instead of hashes; does not modify the input file")
    args = parser.parse_args()
    try:
        if args.command == "check":
            print(release_notes(ROOT, args.tag), end="")
        elif args.command == "package":
            print(write_archive(ROOT, args.arch, dependency_licenses(ROOT, args.arch)))
        else:
            hashes = verify_assets(args.directory, args.tag)
            if args.scoop_manifest:
                sys.stdout.buffer.write(scoop_manifest(args.scoop_manifest, args.tag, hashes))
            else:
                print(json.dumps(hashes, indent=2))
    except (OSError, ValueError, struct.error, zipfile.BadZipFile, subprocess.CalledProcessError) as error:
        parser.exit(1, f"release: {error}\n")


if __name__ == "__main__":
    main()
