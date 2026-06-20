import hashlib
import os
import pathlib
import shutil
import stat
import subprocess
import tempfile
import textwrap
import unittest


SCRIPT_PATH = pathlib.Path(__file__).resolve().parents[1] / "deploy.sh"


def write_executable(path: pathlib.Path, content: str) -> None:
    path.write_text(textwrap.dedent(content).lstrip(), encoding="utf-8")
    path.chmod(path.stat().st_mode | stat.S_IXUSR)


def sha256_file(path: pathlib.Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def hash_manifest(repo_root: pathlib.Path, files: list[pathlib.Path]) -> str:
    payload = bytearray()
    for path in files:
        rel = path.relative_to(repo_root)
        payload.extend(f"path={rel.as_posix()}\n".encode("utf-8"))
        payload.extend(f"{sha256_file(path)}  {path}\n".encode("utf-8"))
    return hashlib.sha256(payload).hexdigest()


def collect_source_files(repo_root: pathlib.Path) -> list[pathlib.Path]:
    excluded_parts = {
        ("backend", "internal", "web", "dist"),
        ("backend", "data"),
        ("backend", ".gocache"),
        ("frontend", "node_modules"),
        ("frontend", "dist"),
        ("frontend", "coverage"),
    }
    files: list[pathlib.Path] = []
    for base in (repo_root / "backend", repo_root / "frontend"):
        for path in base.rglob("*"):
            if not path.is_file():
                continue
            rel_parts = path.relative_to(repo_root).parts
            if any(rel_parts[: len(parts)] == parts for parts in excluded_parts):
                continue
            if path.name.endswith(".tsbuildinfo") or path.name.startswith("vite.config.js.timestamp-"):
                continue
            files.append(path)
    return sorted(files, key=lambda item: str(item))


def collect_frontend_input_files(repo_root: pathlib.Path) -> list[pathlib.Path]:
    files: list[pathlib.Path] = []
    for path in (repo_root / "frontend").rglob("*"):
        if not path.is_file():
            continue
        rel_parts = path.relative_to(repo_root).parts
        if rel_parts[:2] in {
            ("frontend", "node_modules"),
            ("frontend", "dist"),
            ("frontend", "coverage"),
        }:
            continue
        if path.name.endswith(".tsbuildinfo"):
            continue
        files.append(path)
    return sorted(files, key=lambda item: str(item))


def collect_dist_files(repo_root: pathlib.Path) -> list[pathlib.Path]:
    dist_dir = repo_root / "backend" / "internal" / "web" / "dist"
    return sorted(
        (path for path in dist_dir.rglob("*") if path.is_file() and path.name != ".keep"),
        key=lambda item: str(item),
    )


def version_line(
    *,
    version: str,
    commit: str,
    source_hash: str,
    frontend_dist_hash: str,
    built: str = "2026-06-09T00:00:00Z",
) -> str:
    return (
        f"Sub2API version={version} commit={commit} built={built} build_type=source "
        f"dirty=clean source_hash={source_hash} frontend_dist_hash={frontend_dist_hash}"
    )


class DeployScriptTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.repo_root = pathlib.Path(self.temp_dir.name)
        self.log_path = self.repo_root / "deploy.log"
        self.mock_bin = self.repo_root / "mock-bin"
        self.mock_bin.mkdir()
        self.create_repo()
        self.create_mocks()

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def create_repo(self) -> None:
        (self.repo_root / "backend" / "cmd" / "server").mkdir(parents=True)
        (self.repo_root / "backend" / "cmd" / "server" / "VERSION").write_text("0.1.test\n", encoding="utf-8")
        (self.repo_root / "backend" / "cmd" / "sync_checksums").mkdir(parents=True)
        (self.repo_root / "backend" / "cmd" / "sync_checksums" / "main.go").write_text(
            "package main\n",
            encoding="utf-8",
        )
        (self.repo_root / "backend" / "internal" / "web" / "dist").mkdir(parents=True)
        (self.repo_root / "backend" / "internal" / "web" / "dist" / "index.html").write_text(
            "<html>cached</html>\n",
            encoding="utf-8",
        )
        (self.repo_root / "frontend" / "src").mkdir(parents=True)
        (self.repo_root / "frontend" / "package.json").write_text('{"scripts":{"build":"vite build"}}\n', encoding="utf-8")
        (self.repo_root / "frontend" / "pnpm-lock.yaml").write_text("lockfileVersion: '9.0'\n", encoding="utf-8")
        (self.repo_root / "frontend" / "src" / "main.ts").write_text("console.log('hello')\n", encoding="utf-8")
        (self.repo_root / "data").mkdir()
        (self.repo_root / "data" / "config.yaml").write_text(
            textwrap.dedent(
                """
                database:
                  host: 127.0.0.1
                  port: 5432
                  user: sub2api
                  password: test-password
                  dbname: sub2api
                  sslmode: disable
                """
            ).lstrip(),
            encoding="utf-8",
        )
        (self.repo_root / "deploy" / ".cache").mkdir(parents=True)
        subprocess.run(["git", "init"], cwd=self.repo_root, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "add", "backend", "frontend", "data"], cwd=self.repo_root, check=True)
        subprocess.run(
            ["git", "-c", "user.email=test@example.invalid", "-c", "user.name=Test", "commit", "-m", "init"],
            cwd=self.repo_root,
            check=True,
            stdout=subprocess.DEVNULL,
        )
        (self.repo_root / "deploy" / ".cache" / "deploy-state.env").write_text("", encoding="utf-8")

    def create_mocks(self) -> None:
        write_executable(
            self.mock_bin / "pnpm",
            r"""
            #!/bin/sh
            log_file="${DEPLOY_TEST_LOG:?}"
            dir=""
            if [ "$1" = "--dir" ]; then
                dir="$2"
                shift 2
            fi
            if [ "$1" = "install" ]; then
                echo "pnpm install" >> "$log_file"
                exit 0
            fi
            if [ "$1" = "run" ] && [ "$2" = "build" ]; then
                echo "pnpm build" >> "$log_file"
                mkdir -p "$dir/../backend/internal/web/dist"
                printf '<html>rebuilt</html>\n' > "$dir/../backend/internal/web/dist/index.html"
                exit 0
            fi
            echo "unexpected pnpm $*" >> "$log_file"
            exit 1
            """,
        )
        write_executable(
            self.mock_bin / "go",
            r"""
            #!/bin/sh
            log_file="${DEPLOY_TEST_LOG:?}"
            if [ "$1" = "clean" ]; then
                echo "go clean $*" >> "$log_file"
                exit 0
            fi
            if [ "$1" = "run" ]; then
                shift
                echo "go run $*" >> "$log_file"
                backup_file=""
                restore_file=""
                while [ "$#" -gt 0 ]; do
                    if [ "$1" = "--backup-file" ]; then
                        backup_file="$2"
                        break
                    fi
                    if [ "$1" = "--restore-file" ]; then
                        restore_file="$2"
                        break
                    fi
                    shift
                done
                if [ -n "$restore_file" ] && [ "${DEPLOY_TEST_RESTORE_FAIL:-0}" = "1" ]; then
                    exit 1
                fi
                if [ -n "$restore_file" ]; then
                    echo "restored" > "$log_file.checksum_restored"
                fi
                if [ -n "$backup_file" ]; then
                    mkdir -p "$(dirname "$backup_file")"
                    printf '[{"Filename":"001_test.sql","Checksum":"old"}]\n' > "$backup_file"
                fi
                exit 0
            fi
            if [ "$1" = "build" ]; then
                shift
                echo "go build $*" >> "$log_file"
                out=""
                ldflags=""
                while [ "$#" -gt 0 ]; do
                    case "$1" in
                        -o)
                            out="$2"
                            shift 2
                            ;;
                        -ldflags=*)
                            ldflags="${1#-ldflags=}"
                            shift
                            ;;
                        -ldflags)
                            ldflags="$2"
                            shift 2
                            ;;
                        *)
                            shift
                            ;;
                    esac
                done
                extract() {
                    printf '%s\n' "$ldflags" | sed -n "s/.*-X main.$1=\([^ ]*\).*/\1/p"
                }
                version="$(extract Version)"
                commit="$(extract Commit)"
                built="$(extract Date)"
                build_type="$(extract BuildType)"
                dirty="$(extract Dirty)"
                source_hash="$(extract SourceHash)"
                frontend_dist_hash="$(extract FrontendDistHash)"
                cat > "$out" <<EOF
            #!/bin/sh
            if [ "\$1" = "-version" ]; then
                echo "Sub2API version=$version commit=$commit built=$built build_type=$build_type dirty=$dirty source_hash=$source_hash frontend_dist_hash=$frontend_dist_hash"
                exit 0
            fi
            exit 0
            EOF
                chmod +x "$out"
                exit 0
            fi
            echo "unexpected go $*" >> "$log_file"
            exit 1
            """,
        )
        write_executable(
            self.mock_bin / "systemctl",
            r"""
            #!/bin/sh
            log_file="${DEPLOY_TEST_LOG:?}"
            state_file="$log_file.service_state"
            active_state="${DEPLOY_TEST_SERVICE_ACTIVE:-active}"
            if [ -f "$state_file" ]; then
                active_state="$(cat "$state_file")"
            fi
            exec_main_pid="${DEPLOY_TEST_EXEC_MAIN_PID:-0}"
            if [ "$1" = "show" ] && [ "$3" = "--property=ExecMainPID" ]; then
                echo "systemctl show $2 --property=ExecMainPID --value" >> "$log_file"
                echo "$exec_main_pid"
                exit 0
            fi
            if [ "$1" = "show" ] && [ "$3" = "--property=ActiveState" ]; then
                echo "systemctl show $2 --property=ActiveState --value" >> "$log_file"
                echo "$active_state"
                exit 0
            fi
            if [ "$1" = "show" ]; then
                echo "systemctl show $2" >> "$log_file"
                echo "ExecMainPID=$exec_main_pid"
                echo "ActiveState=$active_state"
                echo "SubState=running"
                echo "WorkingDirectory=${REPO_ROOT}"
                exit 0
            fi
            if [ "$1" = "is-active" ]; then
                if [ "${DEPLOY_TEST_IS_ACTIVE_FAIL_UNTIL_RESTORE:-0}" = "1" ] && [ ! -f "$log_file.checksum_restored" ]; then
                    echo "systemctl is-active $3" >> "$log_file"
                    exit 1
                fi
                exit 0
            fi
            if [ "$1" = "status" ]; then
                echo "mock status"
                exit 0
            fi
            if [ "$1" = "restart" ]; then
                echo "systemctl restart $2" >> "$log_file"
                if [ "${DEPLOY_TEST_RESTART_FAIL_ONCE:-0}" = "1" ]; then
                    restart_count_file="$log_file.restart_count"
                    restart_count=0
                    if [ -f "$restart_count_file" ]; then
                        restart_count="$(cat "$restart_count_file")"
                    fi
                    restart_count=$((restart_count + 1))
                    echo "$restart_count" > "$restart_count_file"
                    if [ "$restart_count" -eq 1 ]; then
                        exit 1
                    fi
                fi
                if [ "${DEPLOY_TEST_RESTART_FAIL:-0}" = "1" ]; then
                    exit 1
                fi
                echo "active" > "$state_file"
                exit 0
            fi
            if [ "$1" = "stop" ] || [ "$1" = "start" ]; then
                echo "systemctl $1 $2" >> "$log_file"
                exit 0
            fi
            echo "unexpected systemctl $*" >> "$log_file"
            exit 1
            """,
        )
        write_executable(
            self.mock_bin / "journalctl",
            r"""
            #!/bin/sh
            echo "mock journalctl" >> "${DEPLOY_TEST_LOG:?}"
            exit 0
            """,
        )
        write_executable(
            self.mock_bin / "pgrep",
            r"""
            #!/bin/sh
            exit 1
            """,
        )

    def run_deploy(self, extra_env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
        env = os.environ.copy()
        env["PATH"] = f"{self.mock_bin}:{env['PATH']}"
        env["REPO_ROOT"] = str(self.repo_root)
        env["SERVICE_NAME"] = "sub2api-test"
        env["DEPLOY_TEST_LOG"] = str(self.log_path)
        env["DEPLOY_START_WAIT_SECONDS"] = "0"
        if extra_env:
            env.update(extra_env)
        return subprocess.run(
            ["bash", str(SCRIPT_PATH)],
            cwd=self.repo_root,
            env=env,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=False,
        )

    def prepare_current_binary_and_frontend_cache(self) -> None:
        commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=self.repo_root, text=True).strip()
        frontend_input_hash = hash_manifest(self.repo_root, collect_frontend_input_files(self.repo_root))
        source_hash = hash_manifest(self.repo_root, collect_source_files(self.repo_root))
        frontend_dist_hash = hash_manifest(self.repo_root, collect_dist_files(self.repo_root))
        line = version_line(
            version="0.1.test",
            commit=commit,
            source_hash=source_hash,
            frontend_dist_hash=frontend_dist_hash,
        )
        write_executable(
            self.repo_root / "sub2api",
            f"""
            #!/bin/sh
            if [ "$1" = "-version" ]; then
                echo "{line}"
                exit 0
            fi
            exit 0
            """,
        )
        (self.repo_root / "deploy" / ".cache" / "deploy-state.env").write_text(
            f"FRONTEND_INPUT_HASH={frontend_input_hash}\n"
            f"FRONTEND_DIST_HASH={frontend_dist_hash}\n",
            encoding="utf-8",
        )

    def test_no_change_deploy_skips_frontend_backend_and_restart(self) -> None:
        self.prepare_current_binary_and_frontend_cache()

        result = self.run_deploy()

        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        log = self.log_path.read_text(encoding="utf-8") if self.log_path.exists() else ""
        self.assertNotIn("pnpm install", log)
        self.assertNotIn("pnpm build", log)
        self.assertNotIn("go build", log)
        self.assertNotIn("go run", log)
        self.assertNotIn("go clean", log)
        self.assertNotIn("systemctl stop", log)
        self.assertNotIn("systemctl start", log)
        self.assertNotIn("systemctl restart", log)
        self.assertIn("无需部署", result.stdout)

    def test_no_change_deploy_starts_service_when_binary_current_but_service_inactive(self) -> None:
        self.prepare_current_binary_and_frontend_cache()

        result = self.run_deploy({"DEPLOY_TEST_SERVICE_ACTIVE": "inactive"})

        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        log = self.log_path.read_text(encoding="utf-8") if self.log_path.exists() else ""
        self.assertNotIn("pnpm install", log)
        self.assertNotIn("pnpm build", log)
        self.assertNotIn("go build", log)
        self.assertIn("go run ./cmd/sync_checksums --backup-file", log)
        self.assertIn("systemctl restart sub2api-test", log)
        self.assertNotIn("systemctl stop", log)
        self.assertNotIn("systemctl start", log)
        self.assertNotIn("无需部署", result.stdout)

    def test_generated_frontend_tsbuildinfo_does_not_invalidate_no_change_deploy(self) -> None:
        self.prepare_current_binary_and_frontend_cache()
        (self.repo_root / "frontend" / "tsconfig.tsbuildinfo").write_text(
            '{"generated":true}\n',
            encoding="utf-8",
        )

        result = self.run_deploy()

        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        log = self.log_path.read_text(encoding="utf-8") if self.log_path.exists() else ""
        self.assertNotIn("pnpm install", log)
        self.assertNotIn("pnpm build", log)
        self.assertNotIn("go build", log)
        self.assertNotIn("systemctl stop", log)
        self.assertNotIn("systemctl restart", log)
        self.assertIn("无需部署", result.stdout)

    def test_changed_deploy_builds_before_stop_and_does_not_clean_go_cache(self) -> None:
        result = self.run_deploy()

        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        log_lines = self.log_path.read_text(encoding="utf-8").splitlines()
        self.assertIn("pnpm install", log_lines)
        self.assertIn("pnpm build", log_lines)
        self.assertTrue(any(line.startswith("go build ") for line in log_lines), log_lines)
        sync_index = next(
            i for i, line in enumerate(log_lines) if line.startswith("go run ./cmd/sync_checksums --backup-file ")
        )
        self.assertNotIn("go clean clean -cache", log_lines)
        restart_index = log_lines.index("systemctl restart sub2api-test")
        self.assertLess(log_lines.index("pnpm build"), restart_index)
        self.assertLess(next(i for i, line in enumerate(log_lines) if line.startswith("go build ")), restart_index)
        self.assertLess(sync_index, restart_index)
        self.assertEqual(log_lines.count("systemctl restart sub2api-test"), 1)
        self.assertIn("systemctl show sub2api-test --property=ActiveState --value", log_lines)
        self.assertIn("systemctl show sub2api-test --property=ExecMainPID --value", log_lines)
        self.assertFalse(any(line.startswith("systemctl stop ") for line in log_lines), log_lines)
        self.assertFalse(any(line.startswith("systemctl start ") for line in log_lines), log_lines)
        self.assertTrue((self.repo_root / "sub2api").is_file())
        self.assertFalse(list(self.repo_root.glob("sub2api.new*")))

    def test_restart_failure_rolls_back_binary_and_checksum_snapshot(self) -> None:
        old_binary = self.repo_root / "sub2api"
        write_executable(
            old_binary,
            """
            #!/bin/sh
            if [ "$1" = "-version" ]; then
                echo "Sub2API version=old commit=old built=old build_type=source dirty=clean source_hash=old frontend_dist_hash=old"
                exit 0
            fi
            exit 0
            """,
        )

        result = self.run_deploy({"DEPLOY_TEST_RESTART_FAIL": "1"})

        self.assertNotEqual(result.returncode, 0)
        log_lines = self.log_path.read_text(encoding="utf-8").splitlines()
        self.assertEqual(log_lines.count("systemctl restart sub2api-test"), 3, log_lines)
        self.assertTrue(any(line.startswith("go run ./cmd/sync_checksums --backup-file ") for line in log_lines), log_lines)
        self.assertTrue(any(line.startswith("go run ./cmd/sync_checksums --restore-file ") for line in log_lines), log_lines)
        self.assertIn("version=old commit=old", old_binary.read_text(encoding="utf-8"))

    def test_restart_failure_restarts_old_binary_before_checksum_restore(self) -> None:
        old_binary = self.repo_root / "sub2api"
        write_executable(
            old_binary,
            """
            #!/bin/sh
            if [ "$1" = "-version" ]; then
                echo "Sub2API version=old commit=old built=old build_type=source dirty=clean source_hash=old frontend_dist_hash=old"
                exit 0
            fi
            exit 0
            """,
        )

        result = self.run_deploy({"DEPLOY_TEST_RESTART_FAIL_ONCE": "1"})

        self.assertNotEqual(result.returncode, 0)
        log_lines = self.log_path.read_text(encoding="utf-8").splitlines()
        restart_indexes = [i for i, line in enumerate(log_lines) if line == "systemctl restart sub2api-test"]
        restore_index = next(i for i, line in enumerate(log_lines) if line.startswith("go run ./cmd/sync_checksums --restore-file "))
        self.assertEqual(len(restart_indexes), 2, log_lines)
        self.assertLess(restart_indexes[1], restore_index, log_lines)
        self.assertIn("version=old commit=old", old_binary.read_text(encoding="utf-8"))

    def test_rollback_restarts_again_after_checksum_restore_if_old_binary_not_active(self) -> None:
        old_binary = self.repo_root / "sub2api"
        write_executable(
            old_binary,
            """
            #!/bin/sh
            if [ "$1" = "-version" ]; then
                echo "Sub2API version=old commit=old built=old build_type=source dirty=clean source_hash=old frontend_dist_hash=old"
                exit 0
            fi
            exit 0
            """,
        )

        result = self.run_deploy({
            "DEPLOY_TEST_IS_ACTIVE_FAIL_UNTIL_RESTORE": "1",
            "DEPLOY_START_WAIT_ATTEMPTS": "1",
            "DEPLOY_START_WAIT_SECONDS": "0",
            "DEPLOY_ACTIVE_STABLE_CHECKS": "1",
        })

        self.assertNotEqual(result.returncode, 0)
        log_lines = self.log_path.read_text(encoding="utf-8").splitlines()
        restart_indexes = [i for i, line in enumerate(log_lines) if line == "systemctl restart sub2api-test"]
        restore_index = next(i for i, line in enumerate(log_lines) if line.startswith("go run ./cmd/sync_checksums --restore-file "))
        self.assertEqual(len(restart_indexes), 3, log_lines)
        self.assertLess(restart_indexes[1], restore_index, log_lines)
        self.assertGreater(restart_indexes[2], restore_index, log_lines)
        self.assertIn("version=old commit=old", old_binary.read_text(encoding="utf-8"))

    def test_restart_failure_still_restarts_old_binary_when_checksum_restore_fails(self) -> None:
        old_binary = self.repo_root / "sub2api"
        write_executable(
            old_binary,
            """
            #!/bin/sh
            if [ "$1" = "-version" ]; then
                echo "Sub2API version=old commit=old built=old build_type=source dirty=clean source_hash=old frontend_dist_hash=old"
                exit 0
            fi
            exit 0
            """,
        )

        result = self.run_deploy({
            "DEPLOY_TEST_RESTART_FAIL": "1",
            "DEPLOY_TEST_RESTORE_FAIL": "1",
        })

        self.assertNotEqual(result.returncode, 0)
        log_lines = self.log_path.read_text(encoding="utf-8").splitlines()
        self.assertEqual(log_lines.count("systemctl restart sub2api-test"), 3, log_lines)
        self.assertTrue(any(line.startswith("go run ./cmd/sync_checksums --restore-file ") for line in log_lines), log_lines)
        self.assertIn("version=old commit=old", old_binary.read_text(encoding="utf-8"))

    def test_restart_failure_without_backend_build_restores_checksum_snapshot(self) -> None:
        self.prepare_current_binary_and_frontend_cache()

        result = self.run_deploy({
            "DEPLOY_FORCE_RESTART": "1",
            "DEPLOY_TEST_RESTART_FAIL": "1",
        })

        self.assertNotEqual(result.returncode, 0)
        log_lines = self.log_path.read_text(encoding="utf-8").splitlines()
        self.assertNotIn("go build", "\n".join(log_lines))
        self.assertEqual(log_lines.count("systemctl restart sub2api-test"), 3, log_lines)
        self.assertTrue(any(line.startswith("go run ./cmd/sync_checksums --backup-file ") for line in log_lines), log_lines)
        self.assertTrue(any(line.startswith("go run ./cmd/sync_checksums --restore-file ") for line in log_lines), log_lines)


if __name__ == "__main__":
    unittest.main()
