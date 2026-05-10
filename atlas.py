#!/usr/bin/env python3
"""Atlas Core startup/reset tool."""

import argparse
import json
import shutil
import subprocess
import sys
import time
from pathlib import Path

REPO_DIR = Path(__file__).resolve().parent
PROJECT_DIR = REPO_DIR / "atlas-core"
PROTOCOL_DIR = REPO_DIR / "atlas-protocol"
SYNCED_PROTOCOL_DIR = PROJECT_DIR / "protocol"
SYNCED_PROTOCOL_FILES = {
    PROTOCOL_DIR / "dist" / "atlas-protocol-validator.mjs": SYNCED_PROTOCOL_DIR / "atlas-protocol-validator.mjs",
    PROTOCOL_DIR / "generated" / "schema-bundle.json": SYNCED_PROTOCOL_DIR / "schema-bundle.json",
    PROTOCOL_DIR / "generated" / "types.ts": SYNCED_PROTOCOL_DIR / "types.ts",
    PROTOCOL_DIR / "generated" / "validators" / "index.mjs": SYNCED_PROTOCOL_DIR / "validators" / "index.mjs",
}


def synced_package_json():
    source = json.loads((PROTOCOL_DIR / "package.json").read_text(encoding="utf-8"))
    return json.dumps(
        {
            "name": source["name"],
            "version": source["version"],
            "private": True,
            "type": "module",
            "description": "Synced Atlas Protocol runtime artifacts for Atlas Core",
            "bin": {
                "atlas-protocol-validator": "./atlas-protocol-validator.mjs",
            },
            "exports": {
                "./schema-bundle": "./schema-bundle.json",
                "./types": "./types.ts",
                "./validators": "./validators/index.mjs",
            },
        },
        indent=2,
    ) + "\n"


def synced_readme():
    return """# Atlas Protocol runtime artifacts for Atlas Core

This directory is a synced runtime mirror used by Atlas Core. It does not contain the editable `atlas-protocol/` source package.

## What lives here

- `atlas-protocol-validator.mjs`: bundled validator CLI executed by Atlas Core.
- `schema-bundle.json`: synced schema bundle for runtime and tooling checks.
- `types.ts`: generated TypeScript types from Atlas Protocol schemas.
- `validators/index.mjs`: bundled standalone validator exports.

## Local workflow

Do not edit files in this directory by hand. Regenerate and sync them from the source package with:

```bash
python3 atlas.py protocol-sync
```

`python3 atlas.py start` also rebuilds and syncs these artifacts before the local stack starts.

The full editable Atlas Protocol package lives in `../atlas-protocol/`.
"""


def generated_synced_artifacts():
    return {
        SYNCED_PROTOCOL_DIR / "package.json": synced_package_json(),
        SYNCED_PROTOCOL_DIR / "README.md": synced_readme(),
    }


def verify_validator_module(module_path):
    syntax_check = run_process(["node", "--check", str(module_path)], cwd=REPO_DIR)
    if syntax_check.returncode != 0:
        return False
    import_check = run_process(
        [
            "node",
            "-e",
            f"import({module_path.resolve().as_uri()!r}).then((m) => {{ if (!m.validators?.entity) process.exit(1); }})",
        ],
        cwd=REPO_DIR,
    )
    return import_check.returncode == 0


def verify_protocol_artifacts(local_only=False):
    local_validators = PROTOCOL_DIR / "generated" / "validators" / "index.mjs"
    if not verify_validator_module(local_validators):
        print(f"[atlas] Atlas Protocol validators failed verification: {local_validators}", file=sys.stderr)
        return False
    if local_only:
        return True
    synced_validators = SYNCED_PROTOCOL_DIR / "validators" / "index.mjs"
    if not verify_validator_module(synced_validators):
        print(f"[atlas] Synced Atlas Protocol validators failed verification: {synced_validators}", file=sys.stderr)
        return False
    return True


def show_menu():
    print()
    print("=" * 50)
    print("  Atlas Core")
    print("=" * 50)
    print("  1. Start system")
    print("  2. Stop / Reset system")
    print("  3. Restart system")
    print("  4. Sync Atlas Protocol artifacts")
    print("  0. Exit")
    print("=" * 50)
    print()


def run_compose(*args, capture_output=False, text=False):
    cmd = ["docker", "compose", *args]
    return subprocess.run(cmd, cwd=PROJECT_DIR, capture_output=capture_output, text=text)


def run_process(cmd, cwd, capture_output=False, text=False):
    return subprocess.run(cmd, cwd=cwd, capture_output=capture_output, text=text)


def ensure_command(name, *alternatives):
    for candidate in (name, *alternatives):
        if shutil.which(candidate):
            return candidate
    return None


def pnpm_command():
    if ensure_command("corepack"):
        return ["corepack", "pnpm"]
    if ensure_command("pnpm"):
        return ["pnpm"]
    return None


def copy_protocol_artifacts(check_only=False):
    stale = []
    expected = {destination.resolve() for destination in SYNCED_PROTOCOL_FILES.values()}
    for source, destination in SYNCED_PROTOCOL_FILES.items():
        if not source.exists():
            stale.append(f"missing required protocol artifact: {source}")
            continue
        if not destination.exists() or source.read_bytes() != destination.read_bytes():
            if check_only:
                stale.append(f"stale protocol artifact: {destination}")
                continue
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(source, destination)
    for destination, content in generated_synced_artifacts().items():
        expected.add(destination.resolve())
        current = destination.read_text(encoding="utf-8") if destination.exists() else None
        if current != content:
            if check_only:
                stale.append(f"stale protocol artifact: {destination}")
                continue
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_text(content, encoding="utf-8")
    if check_only and SYNCED_PROTOCOL_DIR.exists():
        for existing in SYNCED_PROTOCOL_DIR.rglob("*"):
            if existing.is_file() and existing.resolve() not in expected:
                stale.append(f"unexpected synced protocol artifact: {existing}")
    if stale:
        for line in stale:
            print(f"[atlas] {line}", file=sys.stderr)
        return False
    return True


def ensure_protocol_ready(run_tests=False, check_only=False):
    if not PROTOCOL_DIR.exists():
        print(f"[atlas] Error: atlas-protocol directory not found at {PROTOCOL_DIR}", file=sys.stderr)
        return False
    if not ensure_command("node"):
        print("[atlas] Error: Node.js is required for atlas-protocol", file=sys.stderr)
        return False
    pnpm = pnpm_command()
    if not pnpm:
        print("[atlas] Error: pnpm is required for atlas-protocol (corepack pnpm is supported)", file=sys.stderr)
        return False

    print("[atlas] Preparing Atlas Protocol artifacts...")
    install = run_process([*pnpm, "install"], cwd=PROTOCOL_DIR)
    if install.returncode != 0:
        print("[atlas] Failed to install atlas-protocol dependencies", file=sys.stderr)
        return False

    script = "verify" if run_tests else "build:all"
    build = run_process([*pnpm, "run", script], cwd=PROTOCOL_DIR)
    if build.returncode != 0:
        print(f"[atlas] Failed to run atlas-protocol {script}", file=sys.stderr)
        return False

    if not copy_protocol_artifacts(check_only=check_only):
        if check_only:
            print("[atlas] Atlas Protocol artifacts are stale. Run `python3 atlas.py protocol-sync`.", file=sys.stderr)
        return False
    if not verify_protocol_artifacts(local_only=False):
        return False

    print("[atlas] Atlas Protocol artifacts are ready.")
    return True


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
    if not ensure_protocol_ready():
        return False
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


def parse_args():
    parser = argparse.ArgumentParser(description="Atlas Core startup/reset tool")
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

    reset_parser = subparsers.add_parser("reset", help="Stop Atlas Core and delete Docker Compose volumes")
    reset_parser.add_argument("--force", action="store_true", help="Skip reset confirmation")

    restart_parser = subparsers.add_parser("restart", help="Reset, rebuild, and start Atlas Core")
    restart_parser.add_argument("--force", action="store_true", help="Skip reset confirmation")
    subparsers.add_parser("protocol-sync", help="Build and sync Atlas Protocol artifacts into atlas-core/protocol")
    subparsers.add_parser("protocol-check", help="Verify Atlas Protocol artifacts are current")

    return parser.parse_args()


def run_command(args):
    if args.command == "start":
        return start()
    if args.command == "stop":
        return stop()
    if args.command == "reset":
        return stop_reset(force=args.force or args.menu_force)
    if args.command == "restart":
        return restart(force=args.force or args.menu_force)
    if args.command == "protocol-sync":
        return ensure_protocol_ready()
    if args.command == "protocol-check":
        return ensure_protocol_ready(run_tests=True, check_only=True)
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
        elif choice == "4":
            if not ensure_protocol_ready():
                sys.exit(1)
        elif choice == "0":
            print("[atlas] Goodbye.")
            break
        else:
            print("[atlas] Invalid choice. Enter 1, 2, 3, 4, or 0.")


if __name__ == "__main__":
    main()
