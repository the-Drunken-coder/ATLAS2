# 16 — Logging is structured at the envelope but not the payload

## Fix complexity

**Low to medium.** Migrating to `slog` is mechanical but touches every log call site (~30-ish). 1–2 days. The bigger payoff is that future log infrastructure (Loki, Datadog, whatever) gets queryable structured data for free.

## Issue

`LogEntry` is a JSON envelope with named fields, but the actual content is jammed into a single free-form `message` string built via concatenation — so anything you'd want to query on later (object_id, entity_id, error code) can't be filtered without regex parsing.

## In depth

`logging/logging.go:44-51`:

```go
type LogEntry struct {
    Timestamp string `json:"timestamp"`
    RunID     string `json:"run_id"`
    Service   string `json:"service"`
    Component string `json:"component"`
    Level     string `json:"level"`
    Message   string `json:"message"`
}
```

And how it's used (`function.go`):

```go
f.log.Info("entity", "creating entity "+entity.EntityID)
f.log.Info("object", "writing file "+filename+" in object "+objectID)
f.log.Info("task", "deleting task "+taskID)
```

Identifiers like `entity.EntityID`, `objectID`, and `filename` are first-class fields in the data model — they belong in structured fields, not stringified into the message. Right now, "find every log line about object `obj_42`" is a substring search on `message`.

Two related gaps:

1. **Stores don't log at all.** Only the function layer logs. So when something fails inside a store, the only signal is the wrapped error returned to the function — there's no audit of *what failed*, just the eventual returned error chain.
2. **No request_id / operation_id.** Spec leaves this for later (`SPEC.md:212-217`) but the structure to support it isn't in place — adding it later means revisiting every log call.

The minor performance gripe — `shouldLog` does a map lookup on every call (`logging.go:76-86`) — is genuinely minor; `level` could be pre-resolved to an int once at construction. Mention only because it's right next to the other logging issues.

## Recommended fix

1. Change the logger API to take structured fields:

   ```go
   func (l *Logger) Info(component, message string, fields ...Field)
   ```

   Where `Field` is a simple `{Key string; Value any}` (or borrow from `slog` — Go 1.21+ has `log/slog` and the project is on 1.26). Using `slog` directly is probably the right move — it's the standard library answer to exactly this problem, with structured fields, levels, and JSON output built in.

2. Replace `f.log.Info("entity", "creating entity "+entity.EntityID)` with `f.log.Info("creating entity", slog.String("entity_id", entity.EntityID))`.

3. Add a logger to each store and log on errors, so failures are captured at the layer they occur rather than only at the function layer where they're caught.

4. Pre-resolve the level to an int once at construction (`logging.go:35-41`).

5. Plumb a `request_id` / `operation_id` through `context.Context` so future API/SSE handlers can attach it without re-touching every log call.
