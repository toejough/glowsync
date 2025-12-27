# Documentation Directory

This directory contains detailed documentation about the TUI architecture, design decisions, and refactoring process.

## Files

### ARCHITECTURE.md
**Purpose:** Complete architectural design document

**Contents:**
- System overview
- Component responsibilities
- Message flow diagrams
- File structure explanation
- Implementation plan
- Benefits of the layered approach

**Audience:** Developers who want to understand the overall architecture

**When to read:** 
- Before making significant changes to the TUI
- When adding new screens
- When debugging cross-screen issues

---

### COMPLEXITY_COMPARISON.md
**Purpose:** Before/after complexity analysis

**Contents:**
- Current monolithic architecture complexity metrics
- Proposed layered architecture complexity metrics
- Detailed breakdown by function
- Migration path
- Estimated effort

**Audience:** Developers interested in the technical rationale for the refactoring

**When to read:**
- When evaluating whether to proceed with the refactoring
- When explaining the refactoring to others
- When measuring success of the refactoring

---

### REFACTORING_PLAN.md
**Purpose:** Step-by-step migration plan

**Contents:**
- New directory structure
- File responsibilities
- Migration steps (phases 1-6)
- Benefits of the new structure
- Estimated effort and risk assessment

**Audience:** Developers performing the refactoring

**When to read:**
- During the refactoring process
- When planning the migration
- When tracking progress

---

### DIRECTORY_PREVIEW.txt
**Purpose:** Visual representation of the directory structure

**Contents:**
- Before/after directory listings
- Navigation examples
- Benefits summary
- Quick reference guide

**Audience:** Anyone who wants a quick overview of the structure

**When to read:**
- When first exploring the codebase
- When looking for where to make changes
- As a quick reference

## Documentation Philosophy

### Keep Documentation Close to Code

Each directory has its own README explaining:
- **What** the directory contains
- **Why** it's organized this way
- **How** to use the code in that directory
- **When** to add new code there

### Documentation Hierarchy

```
internal/tui/
├── README.md                    # Package overview - start here
├── screens/README.md            # Screen-specific architecture
├── shared/README.md             # Shared code guidelines
└── docs/                        # Detailed design docs
    ├── README.md                # This file - doc index
    ├── ARCHITECTURE.md          # Complete architecture
    ├── COMPLEXITY_COMPARISON.md # Technical metrics
    ├── REFACTORING_PLAN.md      # Migration guide
    └── DIRECTORY_PREVIEW.txt    # Visual reference
```

### When to Update Documentation

**Update immediately when:**
- Adding a new screen
- Changing the message protocol
- Modifying the screen flow
- Adding new shared utilities

**Update after stabilization when:**
- Refactoring internal screen logic
- Changing rendering details
- Optimizing performance

### Documentation Standards

**READMEs should:**
- ✅ Be concise and scannable
- ✅ Include examples
- ✅ Explain the "why" not just the "what"
- ✅ Be kept up-to-date with code changes

**READMEs should NOT:**
- ❌ Duplicate information from other READMEs
- ❌ Include implementation details (that's what code comments are for)
- ❌ Become outdated (delete or update, don't let it rot)

## Quick Navigation Guide

**"I want to understand the overall architecture"**
→ Start with `../README.md`, then read `ARCHITECTURE.md`

**"I want to add a new screen"**
→ Read `../screens/README.md` and `REFACTORING_PLAN.md`

**"I want to add a new message type"**
→ Read `../shared/README.md` (messages.go section)

**"I want to understand why we refactored"**
→ Read `COMPLEXITY_COMPARISON.md`

**"I want to see the directory structure"**
→ Read `DIRECTORY_PREVIEW.txt`

**"I want to perform the refactoring"**
→ Follow `REFACTORING_PLAN.md` step by step

## Maintaining These Docs

### After Adding a New Screen

1. Update `../screens/README.md` with new screen details
2. Update `ARCHITECTURE.md` with new flow diagram
3. Update `DIRECTORY_PREVIEW.txt` with new file listing
4. Update `../README.md` if the flow changes

### After Changing Message Protocol

1. Update `../shared/README.md` (messages.go section)
2. Update `ARCHITECTURE.md` (message flow section)
3. Update affected screen READMEs

### After Refactoring

1. Update complexity metrics in `COMPLEXITY_COMPARISON.md`
2. Update `REFACTORING_PLAN.md` to reflect actual vs. planned
3. Add lessons learned to `ARCHITECTURE.md`

## Documentation Principles

1. **Self-Documenting Code First**
   - Good naming > comments > documentation
   - Documentation explains "why", code explains "how"

2. **Keep It DRY**
   - Don't repeat information across docs
   - Link to other docs instead of duplicating

3. **Keep It Current**
   - Outdated docs are worse than no docs
   - Update docs in the same PR as code changes

4. **Make It Scannable**
   - Use headers, lists, and formatting
   - Include examples and diagrams
   - Keep paragraphs short

5. **Know Your Audience**
   - READMEs: Quick reference for developers
   - Architecture docs: Deep understanding for maintainers
   - Plan docs: Step-by-step for implementers

