"""Run the delivered script against temporary filesystem/service fixtures.

All destructive commands are confined to the fixture. No host service is called.
"""
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tempfile
import unittest

SOURCE = Path(__file__).resolve().parents[1] / "uninstall-agent.sh"
STUB = r'''
import json, os, pathlib, shutil, sys
name = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
root = pathlib.Path(os.environ["KOMARI_TEST_ROOT"])
with (root / "calls.jsonl").open("a") as stream:
    stream.write(json.dumps([name, *args]) + "\n")
if name == "uname":
    print("Linux"); sys.exit(0)
if name == "id":
    print(os.environ.get("KOMARI_TEST_UID", "0")); sys.exit(0)
if name == "systemctl":
    args = [a for a in args if a != "--user"]
    unit = pathlib.Path(os.environ["KOMARI_TEST_UNIT"])
    if args[0] == "show":
        if unit.exists():
            if "FragmentPath" in args: print(unit)
            else:
                for line in unit.read_text().splitlines():
                    if line.startswith("WorkingDirectory="): print(line.split("=", 1)[1])
        sys.exit(0)
    if args[0] == "is-active":
        sys.exit(0 if os.environ.get("KOMARI_TEST_ACTIVE") == "1" else 3)
    if args[0] == "stop" and os.environ.get("KOMARI_TEST_STOP_FAIL") == "1": sys.exit(1)
    if args[0] == "disable" and os.environ.get("KOMARI_TEST_DISABLE_FAIL") == "1": sys.exit(1)
    sys.exit(0)
if name in ("rm", "rmdir"):
    for arg in args:
        if arg.startswith("-"): continue
        p = pathlib.Path(arg)
        # Resolve the parent only: unlinking a final symlink must preserve its target.
        if not p.parent.resolve().is_relative_to(root):
            print("TEST BLOCKED HOST DELETION: " + str(p), file=sys.stderr); sys.exit(90)
        if name == "rmdir":
            try: p.rmdir()
            except OSError: sys.exit(1)
        elif p.is_symlink(): p.unlink()
        elif p.is_dir(): shutil.rmtree(p)
        elif p.exists(): p.unlink()
    sys.exit(0)
if name in ("sleep", "kill", "rc-service", "rc-update", "initctl"):
    sys.exit(0)
raise RuntimeError(name)
'''


class UninstallTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="komari-uninstall-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.bin = self.root / "bin"
        self.bin.mkdir()
        for name in ("uname", "id", "systemctl", "rm", "rmdir", "kill", "sleep"):
            self.stub(name)
        self.agent = self.root / "opt/komari"
        self.agent.mkdir(parents=True)
        self.unit = self.root / "etc/systemd/system/komari-agent.service"
        self.unit.parent.mkdir(parents=True)
        self.env = dict(os.environ, PATH=f"{self.bin}:/usr/bin:/bin",
                        KOMARI_TEST_ROOT=str(self.root), KOMARI_TEST_BIN=str(self.bin),
                        KOMARI_TEST_UNIT=str(self.unit))
        text = SOURCE.read_text()
        for key, value in {"systemd_dir": "etc/systemd/system", "initd_dir": "etc/init.d",
                           "upstart_dir": "etc/init", "proc_dir": "proc",
                           "default_dir": "opt/komari", "log_dir": "var/log/komari"}.items():
            original = next(line for line in text.splitlines() if line.startswith(key + "="))
            text = text.replace(original, key + "=" + shlex.quote(str(self.root / value)), 1)
        text = text.replace("[ -f /etc/rc.common ]", "[ -f " + shlex.quote(str(self.root / "etc/rc.common")) + " ]")
        text = text.replace("set -eu\n", 'set -eu\nkill() { "$KOMARI_TEST_BIN/kill" "$@"; }\n', 1)
        self.script = self.root / "uninstall-agent.sh"
        self.script.write_text(text)

    def stub(self, name):
        p = self.bin / name
        p.write_text("#!" + sys.executable + "\n" + STUB)
        p.chmod(0o755)

    def files(self):
        for name in ("agent", "auto-discovery.json", "managed-config.json", "managed-config.json.tmp",
                     "managed-config.json.bak", "net_static.json", "net_static.json.tmp", "net_static.json.bak",
                     ".agent.old", ".agent.new", "agent.before-batch-update-20260922", "agent.before-canary-1"):
            (self.agent / name).write_text("fixture")

    def systemd(self):
        self.unit.write_text(f"[Unit]\nDescription=Komari Agent Service\n[Service]\n"
                             f"ExecStart={self.agent}/agent -e https://example.invalid -t secret\n"
                             f"WorkingDirectory={self.agent}\n")

    def run_script(self, *args, ok=True, shell="/bin/sh"):
        result = subprocess.run([shell, str(self.script), *args], env=self.env,
                                text=True, errors="backslashreplace", capture_output=True, timeout=20)
        self.assertNotIn("secret", result.stdout + result.stderr)
        if ok: self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        else: self.assertNotEqual(result.returncode, 0)
        return result

    def calls(self):
        path = self.root / "calls.jsonl"
        return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []

    def test_systemd_preserves_panel_data_and_unknown_files(self):
        self.files(); self.systemd()
        (self.agent / "data").mkdir(); (self.agent / "data/komari.db").write_text("database")
        (self.agent / "komari").write_text("panel")
        dropins = self.unit.parent / "komari-agent.service.d"
        dropins.mkdir(); (dropins / "override.conf").write_text("fixture")
        logs = self.root / "var/log/komari"
        logs.mkdir(parents=True); (logs / "agent.log.1").write_text("log")
        (logs / "panel.log").write_text("keep")
        self.run_script()
        self.assertEqual(sorted(p.name for p in self.agent.iterdir()), ["data", "komari"])
        self.assertEqual((self.agent / "data/komari.db").read_text(), "database")
        self.assertFalse(self.unit.exists()); self.assertFalse(dropins.exists())
        self.assertEqual(sorted(p.name for p in logs.iterdir()), ["panel.log"])

    def test_empty_install_directory_removed_and_repeatable(self):
        self.files(); self.systemd(); self.run_script()
        self.assertFalse(self.agent.exists()); self.run_script()

    def test_historical_backups_removed_and_notes_preserved(self):
        names = ("agent.backup-20260922-1556", "agent.backup-1.gz", "agent.pre-air-redeploy",
                 "agent.hardening.gz", "komari-agent.backup-1", "komari-agent.pre-custom",
                 "komari-agent.hardening.gz")
        for name in (*names, "agent.notes"):
            (self.agent / name).write_text("fixture")
        self.run_script()
        self.assertEqual(sorted(p.name for p in self.agent.iterdir()), ["agent.notes"])

    def test_service_backups_removed_without_active_unit(self):
        names = (".before-komari-air-0.1.5", ".pre-air-redeploy", ".backup-1", ".bak")
        for suffix in names:
            Path(str(self.unit) + suffix).write_text(f"ExecStart={self.agent}/agent -t secret\n")
        unrelated = Path(str(self.unit) + ".before-unrelated")
        unrelated.write_text("ExecStart=/usr/bin/unrelated\n")
        self.run_script()
        for suffix in names:
            self.assertFalse(Path(str(self.unit) + suffix).exists())
        self.assertTrue(unrelated.exists())

    def test_dry_run_makes_no_changes(self):
        self.files(); self.systemd()
        process = self.root / "proc/999999/exe"
        process.parent.mkdir(parents=True); process.symlink_to(self.agent / "agent")
        before = sorted(p.name for p in self.agent.iterdir())
        result = self.run_script("--dry-run")
        self.assertEqual(before, sorted(p.name for p in self.agent.iterdir()))
        self.assertTrue(self.unit.exists()); self.assertIn("预览", result.stdout)
        self.assertTrue(all(c[0] in ("uname", "id", "systemctl") for c in self.calls()))
        self.assertTrue(all(c[1] == "show" for c in self.calls() if c[0] == "systemctl"))

    def test_stop_failure_preserves_files(self):
        self.files(); self.systemd(); self.env["KOMARI_TEST_STOP_FAIL"] = "1"
        self.run_script(ok=False)
        self.assertTrue(self.unit.exists()); self.assertTrue((self.agent / "agent").exists())
        self.assertFalse(any(c[0] == "rm" for c in self.calls()))

    def test_disable_failure_preserves_files(self):
        self.files(); self.systemd(); self.env["KOMARI_TEST_DISABLE_FAIL"] = "1"
        self.run_script(ok=False)
        self.assertTrue(self.unit.exists()); self.assertTrue((self.agent / "agent").exists())

    def test_unrelated_service_rejected(self):
        self.files(); self.unit.write_text("Description=SSH service\nExecStart=/usr/sbin/sshd\n")
        self.run_script(ok=False); self.assertTrue(self.unit.exists())
        self.assertFalse(any(c[0] == "rm" for c in self.calls()))

    def test_description_alone_does_not_authorize_uninstall(self):
        self.files(); self.systemd()
        self.unit.write_text(self.unit.read_text().replace(str(self.agent / "agent"), "/usr/sbin/sshd"))
        self.run_script(ok=False); self.assertTrue((self.agent / "agent").exists())

    def test_explicit_directory_mismatch_rejected(self):
        self.files(); self.systemd()
        self.run_script("--install-dir", str(self.root / "other"), ok=False)
        self.assertTrue(self.unit.exists())

    def test_custom_directory_and_service_detected(self):
        self.agent = self.root / "srv/custom agent"
        self.agent.mkdir(parents=True); self.files()
        self.unit = self.unit.parent / "my-agent.service"
        self.env["KOMARI_TEST_UNIT"] = str(self.unit); self.systemd()
        self.run_script("--service-name", "my-agent")
        self.assertFalse(self.agent.exists()); self.assertFalse(self.unit.exists())

    def test_dangerous_paths_rejected(self):
        for path in ("/", "/etc", "/usr/bin", "/opt", "/tmp", str(self.root / "../escape")):
            with self.subTest(path=path): self.run_script("--install-dir", path, ok=False)
        self.assertFalse(any(c[0] == "rm" for c in self.calls()))

    def test_symlink_to_system_directory_rejected(self):
        link = self.root / "bad-install"
        link.symlink_to("/usr/bin", target_is_directory=True)
        self.run_script("--install-dir", str(link), ok=False)

    def test_symlink_file_target_preserved(self):
        outside = self.root / "important.json"; outside.write_text("keep")
        (self.agent / "auto-discovery.json").symlink_to(outside)
        self.run_script(); self.assertEqual(outside.read_text(), "keep")

    def test_extra_json_configs_removed(self):
        self.files(); self.systemd()
        extra = self.root / "custom-configs"; extra.mkdir()
        for name in ("start.json", "start.json.bak", "managed.json", "managed.json.tmp"):
            (extra / name).write_text("fixture")
        self.run_script("--config-file", str(extra / "start.json"), "--managed-config", str(extra / "managed.json"))
        self.assertEqual(list(extra.iterdir()), [])

    def test_unexpected_directory_not_recursively_deleted(self):
        (self.agent / "auto-discovery.json").mkdir()
        (self.agent / "auto-discovery.json/keep").write_text("keep")
        self.run_script(ok=False)
        self.assertTrue((self.agent / "auto-discovery.json/keep").exists())

    def test_stubborn_agent_process_aborts_config_deletion(self):
        self.files(); self.systemd()
        process = self.root / "proc/999999/exe"
        process.parent.mkdir(parents=True); process.symlink_to(self.agent / "agent")
        self.run_script(ok=False)
        self.assertTrue((self.agent / "auto-discovery.json").exists())
        self.assertIn(["kill", "-TERM", "999999"], self.calls())

    def test_unrelated_agent_process_not_killed(self):
        self.files()
        process = self.root / "proc/999999/exe"
        process.parent.mkdir(parents=True); process.symlink_to(self.root / "unrelated/agent")
        self.run_script(); self.assertFalse(any(c[0] == "kill" for c in self.calls()))

    def test_active_service_without_definition_aborts(self):
        self.files(); self.env["KOMARI_TEST_ACTIVE"] = "1"
        self.run_script(ok=False); self.assertTrue((self.agent / "agent").exists())

    def test_user_service_uses_user_systemctl(self):
        self.env["KOMARI_TEST_UID"] = "1000"
        self.env["XDG_CONFIG_HOME"] = str(self.root / "user-config")
        self.env["XDG_DATA_HOME"] = str(self.root / "user-data")
        self.agent = self.root / "user-data/komari"; self.agent.mkdir(parents=True); self.files()
        self.unit = self.root / "user-config/systemd/user/komari-agent.service"
        self.unit.parent.mkdir(parents=True); self.env["KOMARI_TEST_UNIT"] = str(self.unit); self.systemd()
        system_backup = self.root / "etc/init.d/komari-agent.pre-test"
        system_backup.parent.mkdir(parents=True)
        system_backup.write_text(f"command={self.agent}/agent\n")
        self.run_script("--user")
        self.assertFalse(self.agent.exists())
        self.assertTrue(system_backup.exists())
        self.assertTrue(all(c[1] == "--user" for c in self.calls() if c[0] == "systemctl"))

    def test_openrc_removes_all_runlevels(self):
        self.files(); self.stub("rc-service"); self.stub("rc-update")
        unit = self.root / "etc/init.d/komari-agent"; unit.parent.mkdir(parents=True)
        unit.write_text(f'#!/sbin/openrc-run\nname="Komari Agent Service"\ncommand="{self.agent}/agent"\ndirectory="{self.agent}"\n')
        self.run_script(); self.assertFalse(unit.exists()); self.assertFalse(self.agent.exists())
        self.assertIn(["rc-update", "--all", "delete", "komari-agent"], self.calls())

    def test_procd_stops_and_disables(self):
        self.files()
        marker = self.root / "etc/rc.common"; marker.parent.mkdir(parents=True, exist_ok=True); marker.touch()
        unit = self.root / "etc/init.d/komari-agent"; unit.parent.mkdir(parents=True)
        unit.write_text(f'#!/bin/sh\nUSE_PROCD=1\nPROG="{self.agent}/agent"\nstart_service() {{ procd_open_instance; }}\nprintf "%s\\n" "$1" >> "{self.root}/procd-calls"\n')
        unit.chmod(0o755)
        self.run_script(); self.assertFalse(unit.exists()); self.assertFalse(self.agent.exists())
        self.assertEqual((self.root / "procd-calls").read_text(), "stop\ndisable\n")

    def test_dash_compatibility(self):
        if not Path("/bin/dash").exists():
            self.skipTest("dash is not installed")
        self.files(); self.systemd(); self.run_script(shell="/bin/dash")
        self.assertFalse(self.agent.exists())


if __name__ == "__main__": unittest.main(verbosity=2)
