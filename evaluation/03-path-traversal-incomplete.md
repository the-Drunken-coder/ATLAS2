# 03 — Path-traversal hardening is incomplete

## Fix complexity

**Medium.** The codebase already has the build-tag split (`nofollow_unix.go` / `nofollow_other.go`), so the structural seam exists. The Linux path is a few hundred lines including the `openat2` syscall plumbing (Go's stdlib doesn't expose it directly). The macOS fallback is more fiddly — the per-component walk has to handle `..` and absolute paths and renames-during-traversal correctly. Realistic estimate: 2–4 days, mostly because testing it on both Linux and macOS adds time.

## Issue

`O_NOFOLLOW` and `safeJoinUnderRoot` together do not match what the spec asks for: a symlink at any *intermediate* path component is silently followed, and `safeJoinUnderRoot` has a TOCTOU window between resolving the parent and opening the leaf.

## In depth

Three concrete gaps in `objectstorage/store.go`:

1. **`O_NOFOLLOW` is leaf-only.** On Unix it only refuses to follow a symlink at the *final* path component. If anything (a bug, an attacker with file-write access on the volume, a misconfigured backup tool) plants a symlink at `/var/lib/atlas-core/objects/<some_id>` pointing at `/etc`, every subsequent file write under that object writes through the symlink — outside the storage root.

2. **`safeJoinUnderRoot` is TOCTOU.** Look at `store.go:286-321`: it calls `EvalSymlinks` on `filepath.Dir(candidate)` when the leaf doesn't exist, then later `openFileNoFollow` opens the leaf. A symlink can be planted between those two calls.

3. **No volume-boundary check.** Spec says `SPEC.md:101`: "validate that the final resolved inode is within the storage volume boundary." There is no `fstatfs` / device-id check anywhere.

Test coverage matches the gap: `TestWriteObjectFile_RejectsSymlinkFile` covers the leaf-symlink case. The intermediate-symlink case ("`obj_link/file.txt` where `obj_link` is a symlink") is not tested.

There's also a related but separate issue: `nofollow_other.go` returns a hardcoded error on non-Unix builds. The whole storage layer is non-functional on Windows. That may be intentional, but it's not documented in the README or spec.

## Recommended fix

The right primitive depends on the OS:

- **Linux:** `openat2` with `RESOLVE_NO_SYMLINKS | RESOLVE_BENEATH`. This is one syscall and refuses to traverse any symlink at any depth, with the kernel enforcing that the result stays under a starting directory FD. Available since 5.6.
- **Other Unix (macOS, BSD):** Walk each path component manually, opening each with `O_PATH | O_NOFOLLOW` (or platform equivalent), and verify the final inode's device matches the storage root's device.

Concrete steps:

1. Open the storage root once at startup and hold the directory FD.
2. Replace `safeJoinUnderRoot` + `openFileNoFollow` with a helper that does the resolve-and-open as a single atomic operation rooted at that FD.
3. Add an `fstatfs`/`Stat_t.Dev` check against the root's device id.
4. Add tests for: symlink at leaf (already exists), symlink at intermediate component (missing), symlink replacement during operation (missing), and a path that crosses a device boundary if feasible to set up.
5. Document the non-Unix stance explicitly somewhere visible.
