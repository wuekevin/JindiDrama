import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("sign-macos-release.sh").resolve()
CREDENTIALS = {
    "MACOS_CERT_P12": "Zml4dHVyZQ==",
    "MACOS_CERT_PASSWORD": "fixture",
    "MACOS_SIGN_IDENTITY": "Developer ID Application: Fixture",
    "APPLE_ID": "fixture@example.test",
    "APPLE_TEAM_ID": "TESTTEAM01",
    "APPLE_APP_PASSWORD": "fixture",
}
TOOL = """#!/usr/bin/env python3
import json
import os
from pathlib import Path
import sys
import time

name = Path(sys.argv[0]).name
args = sys.argv[1:]
if any(key in os.environ for key in json.loads(os.environ["RELEASE_CREDENTIAL_KEYS"])):
    sys.exit("A release credential reached an unrelated child process")
key = name + " " + args[0]
with open(os.environ["TOOL_LOG"], "a", encoding="utf-8") as log:
    log.write(json.dumps([key, args]) + "\\n")

if key == "security create-keychain":
    Path(args[-1]).touch()
if key == "security import":
    p12 = Path(args[1])
    assert p12.read_bytes() == b"fixture"
    assert p12.stat().st_mode & 0o077 == 0
if key == "security delete-keychain":
    Path(args[-1]).unlink()
if key == "codesign --force":
    keychain = Path(args[args.index("--keychain") + 1])
    assert keychain.is_file()
    assert not (keychain.parent / "certificate.p12").exists()
if key == "xcrun notarytool":
    assert Path(args[2]).is_file()
    assert "--wait" in args
    print(os.environ.get("NOTARY_RESPONSE", '{"status":"Accepted"}'))
if key == os.environ.get("CANCEL_TOOL"):
    os.kill(os.getppid(), int(os.environ["CANCEL_SIGNAL"]))
    time.sleep(30)
if key == os.environ.get("FAIL_TOOL"):
    sys.exit(19)
"""


class MacOSReleaseSigningTest(unittest.TestCase):
    def setUp(self):
        work = tempfile.TemporaryDirectory()
        self.addCleanup(work.cleanup)
        self.root = Path(work.name)
        self.app = self.root / "Test App.app"
        self.app.mkdir()
        (self.app / "fixture.txt").write_text("fixture")
        tools = self.root / "tools"
        tools.mkdir()
        fixture = tools / "tool"
        fixture.write_text(TOOL)
        fixture.chmod(0o700)
        for name in ("security", "codesign", "xcrun"):
            (tools / name).symlink_to(fixture)
        self.temporary = self.root / "temporary"
        self.temporary.mkdir()
        self.log = self.root / "tools.log"
        self.env = dict(os.environ, PATH=f"{tools}:{os.environ['PATH']}",
                        RUNNER_TEMP=str(self.temporary), TOOL_LOG=str(self.log))

    def run_operation(self, operation, fail_tool="", cancel_tool="", cancel_signal=15,
                      expected_status=None, error_message=None, notary_response=None, team_id=None):
        self.log.unlink(missing_ok=True)
        prefix = "MACOS_" if operation == "sign" else "APPLE_"
        credentials = {key: value for key, value in CREDENTIALS.items() if key.startswith(prefix)}
        env = {key: value for key, value in self.env.items() if key not in CREDENTIALS}
        env.update(credentials, FAIL_TOOL=fail_tool, CANCEL_TOOL=cancel_tool, CANCEL_SIGNAL=str(cancel_signal),
                   RELEASE_CREDENTIAL_KEYS=json.dumps(list(CREDENTIALS)))
        if notary_response is not None:
            env["NOTARY_RESPONSE"] = notary_response
        if team_id is not None:
            env["APPLE_TEAM_ID"] = team_id
        result = subprocess.run(["bash", SCRIPT, operation, self.app], env=env, capture_output=True, text=True, timeout=10)
        self.assertEqual(list(self.temporary.iterdir()), [])
        if expected_status is None:
            expected_status = 128 + cancel_signal if cancel_tool else 19 if fail_tool else 0
        self.assertEqual(result.returncode, expected_status, result.stderr)
        if error_message:
            self.assertIn(error_message, result.stderr)
        return [json.loads(line) for line in self.log.read_text().splitlines()] if self.log.exists() else []

    def test_signs_with_explicit_keychain_and_removes_credentials(self):
        calls = self.run_operation("sign")
        self.assertEqual([key for key, args in calls], [
            "security create-keychain", "security unlock-keychain", "security import",
            "security set-key-partition-list", "codesign --force", "codesign --verify",
            "security delete-keychain",
        ])
        args = calls[4][1]
        self.assertIn("--timestamp", args)
        self.assertEqual(args[args.index("--options") + 1], "runtime")

    def test_signing_failure_still_deletes_temporary_keychain(self):
        calls = self.run_operation("sign", "security import")
        self.assertEqual([key for key, args in calls][-2:], [
            "security import", "security delete-keychain",
        ])

    def test_termination_removes_temporary_keychain(self):
        calls = self.run_operation("sign", cancel_tool="security import")
        self.assertEqual([key for key, args in calls][-2:], [
            "security import", "security delete-keychain",
        ])

    def test_notarizes_and_validates_before_leaving(self):
        calls = self.run_operation("notarize")
        self.assertEqual([args[:2] for key, args in calls], [
            ["--verify", "--deep"], ["notarytool", "submit"], ["stapler", "staple"],
            ["stapler", "validate"], ["--verify", "--deep"],
        ])
        requirement = next(arg for arg in calls[0][1] if arg.startswith("-R="))
        self.assertIn('certificate leaf[subject.OU] = "TESTTEAM01"', requirement)
        self.assertIn('certificate leaf[field.1.2.840.113635.100.6.1.13] exists', requirement)
        args = calls[1][1]
        self.assertEqual(args[args.index("--output-format") + 1], "json")

    def test_cancellation_interrupts_waiting_for_notary_service(self):
        calls = self.run_operation("notarize", cancel_tool="xcrun notarytool", cancel_signal=2)
        self.assertEqual([key for key, args in calls], ["codesign --verify", "xcrun notarytool"])

    def test_notary_failure_cleans_archive_and_skips_staple(self):
        calls = self.run_operation("notarize", "xcrun notarytool")
        self.assertEqual([key for key, args in calls], ["codesign --verify", "xcrun notarytool"])

    def test_does_not_submit_an_app_outside_the_expected_developer_team(self):
        calls = self.run_operation("notarize", "codesign --verify")
        self.assertEqual([key for key, args in calls], ["codesign --verify"])

    def test_rejects_malformed_team_id_before_using_tools(self):
        calls = self.run_operation("notarize", team_id='BAD"TEAMID',
                                   expected_status=1, error_message="Invalid Apple Team ID")
        self.assertEqual(calls, [])

    def test_requires_accepted_status_even_when_notarytool_exits_successfully(self):
        for response in ('{"status":"Invalid"}', '{"status":"In Progress"}', '{"status":true}', '{}', 'truncated'):
            with self.subTest(response=response):
                calls = self.run_operation("notarize", notary_response=response, expected_status=1,
                                           error_message="Apple did not accept this notarization submission")
                self.assertEqual([key for key, args in calls], ["codesign --verify", "xcrun notarytool"])


if __name__ == "__main__":
    unittest.main()
