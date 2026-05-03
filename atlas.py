#!/usr/bin/env python3
"""Atlas Core startup/reset tool."""

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


def run_compose(*args):
    cmd = ["docker", "compose"] + list(args)
    return subprocess.run(cmd, cwd=PROJECT_DIR)


def start():
    print("[atlas] Starting system...")
    result = run_compose("up", "--build", "-d")
    if result.returncode != 0:
        print("[atlas] Failed to start", file=sys.stderr)
        return False
    print("[atlas] Waiting for services to be ready...")
    time.sleep(2)
    logs = subprocess.run(
        ["docker", "compose", "logs", "--tail=10", "atlas-core"],
        cwd=PROJECT_DIR, capture_output=True, text=True
    )
    if logs.returncode != 0:
        print("[atlas] Failed to read Atlas Core logs", file=sys.stderr)
        if logs.stderr:
            print(logs.stderr, file=sys.stderr)
        return False
    print("[atlas] Atlas Core logs:")
    print(logs.stdout)
    print("[atlas] System started.")
    return True


def stop_reset():
    print("[atlas] Stopping and resetting system...")
    result = run_compose("down", "-v", "--remove-orphans")
    if result.returncode != 0:
        print("[atlas] Failed to stop/reset system", file=sys.stderr)
        return False
    print("[atlas] System stopped and reset.")
    return True


def restart():
    if not stop_reset():
        return False
    print("[atlas] Waiting...")
    time.sleep(2)
    return start()


def main():
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
            if not stop_reset():
                sys.exit(1)
        elif choice == "3":
            if not restart():
                sys.exit(1)
        elif choice == "0":
            print("[atlas] Goodbye.")
            break
        else:
            print("[atlas] Invalid choice. Enter 1, 2, 3, or 0.")


if __name__ == "__main__":
    main()
