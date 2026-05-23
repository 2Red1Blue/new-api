# Local Development Notes

## GoLand Debug Logging

The backend does not use `LOG_LEVEL=debug` to enable debug logs.
`logger.LogDebug(...)` is guarded by `common.DebugEnabled`, which is initialized from:

```text
DEBUG=true
```

For a local GoLand debug run, set these environment variables in the Run/Debug configuration:

```text
DEBUG=true
GIN_MODE=debug
LOG_CALLER_ENABLED=true
```

Meaning:

- `DEBUG=true`: enables `logger.LogDebug(...)` output and debug-only branches.
- `GIN_MODE=debug`: keeps Gin in debug mode instead of release mode.
- `LOG_CALLER_ENABLED=true`: adds `file.go:line` caller information to application logs.

`LOG_LEVEL=debug` is currently ignored by this project unless code is added to map it to `DEBUG=true`.
