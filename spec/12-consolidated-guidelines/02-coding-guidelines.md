# Consolidated: Coding Guidelines — Complete Reference

**Source:** [`../02-coding-guidelines/`](../02-coding-guidelines/)  
**Updated:** 2026-04-16  
**Version:** 3.2.0

---

## Purpose

This is the **single consolidated reference** for all coding guidelines. An AI reading only this file must be able to enforce every rule without consulting the source specs. Each section maps to one or more source files — follow cross-references for deeper examples and edge cases.

---

## 1. Naming Conventions — Zero-Underscore Policy

**Source:** `01-cross-language/11-key-naming-pascalcase.md`, `22-variable-naming-conventions.md`, `10-function-naming.md`

### 1.1 PascalCase Is the Default for All Keys

**ALL string keys** across the project use PascalCase. This overrides language defaults (e.g., JavaScript's camelCase convention).

| Key Type | ❌ Wrong | ✅ Correct |
|----------|----------|-----------|
| JSON response/request keys | `"userId"`, `"createdAt"` | `"UserId"`, `"CreatedAt"` |
| Log context keys | `"errorCode"` | `"ErrorCode"` |
| Config keys (YAML/JSON) | `"readTimeout"` | `"ReadTimeout"` |
| PHP array keys | `$data['pluginVersion']` | `$data['PluginVersion']` |
| Database column names | `user_id`, `created_at` | `UserId`, `CreatedAt` |
| WebSocket message types | `"streamStart"` | `"StreamStart"` |

**Go struct rule:** Go serializes to PascalCase by default — **omit explicit JSON tags** unless `omitempty` or `json:"-"` is needed.

```go
// ✅ CORRECT — implicit PascalCase serialization
type User struct {
    Id        string
    SessionId string
    CreatedAt time.Time
}
// Serializes to: {"Id":"...","SessionId":"...","CreatedAt":"..."}
```

### 1.2 Language-Specific Identifier Conventions

| Language | Identifiers | Database Columns | Enum String Values |
|----------|-------------|------------------|--------------------|
| Go | PascalCase (exported), camelCase (unexported) | PascalCase | PascalCase |
| TypeScript | PascalCase (types/keys), camelCase (variables) | PascalCase | PascalCase |
| PHP | PascalCase (keys/values) | PascalCase | PascalCase |
| C# | PascalCase (methods/props) | PascalCase | PascalCase |
| **Rust** | **snake_case (RFC 430)** | **PascalCase** | **PascalCase** |

**Key rule:** Rust follows its community conventions for identifiers, but database names and enum string values are **always** PascalCase for cross-system consistency.

### 1.3 Abbreviation Standard

Common acronyms MUST use full uppercase: `AI`, `DB`, `CI/CD`, `PHP`, `UI`, `API`, `HTTP`, `URL`, `ID`. This overrides general PascalCase rules for these specific terms.

```go
// ❌ WRONG
type DbConfig struct { ... }
type ApiResponse struct { ... }

// ✅ CORRECT
type DBConfig struct { ... }
type APIResponse struct { ... }
```

### 1.4 Variable Naming

| Rule | ❌ Wrong | ✅ Correct |
|------|----------|-----------|
| Singular for one item | `$users = findById($id)` | `$user = findById($id)` |
| Plural for collections | `$user = findAll()` | `$users = findAll()` |
| Loop variable = singular of collection | `for x of plugins` | `for plugin of plugins` |
| Maps use `Map` or `By` suffix | `const prices = {}` | `const priceByProductId = {}` |

### 1.5 Function Naming — No Boolean Flag Parameters

When a boolean parameter changes the **meaning** of an operation, split into separate named methods:

```typescript
// ❌ FORBIDDEN
logMessage("Failed", true);  // What does true mean?

// ✅ REQUIRED
logMessage("User saved");
logMessageWithStack("Payment failed");
```

### 1.6 File/Folder Naming

| File Type | Convention | Example |
|-----------|-----------|---------|
| `.md` spec files | lowercase-kebab-case with numeric prefix | `02-boolean-principles.md` |
| Go files | snake_case | `deploy_path.go` |
| TypeScript files | kebab-case | `user-profile.tsx` |
| C# files | PascalCase | `UserProfile.cs` |
| PowerShell `.ps1` | lowercase-kebab-case | `run-validator.ps1` |
| PHP classes | PascalCase matching class name | `FileLogger.php` |
| PHP WordPress main file | kebab-case matching slug | `plugin-slug.php` |

### 1.7 Slugs

All slugs (URLs, API endpoints, file paths) MUST be **lowercase kebab-case**. Characters: `a-z`, `0-9`, `-` only. No underscores, spaces, dots, leading/trailing hyphens, or consecutive hyphens.

---

## 2. Boolean Principles (P1–P8)

**Source:** `01-cross-language/02-boolean-principles/` (5 files)

### P1: Always Use `is` or `has` Prefix

Every boolean — variable, property, parameter, or method — **must** start with `is` or `has`. 99% use `is`/`has`. The word `should` is acceptable in rare cases.

**Banned prefixes:** `can`, `was`, `will`, `not`, `no`.

```php
// ❌ FORBIDDEN
$active = true;
$loaded = false;

// ✅ REQUIRED
$isActive = true;
$isLoaded = false;
$hasPermission = true;
```

```typescript
// ❌ FORBIDDEN
const loading = true;

// ✅ REQUIRED
const isLoading = true;
```

```go
// ❌ FORBIDDEN
blocked := true

// ✅ REQUIRED
isBlocked := true
hasItems := len(items) > 0
```

Methods follow the same rule: `$order->hasOverdue()`, `$user->isAdmin()`.

### P2: Never Use Negative Words in Boolean Names

The words **`not`**, **`no`**, and **`non`** are **absolutely banned** from boolean names. Use a **positive semantic synonym** instead.

| ❌ Forbidden | ✅ Required | Meaning |
|-------------|------------|---------|
| `isNotReady` | `isPending` | Waiting |
| `hasNoPermission` | `isUnauthorized` | Lacks access |
| `isNotBlocked` | `isActive` | Active |
| `isClassNotLoaded` | `isClassUnregistered` | Not registered |
| `isNoRecentErrors` | `isErrorListClear` | Clean error list |

### P3: No Raw Negation Operators

**Never** use `!` or `not` on function calls or existence checks. Wrap every negative check in a **positively named guard function**.

| ❌ Forbidden | ✅ Required |
|-------------|------------|
| `!file_exists($path)` | `PathHelper::isFileMissing($path)` |
| `!is_dir($path)` | `PathHelper::isDirMissing($path)` |
| `!arr.includes(x)` | `isMissing(arr, x)` |
| `!strings.Contains(s, x)` | `IsMissing(s, x)` |
| `!$obj->isActive()` | `$obj->isDisabled()` |

### P4: Extract Complex Boolean Expressions

Any condition with more than **one operand** must be extracted into a named boolean variable or function.

```typescript
// ❌ FORBIDDEN
if (user.age >= 18 && user.hasVerifiedEmail && !user.isBanned) { ... }

// ✅ REQUIRED
const isEligible = user.age >= 18 && user.hasVerifiedEmail;
const isAllowed = isEligible && !user.isBanned;
if (isAllowed) { ... }
```

### P5: Explicit Boolean Parameters Only

Never pass `true`/`false` as unnamed arguments. Use named parameters, separate methods, or an options object.

### P6: Never Mix `&&` and `||` in a Single Condition

Max 2 operands per condition. Never mix `&&`/`||` without extracting sub-expressions.

```typescript
// ❌ FORBIDDEN
if (isAdmin && isActive || hasOverride) { ... }

// ✅ REQUIRED
const hasAccess = isAdmin && isActive;
const isAllowed = hasAccess || hasOverride;
if (isAllowed) { ... }
```

### P7–P8: No Inline Boolean Statements / No Raw System Calls in Conditions

P7: Never use `return condition ? true : false` — return the boolean directly.  
P8: Never call system functions (`file_exists`, `is_dir`) directly in conditions — always wrap in a named guard.

---

## 3. Code Style — Braces, Nesting, Spacing, Size

**Source:** `01-cross-language/04-code-style/` (7 files)

### Rule 1: Always Use Braces

Every `if`, `for`, `while` block **must** use curly braces `{}`, even for single-statement bodies.

### Rule 2: Zero Nested `if` — Absolute Ban

Nested `if` blocks are **forbidden**. Flatten using combined conditions, early returns, or extracted helpers.

```php
// ❌ FORBIDDEN
if ($error !== null) {
    if (ErrorChecker::isFatalError($error)) {
        $this->logger->fatal($error);
    }
}

// ✅ REQUIRED — isFatalError handles null internally
if (ErrorChecker::isFatalError($error)) {
    $this->logger->fatal($error);
}
```

Max nesting depth: **2 levels** (function body → one control structure). Max **2 operands** per conditional.

### Rule 3: No Redundant `else` After Return

If an `if` block ends with `return`, `throw`, `break`, or `continue`, the code after the block is implicitly the "else" branch. Adding an explicit `else` is **forbidden**.

```go
// ❌ FORBIDDEN — redundant else after return
if order == nil {
    return ErrNilOrder
} else {
    return process(order)
}

// ✅ REQUIRED — flat
if order == nil {
    return ErrNilOrder
}

return process(order)
```

**Exception:** `else` is acceptable when **neither branch returns** — both branches assign a value and execution continues.

### Rule 4: Blank Line Before `return`/`throw`

Insert one blank line before `return`/`throw` **only if** preceded by other code. If it's the only statement, no blank line.

### Rule 5: Blank Line After Closing `}`

A blank line is required after `}` **unless** the next line is another `if`, `else`, `case`, or closing `}`.

### Rule 6: Maximum 15 Lines Per Function

Every function body must be **≤ 15 lines** (excluding blank lines, comments, and the signature). Error-handling lines (`if err != nil`, `apperror.Wrap()`) are **exempt** from this count.

### Rule 17: Maximum 120 Lines Per Struct/Class

Structs, classes, and interfaces must not exceed 120 lines. Extract behavior into focused sub-types.

### File Size Limits

| Metric | Limit |
|--------|-------|
| Function body | ≤ 15 lines |
| File size | < 300 lines (hard max 400) |
| React components | < 100 lines |
| Struct/class | ≤ 120 lines |
| Parameters per function | ≤ 3 |
| Cognitive complexity | ≤ 10 |

---

## 4. Cyclomatic Complexity — Target Zero

**Source:** `01-cross-language/06-cyclomatic-complexity.md`

Target complexity of **0–1 per function** using guard clauses to eliminate nesting entirely. Every nested `if` adds a branch, increases indentation, and forces readers to mentally track multiple conditions.

### Resolution Methods

| Method | When to Use |
|--------|-------------|
| **Guard clauses (early return)** | Invert conditions, exit early, keep happy path at bottom |
| **Named boolean extraction** | Replace compound inline conditions with named variables |
| **Extract to named function** | Pull conditional logic into a helper with a descriptive name |

```csharp
// ❌ BAD — 5 levels of nesting (Complexity: 5)
public void Process(Order? order) {
    if (order != null) {
        if (order.IsVerified) {
            if (order.Items.Count > 0) { ... }
        }
    }
}

// ✅ GOOD — flat guard clauses (Complexity: 1)
public void Process(Order? order) {
    if (order == null) { return; }
    if (order.IsUnverified) { return; }
    var isItemListEmpty = order.Items.Count <= 0;
    if (isItemListEmpty) { return; }
    // All guards passed — process the order
    order.IsProcessed = true;
}
```

---

## 5. Strict Typing

**Source:** `01-cross-language/13-strict-typing.md`

Every function parameter, return value, and class property **must** have an explicit type declaration.

| Language | Key Rules |
|----------|-----------|
| PHP | All params, returns, and properties typed. Remove redundant `@param`/`@return` docblocks |
| TypeScript | `any` is **prohibited**. `unknown` only at parse boundaries with immediate narrowing |
| Go | `interface{}`/`any` prohibited in exported APIs. **Single return value:** `apperror.Result[T]` — never `(T, error)`. No type assertions `.(Type)` in business logic |
| Rust | No `unwrap()` in production code |
| C# | Nullable reference types enabled. All public methods explicitly typed |

### No Inline Return Types

```typescript
// ❌ FORBIDDEN — inline return type
function getUser(): { name: string; age: number } { ... }

// ✅ REQUIRED — named type
interface UserInfo { name: string; age: number }
function getUser(): UserInfo { ... }
```

---

## 6. Generic Return Types — No `interface{}`/`any`/`object`

**Source:** `01-cross-language/25-generic-return-types.md`

🔴 **CODE RED:** When a method returns different types based on context, use generic Result types or generics — never `interface{}`, `any`, `object`, or `unknown`.

```go
// ❌ BAD — interface{} return forces caller to cast
func (c *Cache) Get(key string) interface{} { return c.store[key] }

// ✅ GOOD — generic function
func Get[T any](c *Cache, key string) (T, bool) { ... }

// ✅ GOOD — Result wrapper (project pattern)
func (s *Service) ProcessOrder(input Input) apperror.Result[OrderData] { ... }
```

```typescript
// ❌ BAD — any return
function fetchData(endpoint: string): Promise<any> { ... }

// ✅ GOOD — generic function
async function fetchData<T>(endpoint: string): Promise<T> { ... }
```

**Rule:** If a function could return multiple types, split into separate typed methods instead of returning a union or `any`.

---

## 7. Casting Elimination

**Source:** `01-cross-language/03-casting-elimination-patterns.md`

Type assertions and casts are **banned** in business logic. Every cast is a potential runtime panic.

| Language | Forbidden Pattern | Required Alternative |
|----------|-------------------|---------------------|
| Go | `value.(Type)` | Generic functions, discriminated interfaces |
| TypeScript | `value as Type` | Type guards, `is` predicates |
| PHP | `(int)$val` | `intval()` with validation, `PhpNativeType` enum |
| C# | `(Type)value` | Pattern matching, `is` operator |

**Acceptable exceptions:**
- Parse boundaries (JSON decode, HTTP request body)
- Framework callbacks with predefined signatures
- Test code

---

## 8. Magic Values & Immutability

**Source:** `01-cross-language/26-magic-values-and-immutability.md`

### Rule: No Magic Strings or Numbers

Every string literal used in comparisons, switch statements, or assignments **must** be replaced with a named constant or enum.

```typescript
// ❌ FORBIDDEN — magic string
if (user.status === "active") { ... }

// ✅ REQUIRED — enum
if (user.status === UserStatus.Active) { ... }
```

### Rule: Immutable by Default

Use `const` over `let`/`var`. Reassignment is the exception, not the rule. In Go, prefer value receivers and avoid pointer mutation where possible.

---

## 9. Null Pointer Safety

**Source:** `01-cross-language/19-null-pointer-safety.md`

**Never access a pointer, array, or return value without checking for nil/null first.** Every pointer dereference is a potential panic/crash.

| Rule | Description |
|------|-------------|
| **Error before value** | Always check `err` before using the returned value |
| **Never chain method calls on unchecked returns** | Separate creation from execution, check after each step |
| **Check pointer before dereference** | Explicit nil check before `*ptr` |
| **Check array/slice before index access** | Nil and length check before `arr[0]` |

```go
// ❌ DANGEROUS — Output() called directly, will panic if command fails
output, err := exec.Command(args...).Output()

// ✅ SAFE — separate creation from execution
cmd, err := exec.Command(args...)
if err != nil || cmd == nil {
    return exitResult
}
output := cmd.Output()
```

---

## 10. Code Mutation Avoidance

**Source:** `01-cross-language/18-code-mutation-avoidance.md`

Minimize mutable state. Prefer creating new values over mutating existing ones.

| Rule | Detail |
|------|--------|
| Avoid mutating function parameters | Return new values instead of modifying inputs |
| Avoid global mutable state | Use dependency injection or explicit context |
| Avoid accumulator-style loops | Prefer `map`, `filter`, `reduce` where idiomatic |
| Mark mutations explicitly | If mutation is unavoidable, document it clearly |

---

## 11. DRY Principles

**Source:** `01-cross-language/08-dry-principles.md`

- **3+ lines** of identical logic → extract to function
- **2+ components** sharing state → extract to custom hook/service
- **2+ endpoints** sharing validation → extract to middleware
- Composition over inheritance — always

---

## 12. SOLID Principles

**Source:** `01-cross-language/23-solid-principles.md`

| Principle | Rule | Example |
|-----------|------|---------|
| **S — Single Responsibility** | One class/function = one reason to change | Split `PluginService` into `PluginService` (CRUD) + `PluginValidator` + `PluginFormatter` |
| **O — Open/Closed** | Open for extension, closed for modification | Use interfaces: `Exporter` interface → `CsvExporter`, `JsonExporter` |
| **L — Liskov Substitution** | Subtypes must be substitutable for their base type | Don't override methods to throw "not supported" |
| **I — Interface Segregation** | Many small interfaces > one fat interface | Split `Repository` into `Reader`, `Writer`, `Deleter` |
| **D — Dependency Inversion** | Depend on abstractions, not concrete types | Accept `Logger` interface, not `FileLogger` directly |

---

## 13. Lazy Evaluation

**Source:** `01-cross-language/16-lazy-evaluation-patterns.md`

Defer expensive computations until they are actually needed. Never compute values "just in case".

| Pattern | Rule |
|---------|------|
| Conditional computation | Only compute inside the branch that uses the result |
| Lazy initialization | Initialize resources on first access, not at startup |
| Short-circuit evaluation | Order conditions so cheap checks come first |

---

## 14. Regex Guidelines

**Source:** `01-cross-language/17-regex-usage-guidelines.md`

| Rule | Detail |
|------|--------|
| Name every regex | Assign to a named constant: `const emailPattern = /^.../` |
| Compile once | Pre-compile regex at module level, never inside loops |
| Comment complex patterns | Add inline comments for non-trivial regex |
| Prefer string methods first | Use `startsWith`, `endsWith`, `includes` when regex is overkill |

---

## 15. Test Naming & Structure

**Source:** `01-cross-language/14-test-naming-and-structure.md`

### Three-Part Convention

Every test function follows: `Test{Unit}_{Scenario}_{ExpectedOutcome}`

```go
// ❌ FORBIDDEN — vague
func TestCreateSession(t *testing.T) { ... }

// ✅ REQUIRED — three-part naming
func TestCreateSession_WithValidCredentials_ReturnsSessionId(t *testing.T) { ... }
func TestCreateSession_WithExpiredToken_ReturnsAuthError(t *testing.T) { ... }
```

```typescript
describe('UserProfile', () => {
    it('renders_WithValidUser_ShowsDisplayName', () => { ... });
    it('onSubmit_WithInvalidEmail_ShowsValidationError', () => { ... });
});
```

### Test File Rules

- One test file per source file — no multi-source test files
- Test file resides in same directory as source file
- Integration tests use `_integration_test.go` / `.integration.test.tsx` suffix

### Table-Driven Tests (Go)

```go
tests := []struct {
    Name     string
    Input    string
    Expected string
}{
    {Name: "EmptyInput_ReturnsDefault", Input: "", Expected: "default"},
    {Name: "ValidSlug_ReturnsTrimmed", Input: " my-slug ", Expected: "my-slug"},
}

for _, tt := range tests {
    t.Run(tt.Name, func(t *testing.T) {
        result := Process(tt.Input)
        assert.Equal(t, tt.Expected, result)
    })
}
```

---

## 16. Types Folder Convention

**Source:** `01-cross-language/27-types-folder-convention.md`

All shared type definitions live in a dedicated `types/` folder. No types defined inline in handler or service files.

| Language | Location | Convention |
|----------|----------|------------|
| Go | `internal/types/` | One file per domain (`user_types.go`, `session_types.go`) |
| TypeScript | `src/types/` | One file per domain (`user.types.ts`) |
| PHP | `includes/Enums/` | Backed enums with `Type` suffix |

---

## 17. Error Handling — 🔴 CODE RED

Swallowing errors is a **CODE RED** violation. Every error must be explicitly handled.

| Rule | Violation Level |
|------|----------------|
| Swallowed errors | 🔴 CODE RED |
| Generic "file not found" without exact path | 🔴 CODE RED (CODE-RED-009) |
| Missing error context/reason | 🔴 CODE RED |

```go
// ❌ CODE RED — swallowed error
result, _ := doSomething()

// ✅ REQUIRED — handle or propagate
result := doSomething()
if result.HasError() {
    return result.PropagateError()
}
```

---

## 18. Language-Specific Standards

### TypeScript

- No `any` — use generics or `unknown` with narrowing
- String-based enums with PascalCase keys and values
- `async/await` over raw promises
- `strict: true` in tsconfig — no exceptions
- Named interfaces for all discriminated union variants
- `TypedAction<TType, TPayload>` pattern for action types

### Go

- PascalCase for exports, camelCase for unexported
- `defer` for cleanup — defer runs LIFO, captures values at defer-time
- Error wrapping via `apperror.Wrap()` — never raw `fmt.Errorf`
- `Result[T]` pattern — single return value, never `(T, error)`
- No `interface{}`/`any` in exported APIs
- Enum: `type Variant byte` with `iota`, `Invalid` zero-value, mandatory `String()`, `Parse()`, `IsAnyOf()`
- Code severity taxonomy: 🔴 CODE RED (fatal) → 🟠 WARN → 🟡 STYLE → 🟢 BEST PRACTICE

### PHP

- PSR-12 compliance
- No `else` chains — use early returns
- `ResponseKeyType` enum for response keys
- String-backed enums with `isEqual()` method and `Type` suffix
- `TypeCheckerTrait` for type-safe validation: `isString()`, `isArray()`, `isInteger()`
- `PhpNativeType` enum for type validation (`String`, `Integer`, `Boolean`, `Array`, `Float`, `Null`)

### Rust

- snake_case for identifiers (RFC 430)
- PascalCase for DB columns and enum string values
- `thiserror` for error types
- No `unwrap()` in production — use `?` operator
- `Debug`, `Clone`, `PartialEq` derives on all enums
- `Result<T, E>` with `?` propagation

### C#

- PascalCase for methods and properties
- Nullable reference types enabled
- PascalCase DB columns and enum values
- Pattern matching preferred over type casting

---

## 19. Blank Line Rules — Quick Reference

| Scenario | Blank Line? |
|----------|-------------|
| Before `return`/`throw` preceded by code | ✅ Yes |
| Before `return`/`throw` as only statement | ❌ No |
| After closing `}` | ✅ Yes (unless next line is `if`/`else`/`}`) |
| At start of function body | ❌ Never |
| Inside single-statement braces | ❌ Never |
| Double blank lines anywhere | ❌ Never |

---

## 20. Nesting Resolution Patterns

**Source:** `01-cross-language/20-nesting-resolution-patterns.md`

Three methods to flatten nested code:

1. **Extract to Named Function** — Pull inner logic into a descriptively named helper
2. **Inverse Logic (Early Return)** — Negative case exits first, happy path continues flat
3. **Named Boolean Variables** — Decompose compound conditions into named booleans

For deeply nested code (Level 4+), combine all three methods:
1. Identify exit conditions → extract guard cases
2. Name intermediate booleans
3. Extract helper functions
4. Use early returns → flatten remaining logic

---

## 21. Validation

Run `linter-scripts/validate-guidelines.py` — zero **CODE-RED** or **STYLE** violations required for all contributions.

---

## Cross-References

| Topic | Full Spec Location |
|-------|-------------------|
| Boolean Principles (P1–P8) | `02-coding-guidelines/01-cross-language/02-boolean-principles/` |
| No Raw Negations | `01-cross-language/12-no-negatives.md` |
| PascalCase Keys | `01-cross-language/11-key-naming-pascalcase.md` |
| Variable Naming | `01-cross-language/22-variable-naming-conventions.md` |
| Function Naming | `01-cross-language/10-function-naming.md` |
| Code Style (7 files) | `01-cross-language/04-code-style/` |
| Strict Typing | `01-cross-language/13-strict-typing.md` |
| Magic Values | `01-cross-language/26-magic-values-and-immutability.md` |
| DRY Principles | `01-cross-language/08-dry-principles.md` |
| Slug Conventions | `01-cross-language/28-slug-conventions.md` |
| Casting Elimination | `01-cross-language/03-casting-elimination-patterns.md` |
| Generic Return Types | `01-cross-language/25-generic-return-types.md` |
| Cyclomatic Complexity | `01-cross-language/06-cyclomatic-complexity.md` |
| SOLID Principles | `01-cross-language/23-solid-principles.md` |
| Null Pointer Safety | `01-cross-language/19-null-pointer-safety.md` |
| Code Mutation Avoidance | `01-cross-language/18-code-mutation-avoidance.md` |
| Lazy Evaluation | `01-cross-language/16-lazy-evaluation-patterns.md` |
| Regex Guidelines | `01-cross-language/17-regex-usage-guidelines.md` |
| Test Naming & Structure | `01-cross-language/14-test-naming-and-structure.md` |
| Types Folder Convention | `01-cross-language/27-types-folder-convention.md` |
| Nesting Resolution | `01-cross-language/20-nesting-resolution-patterns.md` |
| Master Coding Guidelines | `01-cross-language/15-master-coding-guidelines/` |
| Enum Standards | `../12-consolidated-guidelines/04-enum-standards.md` |
| Boolean Flag Methods | `01-cross-language/24-boolean-flag-methods.md` |

---

*Consolidated coding guidelines — v3.2.0 — 2026-04-16*
