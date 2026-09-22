# Plugin Sandbox Bug Report


**Review period:** 2026-09-22 to 2026-09-23  
**Scope:** Plugin stderr bounding and Linux sandbox stdin forwarding 
## Executive summary

The investigation found two independent issues:

1. Plugin stderr collection was unbounded on direct and sandbox execution paths.
2. The real Linux sandbox serialized `PluginStdin` to the helper but never connected it to the plugin's stdin before `syscall.Exec`.

The stderr implementation was bounded and made fail-closed for invalid limits. The Linux stdin issue was fixed by writing the request to a sealed memfd, duplicating it onto fd 0 before Landlock/seccomp lockdown, and then executing the plugin. Real-sandbox tests now cover non-empty, nil, empty, and multi-megabyte stdin.

All requested Windows and WSL validation commands passed. changes were pushed.

## 1. Initial forensic investigation

The initial review covered:

- Git status, local commits, baseline diff, module manifests, untracked files, and reflog.
- Verification that the documentation commits were already on `origin/main`.
- Full diffs of every commit ahead of `origin/main`.
- Full inspection of the affected plugin and sandbox files.
- Six known concerns covering test coverage, fail-closed behavior, formatting, Windows tests, stray files, module changes, identity, and scope.
- Build, vet, lint, Windows tests, and WSL race tests.
- Security reasoning about memory growth, success/failure semantics, authoritative limits, and invariants A, C1, and C2.

Findings from the inventory:

- The worktree initially had no untracked files.
- `test_invoke_truncation.go` was absent.
- No `go.mod` or `go.sum` changes existed.
- The feature diff was limited to `internal/plugin/**`.
- Documentation commits were already on `origin/main`.

## 2. Stderr findings and fixes

### 2.1 Unbounded stderr collection

Before the stderr change, direct invocation used an unbounded `bytes.Buffer` for plugin stderr. The sandbox paths had the same general retention risk.

This could allow a malicious or buggy plugin to grow host memory by continuously writing stderr.

### Fix

Added `sandbox.BoundedStderr` in:

- [`internal/plugin/sandbox/policy.go`](../internal/plugin/sandbox/policy.go)

The collector:

- retains at most `Limit` bytes,
- drains excess stderr without retaining it,
- marks output as truncated,
- does not return an overflow error, so the stderr pipe continues draining.

The authoritative default is:

```go
const defaultStderrLimit int64 = 1 << 20
```

in:

- [`internal/plugin/host.go`](../internal/plugin/host.go)

The limit is normalized in `New` and propagated by `buildPolicy`.

### 2.2 Invalid stderr limits

`BoundedStderr.Write` defensively discards data when `Limit <= 0`. Without validation, this could silently discard all stderr.

Both production sandbox implementations now reject non-positive limits before execution:

- [`internal/plugin/sandbox/sandbox_linux.go`](../internal/plugin/sandbox/sandbox_linux.go)
- [`internal/plugin/sandbox/sandbox_other.go`](../internal/plugin/sandbox/sandbox_other.go)

They return:

```text
sandbox: StderrLimit must be strictly positive
```

Direct host options normalize zero and negative values to the positive default.

### 2.3 Test and portability fixes

The following test issues were found and fixed:

- The Windows stderr fixture now uses `plugin.exe` and references the same name in its manifest.
- The non-Linux fallback test now uses `os.Executable()` instead of hard-coded `/bin/true`.
- The existing Linux integration policy now includes `StderrLimit: 4096`.
- The Linux stderr failure fixture now exits non-zero independently of stdin.

The mocked sandbox truncation test remains a test-double test. A separate real Linux integration test exercises `linuxSandbox.Exec`.

## 3. Linux sandbox stdin bug

### Root cause

The Linux supervisor serialized stdin into the helper protocol:

```go
helperInput := sandboxHelperInput{
    Policy:      *policy,
    Executable:  executable,
    Dir:         dir,
    PluginStdin: stdin,
    LandlockABI: abi,
}
```

The protocol declared:

```go
PluginStdin []byte `json:"plugin_stdin"`
```

The helper decoded the JSON from its own stdin:

```go
var input sandboxHelperInput
dec := json.NewDecoder(os.Stdin)
if err := dec.Decode(&input); err != nil {
    return fmt.Errorf("sandbox helper: decode policy: %w", err)
}
```

However, `input.PluginStdin` was never used. The helper eventually called:

```go
return syscall.Exec(input.Executable, []string{input.Executable}, os.Environ())
```

There was no pipe, fd duplication, `cmd.Stdin`, `dup2`, `dup3`, or equivalent transfer to the plugin.

### Impact

The real Linux sandboxed plugin received EOF instead of the request body.

Affected areas:

- Discovery request delivery.
- Capture request delivery under ADR-0013.
- Any plugin protocol that expects request data on stdin.

This was blocking for E1/E2 capture-staging work because Capture plugins could not receive their request body through the real sandbox.

### Independent reproduction

A temporary real-sandbox probe sent `EXPECTED\n` through `sandbox.NewLinuxSandbox(...).Exec`. The plugin read stdin and reported:

```text
plugin stdout="EMPTY" stderr=""
```

The scratch probe was removed after verification.

## 4. Approved stdin forwarding design

A pipe plus writer goroutine was rejected because pipe capacity and process replacement create data-delivery and EOF lifecycle risks.

A temporary file was rejected as unnecessarily filesystem-dependent.

The selected design was an anonymous memfd:

1. Create a memfd with `MFD_CLOEXEC | MFD_ALLOW_SEALING`.
2. Write the complete `PluginStdin` byte slice.
3. Seek to offset zero.
4. Apply `F_SEAL_WRITE | F_SEAL_GROW | F_SEAL_SHRINK`.
5. Duplicate the memfd onto fd 0.
6. Close the original memfd descriptor.
7. Apply resource limits, Landlock, and seccomp.
8. Call `syscall.Exec`.

All memfd, write, seek, fcntl, dup, and close operations happen before `installSeccomp()`. The seccomp filter is a denylist; these syscalls are not denied, and the implementation does not depend on post-filter setup.

### Read-only guarantee

The plugin's fd 0 is backed by a memfd sealed against:

```go
unix.F_SEAL_WRITE |
unix.F_SEAL_GROW |
unix.F_SEAL_SHRINK
```

The plugin can read the request but cannot modify or resize the input object.

### File descriptor state at exec

At `syscall.Exec`, the intended descriptors are:

| FD | Purpose |
|---:|---|
| 0 | Sealed memfd at offset zero; plugin stdin |
| 1 | Parent-provided stdout capture pipe |
| 2 | Parent-provided bounded stderr capture pipe |

The original memfd descriptor is closed after `Dup2`. The helper protocol's original stdin is replaced when fd 0 is overwritten. No separate helper-protocol descriptor remains.

Nil and explicitly empty stdin are identical: both create a zero-length sealed memfd and produce immediate EOF.

## 5. Permanent regression test

Added:

```text
TestLinuxSandboxPluginStdin
```

in:

- [`internal/plugin/sandbox/sandbox_integration_test.go`](../internal/plugin/sandbox/sandbox_integration_test.go)

The test uses the real `NewLinuxSandbox` path and verifies:

- non-empty stdin,
- nil stdin,
- explicitly empty stdin,
- 4 MiB stdin.

The fixture hashes all bytes read from `os.Stdin`; the test compares the exact length and digest.

## 6. Validation results

### Windows

```text
go build ./...
PASS

go vet ./...
PASS

golangci-lint run
0 issues.

go test ./... -count=1
?   	telos/cmd/telos	[no test files]
?   	telos/core	[no test files]
ok  	telos/internal/config	1.153s
ok  	telos/internal/crypto	1.050s
ok  	telos/internal/logger	0.133s
ok  	telos/internal/model	0.110s
ok  	telos/internal/plugin	8.052s
ok  	telos/internal/plugin/sandbox	1.192s
```

### WSL race suite

```text
ok  	telos/internal/config	1.371s
ok  	telos/internal/crypto	1.633s
ok  	telos/internal/logger	1.103s
ok  	telos/internal/model	1.091s
ok  	telos/internal/plugin	28.985s
ok  	telos/internal/plugin/sandbox	26.786s
```

Relevant Linux sandbox tests:

```text
--- PASS: TestLinuxSandboxIntegration (6.35s)
--- PASS: TestLinuxSandboxStderrLimitValidation (0.00s)
--- PASS: TestLinuxSandboxStderrTruncation (10.21s)
--- PASS: TestLinuxSandboxPluginStdin (9.13s)
```

Detailed stdin test:

```text
=== RUN   TestLinuxSandboxPluginStdin
=== RUN   TestLinuxSandboxPluginStdin/non_empty
=== RUN   TestLinuxSandboxPluginStdin/nil
=== RUN   TestLinuxSandboxPluginStdin/empty
=== RUN   TestLinuxSandboxPluginStdin/large
--- PASS: TestLinuxSandboxPluginStdin (8.64s)
    --- PASS: TestLinuxSandboxPluginStdin/non_empty (0.02s)
    --- PASS: TestLinuxSandboxPluginStdin/nil (0.01s)
    --- PASS: TestLinuxSandboxPluginStdin/empty (0.01s)
    --- PASS: TestLinuxSandboxPluginStdin/large (0.34s)
PASS
ok  	telos/internal/plugin/sandbox	8.657s
```

## 7. Actions requested during the staged investigation

### Initial forensic review

The requested actions were:

- Inventory all local unpushed work.
- Verify documentation commits were already on `origin/main`.
- Read every changed line and affected file.
- Verify the six known concerns.
- Run Windows build, vet, lint, and tests.
- Run WSL race tests.
- Confirm scope, identity, manifests, stray files, and security invariants.

These actions were completed.

### Stderr remediation stage

The requested fixes were:

1. Correct Windows executable naming.
2. Use a platform-valid absolute executable in the fallback test.
3. Add a positive stderr limit to the existing Linux integration policy.
4. Make the Linux stderr failure fixture independent of stdin.

All four were implemented and validated.

### Stdin Stage 1

The design note evaluated:

- pipe plus writer goroutine,
- memfd or temporary file,
- Landlock/seccomp ordering,
- empty stdin,
- large stdin,
- fd lifecycle.

The recommendation was a sealed memfd configured before sandbox lockdown.

### Stdin Stage 2

After approval:

- stdin forwarding was implemented,
- a permanent real-sandbox regression test was added,
- nil, empty, non-empty, and large input were tested,
- both-platform validation was run.

## 8. Final status

The identified stderr and stdin bugs have fixes in the working tree, and the full requested validation matrix is green. The remaining operational step is to review the staged/unstaged separation and create the intended commits when approval is available.
