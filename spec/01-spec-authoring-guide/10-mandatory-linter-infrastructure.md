# Mandatory Linter Infrastructure

**Version:** 3.1.0  
**Updated:** 2026-04-16  
**Type:** AI Instruction — Infrastructure Requirement  
**Priority:** Critical — Enforce Before Any Code Review or Validation

---

## Purpose

This file declares the **mandatory linter infrastructure** that must exist in every project repository that uses this spec system. If these folders or files are missing, the AI agent or contributor must **immediately flag the absence** and request that the files be uploaded or created before proceeding with code review, validation, or standards enforcement.

---

## AI Instruction

```
IMPORTANT — AI INSTRUCTION:

1. Before running any code review, validation, or standards check, verify that the
   mandatory linter infrastructure described below exists in the project repository.

2. If any required folder or file is MISSING:
   a. STOP — do not proceed with validation.
   b. Inform the user: "Required linter infrastructure is missing."
   c. List exactly which folders/files are absent.
   d. Ask the user to upload or create the missing files before continuing.

3. Do NOT attempt to generate linter scripts from scratch — they contain
   project-specific rules and must be provided by the project owner.

4. Do NOT skip linter checks because infrastructure is missing — the absence
   itself is the issue that must be resolved first.
```

---

## Required Folders

The following folders MUST exist at the **root of the project repository**:

| # | Folder | Purpose | Mandatory |
|---|--------|---------|-----------|
| 1 | `linter-scripts/` | Automated validation scripts — coding standards enforcement, link scanning, dashboard generation | ✅ Yes |

### Accepted Alternative Layout

Projects MAY use a unified `linters/` parent folder instead of placing `linter-scripts/` at the root:

```
# Option A — Root-level (default)
project-root/
├── linter-scripts/
│   ├── validate-guidelines.py
│   ├── validate-guidelines.go
│   ├── generate-dashboard-data.cjs
│   ├── check-axios-version.sh
│   ├── run.sh
│   └── run.ps1
└── ...

# Option B — Nested under linters/
project-root/
├── linters/
│   └── linter-scripts/
│       ├── validate-guidelines.py
│       ├── validate-guidelines.go
│       ├── generate-dashboard-data.cjs
│       ├── check-axios-version.sh
│       ├── run.sh
│       └── run.ps1
└── ...
```

Either layout is acceptable. The AI agent must check for **both** layouts before reporting the infrastructure as missing.

---

## Required Files Within `linter-scripts/`

The following files form the minimum linter infrastructure:

| # | File | Purpose | Mandatory |
|---|------|---------|-----------|
| 1 | `validate-guidelines.py` | Python-based coding standards validator — enforces CODE-RED and STYLE rules against `src/` | ✅ Yes |
| 2 | `validate-guidelines.go` | Go-based coding standards validator — alternative/companion to the Python version | ✅ Yes |
| 3 | `generate-dashboard-data.cjs` | Node.js script — validates cross-reference link integrity, generates system health dashboard | ✅ Yes |
| 4 | `check-axios-version.sh` | Shell script — verifies Axios dependency is pinned to a safe version | ✅ Yes |
| 5 | `run.sh` | Shell runner — executes the full validation suite (Linux/macOS) | ✅ Yes |
| 6 | `run.ps1` | PowerShell runner — executes the full validation suite (Windows) | ✅ Yes |

---

## Verification Procedure

AI agents and contributors MUST follow this procedure when onboarding a project or starting a validation task:

### Step 1 — Check Folder Existence

```
Check if ANY of these paths exist:
  - <project-root>/linter-scripts/
  - <project-root>/linters/linter-scripts/
```

If **neither** exists → STOP and report:

> ⚠️ **Required linter infrastructure is missing.**  
> The `linter-scripts/` folder was not found at the project root or under `linters/`.  
> Please upload or create the linter scripts before proceeding with code review or validation.

### Step 2 — Check Required Files

Once the folder is located, verify that all six required files listed above are present.

If any file is missing → report:

> ⚠️ **Incomplete linter infrastructure.**  
> The following required linter files are missing from `linter-scripts/`:  
> - `<missing-file-1>`  
> - `<missing-file-2>`  
> Please upload the missing files before proceeding.

### Step 3 — Proceed

Only after both checks pass should the AI proceed with code review, validation, or standards enforcement.

---

## What These Scripts Do

| Script | What It Validates | Failure Impact |
|--------|-------------------|----------------|
| `validate-guidelines.py` | CODE-RED violations (error handling, file metrics, boolean naming, nesting, typing) and STYLE violations (formatting, blank lines, naming) | Zero violations required — any CODE-RED or STYLE failure blocks contribution |
| `generate-dashboard-data.cjs` | Cross-reference link integrity across all `spec/` and `.lovable/` markdown files | Zero broken links required — ensures documentation is internally consistent |
| `check-axios-version.sh` | Axios dependency pinned to safe version (not 1.14.1 or 0.30.4) | Blocks deployment if vulnerable version detected |
| `run.sh` / `run.ps1` | Orchestrates the full validation suite | Entry point for CI/CD and manual runs |

---

## Why This Is Mandatory

1. **Standards enforcement is automated** — without these scripts, coding standards become advisory rather than enforced.
2. **Link integrity** — the dashboard generator catches broken cross-references that would confuse AI agents and contributors.
3. **Security** — the Axios version check prevents known-vulnerable dependencies from shipping.
4. **Consistency** — the validation scripts encode the exact same rules documented in the coding guidelines spec, ensuring documentation and enforcement stay synchronized.

---

## Cross-References

| Reference | Location |
|-----------|----------|
| Coding Standards (CODE-RED rules) | [../02-coding-guidelines/00-overview.md](../02-coding-guidelines/00-overview.md) |
| Spec Authoring Guide | [00-overview.md](./00-overview.md) |
| Security — Axios Version Control | [../02-coding-guidelines/11-security/01-axios-version-control/00-overview.md](../02-coding-guidelines/11-security/01-axios-version-control/00-overview.md) |
| Folder Structure | [01-folder-structure.md](./01-folder-structure.md) |

---

```
IMPORTANT — AI INSTRUCTION:
- This file defines MANDATORY project infrastructure.
- linter-scripts/ (or linters/linter-scripts/) MUST exist in every repository using this spec system.
- If missing, STOP and ask the user to upload the files before proceeding.
- Do NOT generate linter scripts from scratch — they are project-specific.
- Do NOT skip validation because the scripts are absent — flag the absence as the issue.
```
