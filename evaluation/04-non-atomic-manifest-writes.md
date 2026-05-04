# 04 — Manifest writes are not atomic

## Fix complexity

**Low.** Probably 30 lines of code in `writeFileNoFollow` (or a new `writeFileAtomicNoFollow` helper used only for manifests). One crash-injection test. Half a day, including review.

## Issue

`WriteManifestFile` truncates and rewrites `manifest.json` in place, so a process kill or power loss mid-write leaves a torn manifest — and the spec promotes the filesystem manifest to the single source of truth.

## In depth

`objectstorage/store.go:332-340` (`writeFileNoFollow`):

```go
f, err := openFileNoFollow(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
...
_, err = f.Write(data)
```

`O_TRUNC` zeroes the file before the write, then `Write` may or may not complete. If the process is OOM-killed, the container is restarted, or the host loses power between truncate and write, the result on disk is a half-finished or zero-byte `manifest.json`.

What happens next:

- `ReadManifestFile` returns invalid JSON.
- `function.GetObjectManifest` (`function.go:226-235`) catches the read error or unmarshal error and either returns the error to the caller or — if the read returns `ErrNotFound` — silently returns an empty manifest.
- The empty-manifest fallback is *especially* dangerous because the spec says the FS manifest is canonical. A torn manifest that reads as missing makes the system act as though the object has no files, even though the files are still present on disk.

There is also no `fsync` of the file or the parent directory, so even a successful-looking write isn't guaranteed durable across a kernel crash.

## Recommended fix

Standard atomic-write pattern:

1. Write to `manifest.json.tmp` (in the same directory, so `rename` is atomic).
2. `fsync` the temp file.
3. `rename` over `manifest.json` — this is atomic on POSIX.
4. `fsync` the parent directory so the rename itself is durable.

Add a crash-injection test that aborts the process between truncate and write, then asserts the manifest is either the old version or the new version — never torn.

Consider applying the same pattern to `WriteObjectFile` if any object file is treated as canonical state (probably not — those are caller-supplied bytes — but worth a look).
