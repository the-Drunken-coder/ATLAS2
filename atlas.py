#!/usr/bin/env python3
"""Atlas Core startup/reset tool."""

import argparse
import subprocess
import sys
import time
from pathlib import Path

PROJECT_DIR = Path(__file__).resolve().parent / "atlas-core"


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
    cmd = ["docker", "compose"] + list(args)
    return subprocess.run(cmd, cwd=PROJECT_DIR, capture_output=capture_output, text=text)


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


def parse_args():
    parser = argparse.ArgumentParser(description="Atlas Core startup/reset tool")
    parser.add_argument("--force", action="store_true", help="Skip confirmation for destructive reset operations")
    return parser.parse_args()


def main():
    args = parse_args()
    if not (PROJECT_DIR / "docker-compose.yml").exists():
        print(f"[atlas] Error: docker-compose.yml not found in {PROJECT_DIR}", file=sys.stderr)
        sys.exit(1)

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
            if not stop_reset(force=args.force):
                sys.exit(1)
        elif choice == "3":
            if not restart(force=args.force):
                sys.exit(1)
        elif choice == "0":
            print("[atlas] Goodbye.")
            break
        else:
            print("[atlas] Invalid choice. Enter 1, 2, 3, or 0.")


if __name__ == "__main__":
    main()
