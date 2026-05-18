#!/usr/bin/env python3
"""Atlas local helper commands."""

import argparse
import json
import os
import shutil
import subprocess
import sys
import time
from pathlib import Path

PROJECT_DIR = Path(__file__).resolve().parent / "atlas-core"
PROTOCOL_DIR = Path(__file__).resolve().parent / "atlas-protocol"
REPO_DIR = Path(__file__).resolve().parent
PROTO_FILES = [
    "proto/atlas/shared/v1/common.proto",
    "proto/atlas/datastorage/v1/datastorage.proto",
    "proto/atlas/functions/v1/functions.proto",
]
GENERATED_DIR = "atlas-core/services/shared/gen"
PROTO_PLUGIN_DIR = REPO_DIR / ".atlas-tools" / "proto-bin"
BOUNDARY_DOC_REQUIREMENTS = {
    REPO_DIR / "README.md": [
        "atlas-functions is the only supported public API",
        "External clients must never call atlas-datastorage directly",
    ],
    REPO_DIR / "AGENTS.md": [
        "atlas-functions is the only supported public API",
        "External clients must never call atlas-datastorage directly",
    ],
    REPO_DIR / "docs" / "atlas-core" / "design-decisions" / "0002-service-boundaries-grpc-changefeed.md": [
        "atlas-functions is the only supported public API",
        "External clients must never call atlas-datastorage directly",
    ],
}


def show_menu():
    print()
    print("=" * 50)
    print("  Atlas Core")
    print("=" * 50)
    print("  1. Start system")
    print("  2. Stop / Reset system")
    print("  3. Restart system")
    print("  0. Exit")
    print("=" * 50)
    print()


def run_compose(*args, capture_output=False, text=False):
    cmd = ["docker", "compose", *args]
    return subprocess.run(cmd, cwd=PROJECT_DIR, capture_output=capture_output, text=text)


def run_compose_with_env(*args, env=None, capture_output=False, text=False):
    cmd = ["docker", "compose", *args]
    return subprocess.run(cmd, cwd=PROJECT_DIR, env=env, capture_output=capture_output, text=text)


def run_protocol(*args):
    return subprocess.run(list(args), cwd=PROTOCOL_DIR)


def run_codebase(*args, cwd=REPO_DIR, env=None, capture_output=False, text=False):
    return subprocess.run(list(args), cwd=cwd, env=env, capture_output=capture_output, text=text)


def container_id(service):
    result = run_compose("ps", "-q", service, capture_output=True, text=True)
    if result.returncode != 0:
        return None
    container = result.stdout.strip()
    return container or None


def container_health(service):
    cid = container_id(service)
    if not cid:
        return None
    result = subprocess.run(
        [
            "docker",
            "inspect",
            "--format",
            "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}",
            cid,
        ],
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        return None
    return result.stdout.strip()


def wait_for_health(service, timeout=60):
    deadline = time.time() + timeout
    while time.time() < deadline:
        status = container_health(service)
        if status == "healthy":
            return True
        if status in {"exited", "dead", "unhealthy"}:
            return False
        time.sleep(1)
    return False


def codegen(check=False):
    if shutil.which("protoc") is None:
        print("[atlas] Error: protoc not found in PATH; install protobuf-compiler first.", file=sys.stderr)
        return False

    PROTO_PLUGIN_DIR.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    env["GOBIN"] = str(PROTO_PLUGIN_DIR)
    env["PATH"] = str(PROTO_PLUGIN_DIR) + os.pathsep + env.get("PATH", "")

    for tool in (
        "google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10",
        "google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1",
    ):
        install = run_codebase("go", "install", tool, cwd=PROJECT_DIR, env=env)
        if install.returncode != 0:
            print(f"[atlas] Failed to install {tool}", file=sys.stderr)
            return False

    cmd = [
        "protoc",
        "-I",
        "proto",
        "--go_out=paths=source_relative:services/shared/gen",
        "--go-grpc_out=paths=source_relative:services/shared/gen",
        *PROTO_FILES,
    ]
    print("[atlas] Running Atlas Core gRPC codegen...")
    result = run_codebase(*cmd, cwd=PROJECT_DIR, env=env)
    if result.returncode != 0:
        print("[atlas] Atlas Core gRPC codegen failed", file=sys.stderr)
        return False

    if check:
        status = run_codebase(
            "git",
            "status",
            "--porcelain",
            "--",
            GENERATED_DIR,
            cwd=REPO_DIR,
            capture_output=True,
            text=True,
        )
        if status.returncode != 0:
            print("[atlas] Failed to inspect generated code status", file=sys.stderr)
            return False
        if status.stdout.strip():
            print("[atlas] Generated gRPC code is out of date; run `python3 atlas.py codegen`.", file=sys.stderr)
            return False

    print("[atlas] Atlas Core gRPC codegen passed.")
    return True


def start():
    if not codegen():
        return False
    print("[atlas] Starting system...")
    result = run_compose("up", "--build", "-d")
    if result.returncode != 0:
        print("[atlas] Failed to start", file=sys.stderr)
        return False
    print("[atlas] Waiting for atlas-functions healthcheck...")
    if not wait_for_health("atlas-functions"):
        print("[atlas] atlas-functions did not become healthy", file=sys.stderr)
        return False
    logs = run_compose("logs", "--tail=10", "atlas-datastorage", "atlas-functions", capture_output=True, text=True)
    if logs.returncode != 0:
        print("[atlas] Warning: failed to read Atlas Core logs", file=sys.stderr)
        if logs.stderr:
            print(logs.stderr, file=sys.stderr)
    else:
        print("[atlas] Atlas Core logs:")
        print(logs.stdout)
    print("[atlas] System started.")
    return True


def stop():
    print("[atlas] Stopping system...")
    result = run_compose("down", "--remove-orphans")
    if result.returncode != 0:
        print("[atlas] Failed to stop system", file=sys.stderr)
        return False
    print("[atlas] System stopped.")
    return True


def stop_reset(force=False):
    if not force:
        answer = input("[atlas] This will delete the database and object storage. Continue? [y/N] ").strip().lower()
        if answer not in {"y", "yes"}:
            print("[atlas] Reset cancelled.")
            return True
    print("[atlas] Stopping and resetting system...")
    result = run_compose("down", "-v", "--remove-orphans")
    if result.returncode != 0:
        print("[atlas] Failed to stop/reset system", file=sys.stderr)
        return False
    print("[atlas] System stopped and reset.")
    return True


def restart(force=False):
    if not stop_reset(force=force):
        return False
    print("[atlas] Waiting...")
    time.sleep(2)
    return start()


def protocol_check():
    if not (PROTOCOL_DIR / "package.json").exists():
        print(f"[atlas] Error: package.json not found in {PROTOCOL_DIR}", file=sys.stderr)
        return False

    if not (PROTOCOL_DIR / "node_modules").exists():
        print("[atlas] Installing Atlas Protocol dependencies...")
        install = run_protocol("npm", "ci")
        if install.returncode != 0:
            print("[atlas] Failed to install Atlas Protocol dependencies", file=sys.stderr)
            return False

    print("[atlas] Running Atlas Protocol verification...")
    verify = run_protocol("npm", "run", "verify")
    if verify.returncode != 0:
        print("[atlas] Atlas Protocol verification failed", file=sys.stderr)
        return False

    print("[atlas] Atlas Protocol verification passed.")
    return True


def protocol_validate(forward_argv):
    if not (PROTOCOL_DIR / "package.json").exists():
        print(f"[atlas] Error: package.json not found in {PROTOCOL_DIR}", file=sys.stderr)
        return False

    if not (PROTOCOL_DIR / "node_modules").exists():
        print("[atlas] Installing Atlas Protocol dependencies...")
        install = run_protocol("npm", "ci")
        if install.returncode != 0:
            print("[atlas] Failed to install Atlas Protocol dependencies", file=sys.stderr)
            return False

    cmd = ["npm", "run", "validate", "--", *forward_argv]
    print("[atlas] Running Atlas Protocol validate:", " ".join(cmd))
    result = subprocess.run(cmd, cwd=PROTOCOL_DIR)
    if result.returncode != 0:
        print("[atlas] Atlas Protocol validate failed", file=sys.stderr)
        return False

    print("[atlas] Atlas Protocol validate passed.")
    return True


def architecture_check():
    env = os.environ.copy()
    env["ATLAS_DATASTORAGE_INTERNAL_TOKEN"] = "architecture-check-token"
    env["ATLAS_FUNCTIONS_HOST_PORT"] = "8080"
    result = run_compose_with_env(
        "-f",
        "docker-compose.yml",
        "config",
        "--format",
        "json",
        env=env,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        print("[atlas] Failed to render Docker Compose config", file=sys.stderr)
        if result.stderr:
            print(result.stderr, file=sys.stderr)
        return False

    try:
        compose = json.loads(result.stdout)
    except json.JSONDecodeError as err:
        print(f"[atlas] Failed to parse Docker Compose config JSON: {err}", file=sys.stderr)
        return False

    services = compose.get("services", {})
    if service_ports(services, "atlas-datastorage"):
        print("[atlas] atlas-datastorage must not publish ports in docker-compose.yml", file=sys.stderr)
        return False
    if service_ports(services, "postgres"):
        print("[atlas] postgres must not publish ports in docker-compose.yml", file=sys.stderr)
        return False
    if not functions_port_is_loopback_only(service_ports(services, "atlas-functions")):
        print("[atlas] atlas-functions must publish only 127.0.0.1:8080->8080 in docker-compose.yml", file=sys.stderr)
        return False

    for path, required_phrases in BOUNDARY_DOC_REQUIREMENTS.items():
        try:
            content = path.read_text()
        except OSError as err:
            print(f"[atlas] Failed to read {path.relative_to(REPO_DIR)}: {err}", file=sys.stderr)
            return False
        normalized_content = " ".join(content.replace("`", "").split())
        for phrase in required_phrases:
            if phrase not in normalized_content:
                print(f"[atlas] Missing boundary phrase in {path.relative_to(REPO_DIR)}: {phrase}", file=sys.stderr)
                return False

    print("[atlas] Atlas Core architecture check passed.")
    return True


def service_ports(services, service):
    return services.get(service, {}).get("ports") or []


def functions_port_is_loopback_only(ports):
    if len(ports) != 1:
        return False
    port = ports[0]
    return (
        port.get("host_ip") == "127.0.0.1"
        and str(port.get("published")) == "8080"
        and int(port.get("target", 0)) == 8080
    )


def parse_args(argv=None):
    parser = argparse.ArgumentParser(description="Atlas local helper commands")
    parser.add_argument(
        "--force",
        action="store_true",
        dest="menu_force",
        help="Skip confirmation for destructive operations",
    )

    subparsers = parser.add_subparsers(dest="command")
    subparsers.add_parser("menu", help="Open the interactive menu")
    subparsers.add_parser("start", help="Start Atlas Core and wait for health")
    subparsers.add_parser("stop", help="Stop Atlas Core without deleting volumes")
    subparsers.add_parser("protocol-check", help="Run local Atlas Protocol verification")
    subparsers.add_parser("architecture-check", help="Verify Atlas Core service boundary invariants")
    subparsers.add_parser("codegen", help="Generate Atlas Core gRPC code")
    subparsers.add_parser("codegen-check", help="Verify Atlas Core gRPC code is up to date")
    protocol_validate_parser = subparsers.add_parser(
        "protocol-validate",
        help="Validate a JSON file (forwards args to npm run validate)",
    )

    reset_parser = subparsers.add_parser("reset", help="Stop Atlas Core and delete Docker Compose volumes")
    reset_parser.add_argument("--force", action="store_true", help="Skip reset confirmation")

    restart_parser = subparsers.add_parser("restart", help="Reset, rebuild, and start Atlas Core")
    restart_parser.add_argument("--force", action="store_true", help="Skip reset confirmation")

    args, extra_argv = parser.parse_known_args(argv)
    if args.command == "protocol-validate":
        args.forward_argv = extra_argv
    elif extra_argv:
        parser.error(f"unrecognized arguments: {' '.join(extra_argv)}")

    return args


def run_command(args):
    if args.command == "start":
        return start()
    if args.command == "stop":
        return stop()
    if args.command == "protocol-check":
        return protocol_check()
    if args.command == "architecture-check":
        return architecture_check()
    if args.command == "codegen":
        return codegen()
    if args.command == "codegen-check":
        return codegen(check=True)
    if args.command == "protocol-validate":
        forward_argv = list(args.forward_argv)
        if forward_argv[:1] == ["--"]:
            forward_argv = forward_argv[1:]
        return protocol_validate(forward_argv)
    if args.command == "reset":
        return stop_reset(force=args.force or args.menu_force)
    if args.command == "restart":
        return restart(force=args.force or args.menu_force)
    return None


def exit_from_success(result):
    sys.exit(0 if result else 1)


def command_requires_atlas_core(command):
    return command not in {"protocol-check", "protocol-validate", "codegen", "codegen-check", "architecture-check"}


def main():
    args = parse_args()

    if command_requires_atlas_core(args.command) and not (PROJECT_DIR / "docker-compose.yml").exists():
        print(f"[atlas] Error: docker-compose.yml not found in {PROJECT_DIR}", file=sys.stderr)
        sys.exit(1)

    command_result = run_command(args)
    if command_result is not None:
        exit_from_success(command_result)

    while True:
        show_menu()
        try:
            choice = input("  Choice: ").strip()
        except (EOFError, KeyboardInterrupt):
            print()
            break

        if choice == "1":
            if not start():
                sys.exit(1)
        elif choice == "2":
            if not stop_reset(force=args.menu_force):
                sys.exit(1)
        elif choice == "3":
            if not restart(force=args.menu_force):
                sys.exit(1)
        elif choice == "0":
            print("[atlas] Goodbye.")
            break
        else:
            print("[atlas] Invalid choice. Enter 1, 2, 3, or 0.")


if __name__ == "__main__":
    main()
