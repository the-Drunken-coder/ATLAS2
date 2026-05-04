# 17 — Docker / runtime nits

## Fix complexity

**Low.** Each item is independent and small. Together: maybe a day.

## Issue

A handful of small but genuine issues in the container build and the `atlas.py` controller: unnecessarily large runtime image, missing `.dockerignore`, no healthcheck on the Atlas Core container, fixed `sleep(2)` after startup, and a destructive reset that runs without confirmation.

## In depth

### Runtime image is `alpine:3.21` — it could be `scratch` or `distroless`

`Dockerfile:12`. The Go binary is built with `CGO_ENABLED=0` (line 10), so it has no libc dependency. Alpine ships a shell, package manager, and a stack of utilities that the running service never uses — that's attack surface for nothing. `gcr.io/distroless/static-debian12` (or `scratch`, if you don't need ca-certs / tzdata) cuts the image to a few MB and removes anything an attacker who lands a code-exec in the binary could pivot into.

### No `.dockerignore`

The build context copies the entire `atlas-core/` directory into the build, including `.env` if present, local `bin/`, editor swap files, etc. At minimum: `.env`, `*.log`, build artifacts, IDE files. Cheap insurance against accidentally baking secrets into a layer.

### Atlas Core container has no healthcheck

`docker-compose.yml`. Postgres has a healthcheck (lines 12-17). `atlas-core` doesn't. So `atlas.py:36-38` does:

```python
print("[atlas] Waiting for services to be ready...")
time.sleep(2)
```

If the Go service ever takes longer than 2 seconds to start (cold compile in dev, slow disk, contended CPU), the log scrape that follows runs against a not-yet-ready container and produces a misleading "no logs yet" output. Without an HTTP server in this slice there's nothing for a healthcheck to probe — but you can use `pgrep atlas-core` or a small "ready file" the Go service writes after init.

### `Dockerfile`'s `chown` is build-time only

`Dockerfile:16`: `chown -R atlas:atlas /var/lib/atlas-core`. With a fresh named volume Docker copies the in-image directory contents (and ownership) into the volume on first mount, so this works on the first run. If the user has an old volume from a prior run with different perms, or uses a bind mount, the container starts as `atlas` (uid 10001) and can't write to a root-owned mount point. Worth at least a comment in the Dockerfile.

### `atlas.py` option 2 nukes volumes silently

`atlas.py:53-60`:

```python
def stop_reset():
    print("[atlas] Stopping and resetting system...")
    result = run_compose("down", "-v", "--remove-orphans")
```

`-v` deletes the named volumes. There's no confirmation prompt. One mis-keyed `2` at the menu wipes the database and object storage. The spec is explicit that this is intended ("Stop means destructive reset … Stop does not mean 'pause'") — but the menu just says "Stop / Reset system" with no warning, and the irreversibility is buried in the spec, not the UI.

## Recommended fix

1. **Switch base image** to `gcr.io/distroless/static-debian12:nonroot` (or `scratch` if no certs/tz needed). Drop the Alpine-specific `addgroup`/`adduser`; distroless ships a `nonroot` user.
2. **Add `.dockerignore`** with `.env`, `*.log`, `bin/`, `.git`, common editor files.
3. **Add a healthcheck for `atlas-core`.** A small "ready" file written after `app.New()` succeeds, checked by a tiny shell `[ -f /var/lib/atlas-core/.ready ]`. Update `docker-compose.yml`. Replace `time.sleep(2)` in `atlas.py` with a poll on container health.
4. **Add a confirmation prompt** to `atlas.py` option 2 ("This will delete the database and object storage. Continue? [y/N] "). Skipped if `--force` is passed (for scripting).
5. **Document the volume-perm caveat** in the Dockerfile or the spec.
