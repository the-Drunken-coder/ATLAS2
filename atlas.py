#!/usr/bin/env python3
"""Atlas local helper commands."""

import argparse
import subprocess
import sys
import time
from pathlib import Path

PROJECT_DIR = Path(__file__).resolve().parent / "atlas-core"
PROTOCOL_DIR = Path(__file__).resolve().parent / "atlas-protocol"


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


def run_protocol(*args):
    return subprocess.run(list(args), cwd=PROTOCOL_DIR)


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


def start():
    print("[atlas] Starting system...")
    result = run_compose("up", "--build", "-d")
    if result.returncode != 0:
        print("[atlas] Failed to start", file=sys.stderr)
        return False
    print("[atlas] Waiting for atlas-core healthcheck...")
    if not wait_for_health("atlas-core"):
        print("[atlas] atlas-core did not become healthy", file=sys.stderr)
        return False
    logs = run_compose("logs", "--tail=10", "atlas-core", capture_output=True, text=True)
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


def parse_args():
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
    protocol_validate_parser = subparsers.add_parser(
        "protocol-validate",
        help="Validate a JSON file (forwards args to npm run validate after --)",
    )
    protocol_validate_parser.add_argument(
        "forward_argv",
        nargs=argparse.REMAINDER,
        help="Arguments forwarded to npm run validate",
    )

    reset_parser = subparsers.add_parser("reset", help="Stop Atlas Core and delete Docker Compose volumes")
    reset_parser.add_argument("--force", action="store_true", help="Skip reset confirmation")

    restart_parser = subparsers.add_parser("restart", help="Reset, rebuild, and start Atlas Core")
    restart_parser.add_argument("--force", action="store_true", help="Skip reset confirmation")

    return parser.parse_args()


def run_command(args):
    if args.command == "start":
        return start()
    if args.command == "stop":
        return stop()
    if args.command == "protocol-check":
        return protocol_check()
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


def main():
    args = parse_args()

    if not (PROJECT_DIR / "docker-compose.yml").exists():
        print(f"[atlas] Error: docker-compose.yml not found in {PROJECT_DIR}", file=sys.stderr)
        sys.exit(1)

    command_result = run_command(args)
    if command_result is not None:
        sys.exit(0 if command_result else 1)

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
