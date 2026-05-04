# 15 — `CreateObjectFolder` has dead code and weak idempotency

## Fix complexity

**Low.** The dead-code removal is a one-line edit. Adding the JSON parse check is another few lines plus a test. Maybe an hour total, separate from the larger issue 03 work.

## Issue

`CreateObjectFolder` calls `objectPath` twice for the same ID and discards the second result, which looks like leftover debugging — and the function's "already exists, skip" branch doesn't validate that the existing folder is in a coherent state.

## In depth

`objectstorage/store.go:40-60`:

```go
func (s *Store) CreateObjectFolder(objectID string) error {
    path, err := s.objectPath(objectID)
    if err != nil {
        return err
    }
    if err := os.MkdirAll(path, 0755); err != nil {
        return fmt.Errorf("create object folder %s: %w", objectID, err)
    }
    if _, err := s.objectPath(objectID); err != nil {  // ← second call, result discarded
        return err
    }
    manifestPath := filepath.Join(path, "manifest.json")
    if _, err := os.Stat(manifestPath); err == nil {
        return nil
    } else if errors.Is(err, os.ErrNotExist) {
        emptyManifest := model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}
        return s.WriteManifestFile(objectID, mustMarshal(emptyManifest))
    } else {
        return fmt.Errorf("stat manifest for %s: %w", objectID, err)
    }
}
```

Two issues:

1. **Dead second call to `objectPath`.** `s.objectPath(objectID)` is called once at the top, and then again at line 48 with the result discarded. `objectPath` does input validation, but nothing has changed between the two calls — the second call is either leftover debugging or a misremembered "re-check after MkdirAll" pattern. Either way, it's dead code with a small but real maintenance cost (anyone modifying this function has to figure out whether the second call is load-bearing).

2. **Weak idempotency on "manifest already exists" branch.** If the folder and `manifest.json` already exist, the function returns success without checking that the manifest is valid JSON or that it represents the right object. So if a previous bad operation left a corrupt manifest behind, `CreateObjectFolder` silently accepts it — and the next caller assumes the object is in a clean state.

3. **Related: `os.MkdirAll` follows intermediate symlinks.** This connects to issue 03 — even though `ValidateObjectID` rejects path separators, `MkdirAll` still walks any directory symlink that already exists in the path. Not a vulnerability today because the only intermediate component is the storage root itself, but worth being aware of.

## Recommended fix

1. Remove the dead second call to `objectPath` (lines 48-50 of the snippet).
2. On the "manifest already exists" branch, either (a) validate it parses as JSON before returning success, or (b) accept that idempotency is "best effort" and document that a corrupt manifest needs explicit repair (issue 07's reconcile function).
3. As part of the issue 03 fix, replace `MkdirAll` with a `mkdirat`-equivalent operation rooted at the storage-root FD.
