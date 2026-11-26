#!/usr/bin/env python3
"""Smoke-test a macOS packaging artifact using a Go toolchain for SQLite verification."""

import argparse
from contextlib import contextmanager
import hashlib
import json
import os
import pathlib
import plistlib
import re
import subprocess
import tempfile
import time
import zipfile


SOURCE_ROOT = pathlib.Path(__file__).resolve().parents[1]


WINDOW_HELPER = """
import AppKit
import CoreGraphics

guard CommandLine.arguments.count == 3,
      let pid = Int32(CommandLine.arguments[2]),
      let app = NSRunningApplication(processIdentifier: pid),
      !app.isTerminated else { exit(1) }

switch CommandLine.arguments[1] {
case "window":
    let windows = CGWindowListCopyWindowInfo(
        [.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID
    ) as? [[String: Any]] ?? []
    let visible = windows.contains {
        ($0[kCGWindowOwnerPID as String] as? NSNumber)?.int32Value == pid &&
        ($0[kCGWindowLayer as String] as? NSNumber)?.intValue == 0
    }
    exit(visible ? 0 : 1)
case "quit":
    exit(app.terminate() ? 0 : 1)
default:
    exit(2)
}
"""


def validate_archive(path):
    with zipfile.ZipFile(path) as archive:
        for member in archive.infolist():
            name = pathlib.PurePosixPath(member.filename)
            if name.is_absolute() or ".." in name.parts or not name.parts:
                raise ValueError(f"Invalid archive path: {member.filename}")
            if name.parts[0] not in ("JindiDrama.app", "__MACOSX"):
                raise ValueError(f"Unexpected archive entry: {member.filename}")
            if member.external_attr >> 28 == 0o12:
                raise ValueError(f"Unexpected symbolic link in app archive: {member.filename}")


def verify_webview_profile(bundle_id):
    if os.environ.get("GITHUB_ACTIONS") != "true" and not bundle_id.startswith("local.jindidrama.archiveqa."):
        raise ValueError("Local smoke archives require a unique local.jindidrama.archiveqa.* bundle identifier")

    # Wails v2 uses WKWebView's persistent profile. Never reuse one from
    # an existing installation or an earlier attempt under the same account.
    library = pathlib.Path.home() / "Library"
    for segment in ("WebKit", "Caches", "HTTPStorages", "Application Support", "Containers"):
        parent = library / segment
        if parent.is_dir() and any(child.name.casefold() == bundle_id.casefold() for child in parent.iterdir()):
            raise ValueError(f"Existing WebView profile for {bundle_id} under {segment}")


@contextmanager
def exclusive_webview_profile(bundle_id):
    import fcntl

    identity = hashlib.sha256(bundle_id.casefold().encode("ascii")).hexdigest()
    lock = pathlib.Path(tempfile.gettempdir()) / f"jindi-webview-{identity}.lock"
    with os.fdopen(os.open(lock, os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600), "r+b") as file:
        fcntl.flock(file, fcntl.LOCK_EX | fcntl.LOCK_NB)
        verify_webview_profile(bundle_id)
        yield


def ready(profile, expected_version, expected_arch):
    if len(expected_version) > 128:
        raise ValueError("Expected version exceeds the application marker limit")
    for marker in (profile / "logs" / "sessions").glob("run-*.session"):
        try:
            with marker.open("rb") as stream:
                data = stream.read(513)
        except FileNotFoundError:
            continue
        if data[:1] == b"R":
            if len(data) > 512:
                raise RuntimeError("Ready marker exceeds the application size limit")
            metadata = json.loads(data[1:])
            if not isinstance(metadata, dict):
                raise RuntimeError("Ready marker is missing application identity")
            actual = metadata.get("version")
            if actual != expected_version:
                raise RuntimeError(f"Running app reported {actual}, expected {expected_version}")
            system, arch = metadata.get("os"), metadata.get("arch")
            if (system, arch) != ("darwin", expected_arch):
                raise RuntimeError(f"Running app reported {system}/{arch}, expected darwin/{expected_arch}")
            return True
    return False

def verify_database(profile, probe, output):
    subprocess.run([probe, "verify-db", profile / "jindi.db"], check=True, timeout=30,
                   stdout=output, stderr=subprocess.STDOUT)


def verify_archive_signature(app, release, output):
    commands = [["codesign", "--verify", "--deep", "--strict", app]]
    if release:
        commands.append(["xcrun", "stapler", "validate", app])
        policy_tool = pathlib.Path("/usr/bin/syspolicy_check")
        if policy_tool.is_file():
            commands.append([str(policy_tool), "distribution", app])
        else:
            commands.append(["/usr/sbin/spctl", "--assess", "--type", "execute", "--verbose=2", app])
    for command in commands:
        subprocess.run(command, check=True, timeout=60, stdout=output, stderr=subprocess.STDOUT)
    if release:
        print("Archived app passed macOS distribution policy assessment", flush=True)


def run_app(executable, helper, database_probe, profile, output, timeout, version, arch):
    env = dict(os.environ, JINDIDRAMA_DATA_DIR=str(profile))
    env.pop("JINDIDRAMA_LICENSED_MEDIA_DIR", None)
    env.pop("JINDIDRAMA_CMS_URL", None)
    with output.open("ab") as stream:
        process = subprocess.Popen([str(executable)], env=env, stdout=stream, stderr=subprocess.STDOUT)
        try:
            deadline = time.monotonic() + timeout
            while time.monotonic() < deadline:
                if process.poll() is not None:
                    raise RuntimeError(f"App exited before readiness ({process.returncode})")
                if ready(profile, version, arch) and subprocess.run(
                    [helper, "window", str(process.pid)], timeout=5
                ).returncode == 0:
                    break
                time.sleep(0.2)
            else:
                raise RuntimeError("Timed out waiting for the native window and Wails page")

            print(f"macOS native window and Wails page are ready ({version}, darwin/{arch})", flush=True)
            subprocess.run([helper, "quit", str(process.pid)], check=True, timeout=5)
            if process.wait(timeout=15) != 0:
                raise RuntimeError(f"App exit code: {process.returncode}")
            if list((profile / "logs" / "sessions").glob("run-*.session")):
                raise RuntimeError("Normal exit left a running-session marker")
            verify_database(profile, database_probe, stream)
        finally:
            if process.poll() is None:
                process.kill()
                process.wait(timeout=10)


def report_failure(error, output, profile, diagnostics_dir):
    evidence = [f"{type(error).__name__}: {error}"]
    database = profile / "jindi.db"
    if database.is_file():
        evidence.append(f"SQLite file present ({database.stat().st_size} bytes)")
    for file in (output, profile / "logs" / "jindi.log", profile / "logs" / "jindi.log.1"):
        if file.is_file():
            with file.open("rb") as stream:
                stream.seek(max(0, file.stat().st_size - 32768))
                lines = stream.read(32768).decode(errors="replace").splitlines()[-25:]
            evidence.append(f"--- {file.name} ---")
            evidence.extend(lines)
    report = "\n".join(evidence) + "\n"
    print(report, end="", flush=True)
    if diagnostics_dir:
        try:
            diagnostics_dir.mkdir(parents=True, exist_ok=True)
            with (diagnostics_dir / "smoke.log").open("x", encoding="utf-8") as file:
                file.write(report)
        except OSError as write_error:
            print(f"Could not retain the diagnostic report: {write_error}", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("archive", type=pathlib.Path)
    parser.add_argument("--version", required=True, help="expected full application version, including v prefix")
    parser.add_argument("--arch", choices=("arm64", "amd64"), help="expected running architecture (defaults to host)")
    parser.add_argument("--release", action="store_true", help="require notarization and system distribution policy approval")
    parser.add_argument("--diagnostics-dir", type=pathlib.Path, help="directory for a failure report")
    parser.add_argument("--startup-timeout", type=int, default=45, choices=range(5, 181), metavar="5–180")
    args = parser.parse_args()

    if os.uname().sysname != "Darwin":
        parser.error("this smoke test requires a macOS desktop session")
    arch = args.arch or {"arm64": "arm64", "x86_64": "amd64"}.get(os.uname().machine)
    if arch is None:
        parser.error("unknown host architecture")
    numeric = re.match(r"^v([0-9]+\.[0-9]+\.[0-9]+)(?:[-+]|$)", args.version)
    if not numeric:
        parser.error("version must start with vMAJOR.MINOR.PATCH")
    if len(args.version) > 128:
        parser.error("expected version exceeds the application marker limit")
    with tempfile.TemporaryDirectory(prefix="jindi-macos-smoke-") as workdir:
        work = pathlib.Path(workdir)
        profile = work / "profile"
        output = work / "native-output.log"
        try:
            validate_archive(args.archive)
            subprocess.run(["ditto", "-x", "-k", args.archive, work], check=True)
            app = work / "JindiDrama.app"
            info = plistlib.loads(subprocess.check_output([
                "plutil", "-convert", "xml1", "-o", "-", app / "Contents" / "Info.plist",
            ]))
            if info.get("CFBundleExecutable") != "JindiDrama":
                raise RuntimeError("Unexpected app executable name")
            if any(info.get(key) != numeric[1] for key in ("CFBundleVersion", "CFBundleShortVersionString")):
                raise RuntimeError("Archived bundle version disagrees with the expected package version")
            bundle_id = info.get("CFBundleIdentifier", "")
            if not re.fullmatch(r"[A-Za-z0-9.-]+", bundle_id):
                raise ValueError("Invalid app bundle identifier")
            with exclusive_webview_profile(bundle_id):
                with output.open("ab") as stream:
                    verify_archive_signature(app, args.release, stream)

                source = work / "window.swift"
                source.write_text(WINDOW_HELPER)
                helper = work / "window"
                subprocess.run(["xcrun", "swiftc", source, "-o", helper], check=True)
                database_probe = work / "database-probe"
                # The existing Windows probe exposes a platform-independent verify-db command.
                probe_env = dict(os.environ, GOOS="darwin", GOARCH=arch, CGO_ENABLED="0")
                with output.open("ab") as stream:
                    subprocess.run(
                        ["go", "build", "-mod=readonly", "-o", database_probe, "./cmd/windows-desktop-smoke"],
                        cwd=SOURCE_ROOT, env=probe_env, check=True, stdout=stream, stderr=subprocess.STDOUT,
                    )
                run_app(app / "Contents" / "MacOS" / "JindiDrama", helper, database_probe,
                        profile, output, args.startup_timeout, args.version, arch)
        except Exception as error:
            report_failure(error, output, profile, args.diagnostics_dir)
            raise
        print("Archive startup, normal exit and isolated SQLite verified")


if __name__ == "__main__":
    main()
