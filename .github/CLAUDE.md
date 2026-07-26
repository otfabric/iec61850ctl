# claude.md — Engineering Standards

## 0 — Purpose
These rules exist to ensure **maintainability**, **safety**, and **developer velocity**.  
**MUST** = required by CI and review.  
**SHOULD** = strong guidance; deviations require justification.  
**CAN** = permitted and encouraged when appropriate.

---

## 1 — Before Coding
- **BP-1 (MUST)** Ask clarifying questions for ambiguous or incomplete requirements.
- **BP-2 (MUST)** Draft and confirm an approach (API shape, data flow, invariants, failure modes) before writing code.
- **BP-3 (SHOULD)** When >2 approaches exist, list pros/cons and justify the selection.
- **BP-4 (SHOULD)** Define testing strategy (unit, integration, mocks) and required observability signals upfront.
- **BP-5 (CAN)** Produce short design docs for changes that affect public APIs, concurrency, or storage.

---

## 2 — Modules & Dependencies
- **MD-1 (SHOULD)** Prefer stdlib. Introduce deps only with clear cost/benefit reasoning; check transitive size and licenses.
- **MD-2 (CAN)** Use `govulncheck` when upgrading dependencies.
- **MD-3 (MUST)** Avoid unnecessary dependency sprawl; remove unused deps quickly.

---

## 3 — Code Style
- **CS-1 (MUST)** Enforce `gofmt`, `golangci-lint`, and `go vet`.
- **CS-2 (MUST)** Avoid stutter: `package kv; type Store`, not `KVStore`.
- **CS-3 (SHOULD)** Keep interfaces small and owned by consumers; prefer composition over inheritance.
- **CS-4 (SHOULD)** Avoid reflection on hot paths; prefer generics for clarity + performance.
- **CS-5 (MUST)** When a function accepts more than 2 arguments, use an input struct (except `ctx`).
- **CS-6 (SHOULD)** Declare input structs immediately above the function using them.
- **CS-7 (SHOULD)** Keep functions small and readable; avoid excessive nesting; early-return instead of deep if-chains.
- **CS-8 (MUST)** Follow idiomatic Go naming conventions: no Hungarian notation, no interface prefixes like `I`.

---

## 4 — Errors
- **ERR-1 (MUST)** Wrap errors with context using `%w`: `fmt.Errorf("open %s: %w", p, err)`.
- **ERR-2 (MUST)** Use `errors.Is` / `errors.As` for control flow; never string matching.
- **ERR-3 (SHOULD)** Define sentinel errors in the package; document behavior clearly.
- **ERR-4 (CAN)** Use `context.WithCancelCause` for structured cancellation.
- **ERR-5 (MUST)** Do not bury errors — return or explicitly handle them.

---

## 5 — Concurrency
- **CC-1 (MUST)** Only senders close channels.
- **CC-2 (MUST)** Tie goroutine lifetime to `context.Context`; prevent leaks.
- **CC-3 (MUST)** Protect shared state with `sync.Mutex` or `atomic`.
- **CC-4 (SHOULD)** Use `errgroup` for fan-out work and cancel on first error.
- **CC-5 (CAN)** Use buffered channels only with a documented rationale.

---

## 6 — Contexts
- **CTX-1 (MUST)** If a function accepts a context, it must be the first argument.
- **CTX-2 (MUST)** Never store a context in a struct.
- **CTX-3 (MUST)** Honor `ctx.Done` and propagate cancellation.
- **CTX-4 (CAN)** Provide helpers like `WithTimeoutFromConfig(ctx, cfg)`.

---

## 7 — Testing
- **T-1 (MUST)** Use table-driven tests; avoid nondeterministic behavior.
- **T-2 (MUST)** CI runs `go test -race ./...`; use `t.Cleanup` for teardown.
- **T-3 (SHOULD)** Use `t.Parallel()` for safe tests.
- **T-4 (CAN)** Use mocks/fakes where isolation is required.
- **T-5 (SHOULD)** Integration tests should be hermetic (docker-compose, test containers).

---

## 8 — Logging & Observability
- **OBS-1 (MUST)** Use structured logging (`slog`) with consistent fields.
- **OBS-2 (SHOULD)** Correlate logs, metrics, and traces with request IDs from context.
- **OBS-3 (CAN)** Provide debug/pprof endpoints restricted by auth/local-only.
- **OBS-4 (SHOULD)** Log actionable events; avoid noisy logs in hot paths.

---

## 9 — Performance
- **PERF-1 (MUST)** Benchmark or profile before optimizing.
- **PERF-2 (SHOULD)** Avoid unnecessary allocations; use slices/strings efficiently.
- **PERF-3 (CAN)** Add microbenchmarks for critical code paths.
- **PERF-4 (SHOULD)** Avoid premature optimization — correctness first, clarity second.

---

## 10 — Configuration
- **CFG-1 (MUST)** Config via env vars or flags; validate on startup; fail fast.
- **CFG-2 (MUST)** Treat config as immutable after initialization.
- **CFG-3 (SHOULD)** Provide sane defaults and clear docs.
- **CFG-4 (CAN)** Support hot-reload only if correctness is preserved.

---

## 11 — APIs & Boundaries
- **API-1 (MUST)** Document all exported identifiers (packages, functions, types, methods).
- **API-2 (MUST)** Accept interfaces only where variation is required; return concrete types by default.
- **API-3 (SHOULD)** Keep the exported surface minimal and orthogonal.
- **API-4 (SHOULD)** Design explicit, predictable APIs (clear inputs/outputs).
- **API-5 (CAN)** Use constructor-options pattern for extensibility.

---

## 12 — Security
- **SEC-1 (MUST)** Validate all untrusted input; set explicit I/O deadlines.
- **SEC-2 (MUST)** Never log secrets; load them from env/secret manager.
- **SEC-3 (SHOULD)** Apply least-privilege to filesystem and network access.
- **SEC-4 (CAN)** Add fuzz tests for parsers and boundary-layer code.

---

## 13 — CI/CD
- **CI-1 (MUST)** Lint, vet, test (`-race`), and build on every PR.
- **CI-2 (MUST)** Use reproducible builds: `-trimpath` and version injection via `-ldflags "-X main.version=$TAG"`.
- **CI-3 (SHOULD)** Require review sign-off for MUST-rule changes.
- **CI-4 (CAN)** Publish SBOM and run license/vuln scans.

---

## 14 — Tooling
- **TL-1 (CAN)** Code quality tools: `golangci-lint`, `staticcheck`, `gofumpt`.
- **TL-2 (CAN)** Security: `govulncheck`, dependency scanners.
- **TL-3 (CAN)** Testing: `gotestsum`, `mockgen`, `counterfeiter`.
- **TL-4 (CAN)** API tooling: `buf`, `oapi-codegen`.

---

## 15 — Documentation
- **DOC-1 (MUST)** Every exported package must have a package doc comment (`// Package foo ...`).
- **DOC-2 (MUST)** Every exported function, type, interface, and method must have a doc comment describing:
  - what it does  
  - meaning of inputs/outputs  
  - error behavior  
  - invariants or preconditions  
- **DOC-3 (SHOULD)** New modules should include a `README.md` documenting purpose, dependencies, data flow, and testing strategy.
- **DOC-4 (SHOULD)** Comments should describe *intent*, not restate code.
- **DOC-5 (CAN)** Use Go examples (`ExampleFoo`) to illustrate usage.
- **DOC-6 (MUST)** Update documentation whenever behavior changes.

---

## 16 — Tooling Gates
- **G-1 (MUST)** `go vet ./...` passes.
- **G-2 (MUST)** `golangci-lint run` passes.
- **G-3 (MUST)** `go test -race ./...` passes.
- **G-4 (CAN)** Allow `gh pr view` / `gh pr diff` for review context.

---

# Appendix — Function-Writing Best Practices

### FP-1 — Readability First  
If you cannot read the function top-to-bottom and immediately understand its intent, refactor.

### FP-2 — Cyclomatic Complexity  
Reduce deep nesting via extraction and early returns.

### FP-3 — Known Patterns  
Prefer established patterns and data structures over ad-hoc logic.

### FP-4 — Hidden Dependencies  
Functions must not rely on global state; pass dependencies explicitly.

### FP-5 — Naming  
Brainstorm 3 alternative names; pick the clearest.

### FP-6 — Input/Output Clarity  
Inputs must be explicit; outputs must leave no ambiguity. Avoid side effects unless unavoidable.

### FP-7 — Testability  
A function that is hard to test is a design smell—refactor for determinism and isolation.