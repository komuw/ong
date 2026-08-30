# Error stack preservation plan

## Scope

This plan applies to `errors/`.

The package must keep one canonical origin stack for a linear error chain. The first stack captured by this package is the canonical stack. Later wrapping must not replace it. Later wrapping must also not remove any error from the chain.

A multi-cause error must retain every non-nil input error. Each branch must keep its own existing origin stack.

This plan does not add propagation stacks at every wrap site. It also does not add a new exported stack collection API.

## Source file responsibilities

`errors/stdlib.go` is only for forwarding standard-library APIs that this package does not implement itself. Its forwarding functions must remain thin pass-through calls with no local stack or wrapping policy.

`Errorf` has package-specific stack behavior. It is an implementation owned by this package, not a standard-library forwarding function. Move it from `errors/stdlib.go` to `errors/errors.go`. Keep `As`, `Is`, and `Unwrap` in `errors/stdlib.go` as direct forwards to the standard `errors` package.

`Join` has package-specific stack selection, formatting, and capture behavior. It is not a direct forward to `errors.Join`, so its implementation remains in `errors/join.go`.

Future standard-library APIs must go in `errors/stdlib.go` only when this package forwards them without custom behavior.

## Current issue

`stackError` stores one error and one stack in `errors/errors.go`. The normal direct path already keeps the first stack:

1. `New` captures a stack.
2. The first `Wrap` of an error without an `ong/errors` stack captures a stack.
3. A later direct `Wrap` or `Dwrap` returns the same `stackError`.
4. `Errorf` can copy an existing stack into a new outer `stackError`.

The current behavior is not consistent when other wrappers exist in the chain.

### An intermediate wrapper can be removed

`wrap` calls `Unwrap` once when it detects a nested `stackError`. If the immediate child is a `stackError`, `wrap` returns that child. This removes the outer error passed to `Wrap`.

This can remove:

- Message context.
- A concrete error type.
- Custom `Is` or `As` behavior.
- Other metadata held by the wrapper.

### A deeper stack can be replaced in visible output

If an existing `stackError` is more than one wrapper deep, `wrap` captures a new stack. The old stack remains in the chain, but `%+v` and `StackTrace` use the new outer stack. The visible result is then last-stack-wins.

### `StackTrace` does not traverse the complete chain

`StackTrace` checks the top error and one child. It does not recursively handle:

- More than one `Unwrap() error` layer.
- `Unwrap() []error` trees.

It can return an empty string even when a deeper error has a stack.

### `Errorf` selects stacks from unrelated arguments

`Errorf` checks every argument for a direct `*stackError`. It does not check whether the argument is wrapped with `%w`.

Consequences:

- An error formatted with `%v` can supply the returned stack.
- With multiple stack error arguments, the last direct argument wins.
- A stack below another wrapper is not found by this argument scan.

### `stackError.Is` gives unrelated errors the same identity

`stackError.Is` reports true for every target with type `*stackError`. As a result, two unrelated errors created by this package can match through `errors.Is`.

Type discovery must use `errors.As`. `errors.Is` must remain an identity and semantic-equivalence operation.

### `Join` does not retain its input errors

`Join` converts each input error to text and wraps a new standard error. It does not expose the original inputs through `Unwrap() []error`.

Consequences:

- `errors.Is` and `errors.As` cannot reach the joined inputs.
- Stack traces from branches are discarded.
- Stack selection checks `errs[0]`, not the first non-nil error.

### The race test exposes fragile stack skipping

With Go 1.27, this command currently fails:

```text
go test -race github.com/komuw/ong/errors
```

`TestStackError/success/errors.Dwrap` expects `errors_test.go:124`. The race build starts the visible trace at lines 129 and 131. The same test passes without `-race`.

The package uses a fixed `runtime.Callers` skip count. Deferred calls and race instrumentation can change the visible frame sequence. Tests must not require a frame that the runtime does not expose in all supported build modes.

## Required behavior

### `New`

- Return a non-nil error for any input string.
- Capture a new origin stack.
- Keep the current `%s`, `%q`, `%v`, and `%+v` behavior.

### `Wrap`

- Return nil for a nil error.
- Capture a stack only when the complete error tree has no stack created by this package.
- Preserve the innermost existing stack when one exists.
- Preserve the complete input error and its unwrap chain.
- Never return a nested child in place of the input error.
- Ensure that `%+v` on the returned error can show the canonical stack.
- Avoid a new `runtime.Callers` call when a canonical stack already exists.

For a linear chain, “innermost” means the stack nearest to the root cause. For a multi-cause tree, traversal is depth-first and left-to-right. The first branch with an existing stack supplies the singular canonical stack used by `StackTrace` and the outer `%+v` formatter.

### `Dwrap`

- Apply the same rules as `Wrap` when the pointed-to error is non-nil.
- Do nothing when the pointed-to error is nil.
- Preserve a stack captured before deferred execution.
- Capture a new stack only when no package stack exists.

### `Errorf`

- Build the formatted error with `fmt.Errorf` so standard `%w` behavior remains intact.
- Search the resulting unwrap chain or tree for an existing package stack.
- Preserve the innermost existing stack when `%w` wraps an error that has one.
- Capture a new stack at `Errorf` when the formatted error has no wrapped package stack.
- Do not use errors formatted only with `%v`, `%s`, or `%q` as stack sources.
- Preserve every branch produced by multiple `%w` verbs.
- Keep the complete formatted error as the wrapped cause.

### `StackTrace`

- Return an empty string for nil or when no package stack exists.
- Traverse the complete error chain.
- Support both `Unwrap() error` and `Unwrap() []error`.
- Return the innermost stack for a linear chain.
- Search multi-cause branches from left to right.
- Return the first branch stack found by that traversal.
- Return the same canonical stack that `%+v` shows after `Wrap`, `Dwrap`, or `Errorf`.

### Formatting

- `%s`, `%q`, and `%v` must keep the complete outer error message.
- `%+v` must print the complete outer error message and one canonical origin stack.
- `%+v` must not print duplicate copies of a shared canonical stack.
- A standard or custom wrapper between the formatter and the origin error must remain in the error chain.

This package will continue to use one full canonical stack for linear errors. It will not copy the `pkg/errors` behavior of printing every full stack captured at each wrapper.

### `Join`

- Discard nil inputs.
- Return nil when all inputs are nil.
- Retain every non-nil input error as an error value, not only as text.
- Implement `Unwrap() []error` so standard `errors.Is` and `errors.As` can traverse all branches.
- Keep the current newline-separated `Error` text.
- Keep each branch and its existing stack unchanged.
- Use the first available branch stack, in input order, as the singular canonical stack.
- Capture the `Join` call stack only when no branch has a package stack.
- Document the singular `StackTrace` selection rule.

Changing `Join` from `Unwrap() error` to `Unwrap() []error` is an observable API change. It is required to retain the original errors and to follow the standard Go multi-error model.

## Implementation plan

### 1. Add internal stack traversal

Add an unexported helper in `errors/errors.go` or a focused internal source file.

The helper must:

1. Accept an `error`.
2. Recurse into `Unwrap() error` and `Unwrap() []error`.
3. Inspect children before the current error so the innermost stack wins.
4. Visit multi-cause children from left to right.
5. Return the selected `*stackError`, or equivalent internal stack data.
6. Return no match for nil.

Use type assertions or `errors.As` for stack type discovery. Do not use `errors.Is` as a type test.

The implementation must not alter the error tree while it searches.

### 2. Remove stack type matching from `errors.Is`

Remove `stackError.Is` from `errors/errors.go`.

Update tests that use `errors.Is(err, &stackError{})` as a type assertion. Use a direct assertion or `errors.As` instead.

Add a regression test that confirms two unrelated errors from `New` do not match with `errors.Is`.

### 3. Make `wrap` preserve the input chain

Refactor `wrap` to use the traversal helper.

Required cases:

- Nil input: return nil.
- Direct `*stackError`: return it unchanged.
- Existing nested package stack: return an outer `stackError` that wraps the complete input error and references the existing canonical stack.
- No existing package stack: capture a new stack and wrap the complete input error.

The nested-stack case must not call `Unwrap` once and return the child. It must retain the original argument as the cause.

Reusing an existing stack slice is acceptable because the stack data is immutable after capture.

### 4. Move and refactor `Errorf`

Move `Errorf` from `errors/stdlib.go` to `errors/errors.go`. After the move, `errors/stdlib.go` must contain only direct forwarding functions.

Remove the loop that scans all format arguments for direct `*stackError` values.

After `fmt.Errorf` returns:

1. Search that returned error tree for an existing package stack.
2. If a stack exists, wrap the complete formatted error with a formatting carrier that uses that stack.
3. If no stack exists, capture a new stack at `Errorf`.

This makes `%w`, rather than argument type or argument order, control stack inheritance.

### 5. Make `StackTrace` use the traversal helper

Replace the current direct and one-level checks in `StackTrace`.

The function must use the same selection helper as `Wrap` and `Errorf`. This prevents differences between stack preservation, formatting, and extraction.

Keep the exported return type as `string`. Do not add `StackTraces` as part of this change.

### 6. Replace the current `Join` representation

Refactor `errors/join.go` so the join value stores `[]error`.

The join type must provide:

- `Error() string` with newline-separated child messages.
- `Unwrap() []error` with the retained non-nil children.

Pass the join value through the same stack-preservation logic used by `Wrap`. The traversal helper will select the first available branch stack. If there is no branch stack, capture a stack at `Join`.

Do not copy child messages into a new `errors.New` value as the only cause.

### 7. Keep formatting and stack selection consistent

Ensure that the outer stack carrier:

- Calls the complete wrapped error’s `Error` method for the message.
- Prints only the selected canonical stack for `%+v`.
- Does not recursively print duplicate local stacks.

A nested branch stack must remain available through that branch’s retained error value even when the singular top-level formatter selects another branch.

### 8. Update documentation

Update public comments in:

- `errors/errors.go`
- `errors/join.go`

Keep the comments in `errors/stdlib.go` limited to the direct forwarding behavior of `As`, `Is`, and `Unwrap`.

Document these rules:

- The first package stack is the canonical stack for a linear chain.
- Later wrapping preserves the error chain and reuses that stack.
- `Errorf` inherits a stack only through `%w`.
- `StackTrace` traverses nested wrappers.
- `Join` retains all causes and selects the first available branch stack for its singular stack output.

Update examples if their output or unwrap assumptions change.

Add a changelog entry because `Join.Unwrap` and `errors.Is` behavior will change.

## Test plan

### Direct wrapping

Add or update tests for:

- `Wrap(nil)` returns nil.
- `New` followed by repeated `Wrap` keeps the original stack.
- Repeated `Dwrap` keeps the original stack.
- `Errorf("context: %w", err)` keeps the wrapped origin stack.
- Later wrap locations are not selected as the canonical stack.
- The complete error message remains present.

Prefer comparing a trace captured before and after wrapping. Do not depend only on hard-coded source line numbers.

### External wrappers

Add wrapper types with distinct messages and observable `Is` or `As` behavior.

Test:

- One external wrapper around a `stackError`.
- Two or more external wrappers around a `stackError`.
- `fmt.Errorf("context: %w", err)` around a `stackError`.
- `Wrap` around each of these chains.

Verify:

- Every wrapper remains reachable.
- The complete message remains unchanged.
- `errors.Is` and `errors.As` still find the original values.
- The origin stack remains the canonical stack.
- `StackTrace` works at arbitrary wrapper depth.

### `Errorf`

Test:

- One direct `%w` operand with a stack.
- A `%w` operand whose stack is below custom wrappers.
- An error used only with `%v`; it must not supply its stack.
- Multiple `%w` operands.
- A mix of wrapped and non-wrapped error arguments.
- No `%w`; the stack must start at `Errorf`.

For multiple `%w`, verify that all wrapped errors remain reachable through `errors.Is` and that the first branch with a stack supplies the singular canonical trace.

### Error identity

Test that:

- `errors.Is(err, err)` remains true.
- A wrapped sentinel remains reachable.
- Two unrelated errors returned by `New` do not match.
- Removing `stackError.Is` does not break normal unwrap traversal.

### `Join`

Update existing `errors/join_test.go` tests and add cases for:

- No inputs.
- Only nil inputs.
- Nil before the first non-nil input.
- One input.
- Multiple plain errors.
- Multiple stack errors.
- Mixed plain and stack errors.
- Nested joins.

Verify:

- `Error` keeps the current newline format.
- `Unwrap() []error` returns the original non-nil values in order.
- `errors.Is` and `errors.As` reach every branch.
- Each child still exposes its original stack when inspected directly.
- The top-level singular stack uses the first available branch stack.
- `Join` captures its own stack only when no branch has one.

### Formatting

Test `%s`, `%q`, `%v`, and `%+v` for:

- A direct stack error.
- A stack error below custom wrappers.
- `Errorf` with `%w`.
- A joined error.

Verify that `%+v` contains one canonical stack and does not duplicate it.

### Race-mode stack behavior

Update the `Dwrap` test so it checks frames that are stable in normal and race builds. Do not require the deferred function’s return line when that frame is absent under race instrumentation.

The test must still verify that:

- The trace points to application code.
- The outer caller is present.
- Internal `runtime` frames are not printed.

## Validation

Run these commands after implementation:

```text
gofmt -w errors/*.go
go test github.com/komuw/ong/errors
go test -race github.com/komuw/ong/errors
go vet github.com/komuw/ong/errors
```

Then run the repository test suite:

```text
go test ./...
go test -race ./...
```

Review benchmark results for `New`, first `Wrap`, and repeated `Wrap`. Repeated wrapping of an error that already has a package stack must not capture another runtime stack.

## Acceptance criteria

The work is complete when all of these statements are true:

1. A linear error chain has one canonical origin stack.
2. Repeated wrapping does not replace that stack.
3. Wrapping never removes an existing error or wrapper from the chain.
4. `Errorf` inherits stacks only through `%w` relationships.
5. `StackTrace` finds package stacks at any unwrap depth.
6. `StackTrace` supports multi-cause errors with a documented left-to-right rule.
7. `Join` retains all non-nil input errors through `Unwrap() []error`.
8. Unrelated stack errors do not match through `errors.Is`.
9. `%+v` prints the complete message and one canonical stack without duplicates.
10. `errors/stdlib.go` contains only direct standard-library forwarding functions.
11. The package passes its normal and race test suites.
