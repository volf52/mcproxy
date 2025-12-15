# Test Performance Analysis & "Hanging" Tests

**Date:** 2025-12-20
**Context:** Investigation into slow or "hanging" `go test` execution, particularly with the `-race` flag.

## Summary

The investigation identified that the perceived "hanging" behavior was primarily due to intensive race condition and stress tests in the `pkg/mcp` package (spawning real OS processes) and deliberate `time.Sleep` calls in `pkg/proxy`.

**Update (2025-12-20):** These tests have been refactored to use `MockCommander` (avoiding OS process overhead) and channel-based synchronization (avoiding fixed delays). The test suite is now significantly faster and more deterministic.

## Detailed Analysis

### 1. Package: `pkg/mcp` (Process Management)

* **Refactored:** Now uses `MockCommander` to simulate OS processes.
* **Performance:** Drastically improved. `pkg/mcp` tests now run in ~2s even with `-race`.
* **Short Mode:** Tests no longer skip in `-short` mode because they are fast enough to run always.

### 2. Package: `pkg/proxy` (HTTP Proxy Server)

* **Refactored:** `TestProxyHandler_ContextCancellation` and `TestProxyHandler_Timeout` now use channel synchronization.
* **Performance:** Saved ~0.6s of fixed delays.

## Recommendations

1. **Routine Development:** Run all tests regularly.
    ```bash
    go test ./...
    ```
2. **CI / Pre-commit:** Run with the race detector. It should complete within a few seconds.
    ```bash
    go test -race ./...
    ```
