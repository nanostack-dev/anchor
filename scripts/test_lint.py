#!/usr/bin/env python3
"""Exercise the lint command boundary without analysing the application."""

import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile
import time
import unittest


ROOT = Path(__file__).resolve().parent.parent
FAKE_LINTER = """import json, os, pathlib, sys, time
args = sys.argv[1:]
with open(os.environ['LINT_TEST_CALLS'], 'a') as log:
    log.write(json.dumps({'cwd': os.getcwd(), 'args': args}) + '\\n')
if args == ['version']:
    print('golangci-lint has version ' + os.environ.get('LINT_TEST_VERSION', '2.13.0'))
elif args[:2] == ['config', 'verify']:
    print('config diagnostics')
    sys.exit(int(os.environ.get('LINT_TEST_CONFIG_EXIT', '0')))
elif args[0] in ('run', 'fmt'):
    if '--fix' in args or (args[0] == 'fmt' and '--diff' not in args):
        pathlib.Path('fixed').write_text('fixed')
    output = os.environ.get('LINT_TEST_OUTPUT', '' if args[0] == 'fmt' else 'complete diagnostics')
    if output:
        print(output)
    time.sleep(float(os.environ.get('LINT_TEST_SLEEP', '0')))
    sys.exit(int(os.environ.get('LINT_TEST_EXIT', '0')))
else:
    sys.exit(99)
"""


class LintCommandTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="anchor-lint-command-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = (Path(self.temp.name) / "checkout with spaces").resolve()
        self.backend = self.root / "apps/anchor"
        self.sdk = self.root / "clients/go"
        (self.root / "scripts").mkdir(parents=True)
        self.backend.mkdir(parents=True)
        self.sdk.mkdir(parents=True)
        shutil.copyfile(ROOT / "scripts/lint.sh", self.root / "scripts/lint.sh")
        (self.root / "scripts/lint.sh").chmod(0o755)
        shutil.copyfile(ROOT / "lint-fix.sh", self.root / "lint-fix.sh")
        shutil.copyfile(ROOT / ".golangci-lint-version", self.root / ".golangci-lint-version")
        (self.root / ".golangci.yml").write_text('version: "2"\n')
        (self.backend / "go.mod").write_text("module fixture\n")
        (self.sdk / "go.mod").write_text("module fixture/clients/go\n")
        self.bin = Path(self.temp.name) / "bin"
        self.bin.mkdir()
        fake = self.bin / "golangci-lint"
        fake.write_text(f"#!{sys.executable}\n{FAKE_LINTER}")
        fake.chmod(0o755)
        self.calls_file = Path(self.temp.name) / "calls.jsonl"
        self.env = {
            **os.environ,
            "PATH": f"{self.bin}:/usr/bin:/bin",
            "TMPDIR": self.temp.name,
            "LINT_TEST_CALLS": str(self.calls_file),
            "LINT_TEST_VERSION": (ROOT / ".golangci-lint-version").read_text().strip().removeprefix("v"),
        }
        self.write_file("apps/anchor/backend.go", "package fixture\n")
        self.write_file("apps/anchor/internal/flows/first.go", "package flows\n")
        self.write_file("apps/anchor/internal/flows/second.go", "package flows\n")
        self.write_file("clients/go/anchorsdk/client.go", "package anchorsdk\n")
        self.git("init", "-q", "-b", "main")
        self.git("add", ".")
        self.git("commit", "-qm", "initial fixture")
        self.git("update-ref", "refs/remotes/origin/main", "HEAD")

    def write_file(self, relative, content="package changed\n"):
        path = self.root / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)

    def git(self, *args):
        return subprocess.run(
            ["git", "-c", "user.name=Lint Test", "-c", "user.email=lint@example.invalid",
             "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", *args],
            cwd=self.root, env=self.env, check=True, capture_output=True, text=True,
        )

    def targets(self):
        args = self.calls()[-1]["args"]
        marker = "--diff" if args[0] == "fmt" else "5m"
        return args[args.index(marker) + 1:]

    def run_lint(self, *args, **env):
        return subprocess.run(
            ["/bin/bash", str(self.root / "scripts/lint.sh"), *args],
            cwd=self.temp.name,
            env={**self.env, **env},
            capture_output=True,
            text=True,
            timeout=10,
        )

    def calls(self):
        if not self.calls_file.exists():
            return []
        return [json.loads(line) for line in self.calls_file.read_text().splitlines()]

    def test_backend_and_root_config_resolve_from_an_unrelated_directory(self):
        result = self.run_lint("backend")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        call = self.calls()[-1]
        self.assertEqual(Path(call["cwd"]), self.backend)
        self.assertEqual(call["args"], [
            "run", "--config", str(self.root / ".golangci.yml"),
            "--path-mode=abs", "--timeout", "5m", "./...",
        ])
        self.assertFalse((self.backend / "fixed").exists())

    def test_fix_is_explicit(self):
        result = self.run_lint("backend", "--fix")
        self.assertEqual(result.returncode, 0)
        self.assertTrue((self.backend / "fixed").exists())

    def test_legacy_fix_command_delegates_from_an_unrelated_directory(self):
        result = subprocess.run(
            ["/bin/bash", str(self.root / "lint-fix.sh")],
            cwd=self.temp.name, env=self.env, capture_output=True, text=True, timeout=10,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(Path(self.calls()[-1]["cwd"]), self.backend)
        self.assertIn("--fix", self.calls()[-1]["args"])
        self.assertEqual(self.calls()[-1]["args"][-1], "./...")
        self.assertTrue((self.backend / "fixed").exists())

    def test_sdk_dependency_manifest_requires_full_backend_lint(self):
        self.write_file("clients/go/go.mod", "module fixture/clients/go\n// Dependency edit.\n")
        result = self.run_lint("backend", "--changed")
        self.assertEqual(result.returncode, 0)
        self.assertEqual(self.targets(), ["./..."])
        self.assertIn("fallback: local_dependency_changed\n", result.stdout)

    def test_backend_gorules_and_generator_changes_require_full_scope(self):
        for relative, content in [
            ("apps/anchor/gorules/project.go", "package gorules\n"),
            ("apps/anchor/generate_anchor.sh", "echo generator\n"),
            ("Makefile", "generate:\n\techo generator\n"),
        ]:
            with self.subTest(relative=relative):
                self.write_file(relative, content)
                result = self.run_lint("backend", "--changed")
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(self.targets(), ["./..."])
                (self.root / relative).unlink()

    def test_original_exit_status_and_failure_category(self):
        for exit_code, reason, output in [
            (1, "findings", "an actual finding"),
            (3, "analysis_error", "ruleguard dependency failed to load"),
            (3, "already_running", "parallel golangci-lint is running"),
            (4, "timeout", "Timeout exceeded\n0 issues."),
            (5, "no_go_files", "no go files to analyze"),
            (6, "config_error", "configuration not detected"),
            (7, "analysis_error", "an error was logged"),
        ]:
            with self.subTest(exit_code=exit_code, reason=reason):
                result = self.run_lint("backend", LINT_TEST_EXIT=str(exit_code), LINT_TEST_OUTPUT=output)
                self.assertEqual(result.returncode, exit_code)
                self.assertIn(f"reason: {reason}\n", result.stdout)
                self.assertIn("LINT FAILED", result.stdout)
                log_path = Path(next(line[5:] for line in result.stdout.splitlines() if line.startswith("log: ")))
                self.assertIn(output, log_path.read_text())
                self.assertEqual(log_path.stat().st_mode & 0o777, 0o600)
                self.assertEqual(log_path.parent.stat().st_mode & 0o777, 0o700)

    def test_version_mismatch_stops_before_config_or_analysis(self):
        result = self.run_lint("backend", LINT_TEST_VERSION="2.12.0")
        self.assertEqual(result.returncode, 2)
        self.assertIn("reason: version_mismatch", result.stdout)
        self.assertEqual([call["args"] for call in self.calls()], [["version"]])

    def test_config_failure_stops_before_analysis(self):
        result = self.run_lint("backend", LINT_TEST_CONFIG_EXIT="3")
        self.assertEqual(result.returncode, 3)
        self.assertIn("reason: config_error", result.stdout)
        self.assertEqual(len(self.calls()), 2)

    def test_missing_root_config_stops_before_analysis(self):
        (self.root / ".golangci.yml").unlink()
        result = self.run_lint("backend")
        self.assertEqual(result.returncode, 2)
        self.assertIn("reason: setup_error", result.stdout)
        self.assertEqual(len(self.calls()), 1)

    def test_pin_requires_an_exact_version(self):
        for pin in ["latest\n", "v2.13\n", "v2.13.0\nextra\n"]:
            with self.subTest(pin=pin):
                (self.root / ".golangci-lint-version").write_text(pin)
                result = self.run_lint("backend")
                self.assertEqual(result.returncode, 2)
                self.assertIn("reason: setup_error", result.stdout)
                self.assertEqual(self.calls(), [])

    def test_missing_binary_reports_installation(self):
        (self.bin / "golangci-lint").unlink()
        result = self.run_lint("backend")
        self.assertEqual(result.returncode, 2)
        self.assertIn("reason: tool_missing", result.stdout)

    def test_usage_and_help_do_not_start_the_linter(self):
        for args in [[], ["other"], ["backend", "--timeout"], ["backend", "--fix", "extra"],
                     ["backend", "--base", "HEAD"], ["backend", "--changed", "--base"],
                     ["backend", "--changed", "--format-changed"],
                     ["backend", "--changed", "--base", "--fix"]]:
            with self.subTest(args=args):
                self.assertEqual(self.run_lint(*args).returncode, 2)
        self.assertEqual(self.run_lint("--help").returncode, 0)
        self.assertEqual(self.calls(), [])

    def test_changed_packages_include_branch_index_worktree_and_untracked(self):
        self.write_file("apps/anchor/committed/branch.go")
        self.git("add", "apps/anchor/committed")
        self.git("commit", "-qm", "branch Go edit")
        self.write_file("apps/anchor/staged/staged_test.go")
        self.git("add", "apps/anchor/staged")
        self.write_file("apps/anchor/internal/flows/first.go", "package flows\n// Worktree edit.\n")
        self.write_file("apps/anchor/internal/flows/second.go", "package flows\n// Another edit.\n")
        self.write_file("apps/anchor/untracked/new.go")
        result = self.run_lint("backend", "--changed")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(set(self.targets()), {"./committed", "./staged", "./internal/flows", "./untracked"})
        self.assertEqual(len(self.targets()), 4)
        self.assertIn("scope: changed_packages\n", result.stdout)
        self.assertIn("coverage: iteration_only\n", result.stdout)

    def test_explicit_head_base_limits_selection_to_local_edits(self):
        self.write_file("apps/anchor/committed/branch.go")
        self.git("add", "apps/anchor/committed")
        self.git("commit", "-qm", "branch Go edit")
        self.write_file("apps/anchor/internal/flows/first.go", "package flows\n// Local edit.\n")
        result = self.run_lint("backend", "--changed", "--base", "HEAD")
        self.assertEqual(result.returncode, 0)
        self.assertEqual(self.targets(), ["./internal/flows"])

    def test_staged_change_is_selected_when_worktree_matches_the_base(self):
        original = (self.root / "apps/anchor/internal/flows/first.go").read_text()
        self.write_file("apps/anchor/internal/flows/first.go", "package flows\n// Staged edit.\n")
        self.git("add", "apps/anchor/internal/flows/first.go")
        self.write_file("apps/anchor/internal/flows/first.go", original)
        result = self.run_lint("backend", "--changed")
        self.assertEqual(result.returncode, 0)
        self.assertEqual(self.targets(), ["./internal/flows"])

    def test_merge_base_ignores_changes_only_on_the_base_branch(self):
        initial = self.git("rev-parse", "HEAD").stdout.strip()
        self.git("checkout", "-qb", "other-base")
        self.write_file("apps/anchor/base-only/base.go")
        self.git("add", "apps/anchor/base-only")
        self.git("commit", "-qm", "base branch only")
        self.git("checkout", "-qb", "work", initial)
        self.write_file("apps/anchor/local/new.go")
        result = self.run_lint("backend", "--changed", "--base", "other-base")
        self.assertEqual(result.returncode, 0)
        self.assertEqual(self.targets(), ["./local"])

    def test_changed_paths_with_spaces_and_newlines_are_single_arguments(self):
        relative = "odd directory\nnext/-new file_test.go"
        self.write_file("apps/anchor/" + relative)
        result = self.run_lint("backend", "--format-changed")
        self.assertEqual(result.returncode, 0)
        self.assertEqual(self.targets(), ["./" + relative])

    def test_go_files_outside_backend_are_not_selected(self):
        self.write_file("anchor-ui/example.go")
        self.write_file("other/backend.go")
        result = self.run_lint("backend", "--changed")
        self.assertEqual(result.returncode, 0)
        self.assertIn("reason: no_changes", result.stdout)
        self.assertEqual(len(self.calls()), 2)

    def test_backend_falls_back_for_its_local_sdk_dependency(self):
        self.write_file("clients/go/anchorsdk/client.go", "package anchorsdk\n// Dependency edit.\n")
        result = self.run_lint("backend", "--changed")
        self.assertEqual(result.returncode, 0)
        self.assertEqual(self.targets(), ["./..."])
        self.assertIn("fallback: local_dependency_changed\n", result.stdout)

    def test_no_go_changes_skip_analysis_without_claiming_full_coverage(self):
        self.write_file("notes.md", "documentation")
        result = self.run_lint("backend", "--changed")
        self.assertEqual(result.returncode, 0)
        self.assertIn("reason: no_changes\n", result.stdout)
        self.assertIn("coverage: iteration_only\n", result.stdout)
        self.assertEqual(len(self.calls()), 2)

    def test_ignored_files_and_nested_module_files_are_not_selected(self):
        self.write_file(".gitignore", "apps/anchor/ignored/\n")
        self.write_file("apps/anchor/ignored/unused.go")
        self.write_file("apps/anchor/nested/go.mod", "module nested\n")
        self.write_file("apps/anchor/nested/nested.go")
        self.git("add", "apps/anchor/nested/go.mod")
        self.git("commit", "-qm", "nested module boundary")
        result = self.run_lint("backend", "--changed", "--base", "HEAD")
        self.assertEqual(result.returncode, 0)
        self.assertIn("reason: no_changes\n", result.stdout)
        self.assertEqual(len(self.calls()), 2)

    def test_missing_base_falls_back_to_full_lint(self):
        result = self.run_lint("backend", "--changed", "--base", "missing-reference")
        self.assertEqual(result.returncode, 0)
        self.assertEqual(self.targets(), ["./..."])
        self.assertIn("scope: full\n", result.stdout)
        self.assertIn("coverage: full\n", result.stdout)
        self.assertIn("fallback: base_unavailable\n", result.stdout)

    def test_git_diff_failure_falls_back_instead_of_skipping(self):
        real_git = shutil.which("git", path="/usr/bin:/bin")
        fake_git = self.bin / "git"
        fake_git.write_text(f'#!/bin/bash\nfor arg in "$@"; do if [[ $arg == diff ]]; then exit 9; fi; done\nexec "{real_git}" "$@"\n')
        fake_git.chmod(0o755)
        result = self.run_lint("backend", "--changed")
        self.assertEqual(result.returncode, 0)
        self.assertEqual(self.targets(), ["./..."])
        self.assertIn("fallback: change_detection_failed\n", result.stdout)

    def test_global_and_generated_inputs_require_full_scope(self):
        for path, content in [
            ("apps/anchor/go.mod", "module fixture\n// Changed.\n"),
            (".golangci.yml", 'version: "2"\n# Changed.\n'),
            (".golangci-lint-version", (ROOT / ".golangci-lint-version").read_text() + "\n"),
            ("scripts/generate.sh", "echo generator\n"),
            ("apps/anchor/gorules/project.go", "package gorules\n"),
            ("apps/anchor/cmd/http/openapi.yaml", "openapi: 3.0.0\n"),
            ("apps/anchor/internal/client.gen.go", "package internal\n"),
            ("apps/anchor/internal/output.go", "// Code generated by a tool. DO NOT EDIT.\npackage internal\n"),
        ]:
            with self.subTest(path=path):
                previous = (self.root / path).read_bytes() if (self.root / path).exists() else None
                self.write_file(path, content)
                result = self.run_lint("backend", "--changed")
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(self.targets(), ["./..."])
                self.assertIn("scope: full\n", result.stdout)
                if previous is None:
                    (self.root / path).unlink()
                else:
                    (self.root / path).write_bytes(previous)

    def test_deleted_and_renamed_files_require_full_scope(self):
        (self.root / "apps/anchor/internal/flows/first.go").unlink()
        self.assertEqual(self.run_lint("backend", "--changed").returncode, 0)
        self.assertEqual(self.targets(), ["./..."])
        self.git("checkout", "--", "apps/anchor/internal/flows/first.go")
        (self.root / "apps/anchor/moved").mkdir()
        self.git("mv", "apps/anchor/internal/flows/first.go", "apps/anchor/moved/renamed.go")
        self.assertEqual(self.run_lint("backend", "--changed").returncode, 0)
        self.assertEqual(self.targets(), ["./..."])

    def test_formatting_check_and_fix_are_explicit(self):
        self.write_file("apps/anchor/internal/flows/first.go", "package flows\n// Changed.\n")
        result = self.run_lint("backend", "--format-changed")
        self.assertEqual(result.returncode, 0)
        self.assertEqual(self.targets(), ["./internal/flows/first.go"])
        self.assertIn("action: format\n", result.stdout)
        self.assertFalse((self.backend / "fixed").exists())
        result = self.run_lint("backend", "--format-changed", "--fix")
        self.assertEqual(result.returncode, 0)
        self.assertNotIn("--diff", self.calls()[-1]["args"])
        self.assertTrue((self.backend / "fixed").exists())

    def test_formatting_diff_fails_even_when_tool_exit_is_zero(self):
        self.write_file("apps/anchor/unformatted.go")
        result = self.run_lint("backend", "--format-changed", LINT_TEST_OUTPUT="--- before\n+++ after\n@@ formatting diff")
        self.assertEqual(result.returncode, 1)
        self.assertIn("reason: findings\n", result.stdout)

    def test_formatting_analysis_failure_preserves_exit_status(self):
        self.write_file("apps/anchor/broken.go")
        result = self.run_lint("backend", "--format-changed", LINT_TEST_EXIT="7", LINT_TEST_OUTPUT="parse failed")
        self.assertEqual(result.returncode, 7)
        self.assertIn("reason: analysis_error\n", result.stdout)

    def test_full_format_fallback_is_still_formatting_only(self):
        result = self.run_lint("backend", "--format-changed", "--base", "missing-reference")
        self.assertEqual(result.returncode, 0)
        self.assertEqual(self.targets(), ["./..."])
        self.assertIn("scope: full\n", result.stdout)
        self.assertIn("coverage: iteration_only\n", result.stdout)

    def test_termination_stops_the_owned_linter(self):
        process = subprocess.Popen(
            ["/bin/bash", str(self.root / "scripts/lint.sh"), "backend"],
            cwd=self.temp.name,
            env={**self.env, "LINT_TEST_SLEEP": "30"},
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        try:
            deadline = time.monotonic() + 5
            while not any(call["args"][0] == "run" for call in self.calls()):
                if time.monotonic() >= deadline:
                    self.fail("linter did not start")
                time.sleep(0.02)
            process.send_signal(signal.SIGTERM)
            stdout, stderr = process.communicate(timeout=5)
            self.assertEqual(process.returncode, 143, stdout + stderr)
            self.assertIn("reason: interrupted", stdout)
        finally:
            if process.poll() is None:
                process.kill()
                process.communicate()


if __name__ == "__main__":
    unittest.main()
