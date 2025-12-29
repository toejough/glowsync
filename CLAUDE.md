For every interaction we have:

1. Evaluate if any of the user agents should be used. Say YES or NO and explain why.
2. If any are YES, and the user is not already using one of those agents, suggest switching to that agent.

The user agents are POINTLESS if you don't actually evaluate their descriptions and use them for the purposes they were
intedended for.

## Git Commits

When the user asks to commit changes:

1. **Always check git status first** to see what has changed
2. **If changes span multiple categories** (code, tests, docs, config, dependencies), USE the git-commit-organizer agent
3. **The agent will split changes** into proper conventional commits following best practices
4. **Default to using the agent** unless it's a single trivial change to one file

**Key insight**: Simplicity of the user's request ≠ simplicity of organizing the commits properly.

Examples of when to use the agent:
- User says "let's commit our work" and git status shows changes to code + tests + docs
- Multiple files modified across different parts of the codebase
- Any time there are 4+ modified files
- After completing a feature that involved several types of changes

## Linter Configuration Lessons

### Key Learnings from Linter Fixes

1. **Constructor Ordering (funcorder)**
   - Constructors must come before struct methods in Go files
   - Place all `New*()` functions immediately after the struct definition and before methods
   - The mage reorder tool may move functions around - it's authoritative for declaration order

2. **Godoc Comments (godoclint)**
   - Avoid decorative section headers (like `// ====...`) inside const blocks
   - These headers are interpreted as godoc for the next symbol
   - Only one file per package should have a package-level godoc comment
   - If multiple files have package comments, it triggers "more than one godoc" errors

3. **Line Length (lll)**
   - Maximum line length is 120 characters
   - For struct tags that need alignment and exceed 120 chars, use `//nolint:lll` comment
   - The `//nolint` comment must be placed at the END of the long line after gofmt
   - gofmt will reformat and align nolint comments to the right

4. **Struct Tag Alignment (tagalign)**
   - Struct tags should be aligned for readability
   - Use consistent spacing between tag components
   - Don't remove alignment to fix line length - use `//nolint:lll` instead
   - Example: `arg:"-s,--source"             help:"Source directory path"`

5. **Package Naming (revive var-naming)**
   - Generic package names like "shared" trigger revive warnings
   - Each .go file in a package can independently trigger the warning
   - Adding `//nolint:revive` to individual files creates "unused directive" errors
   - **Solution**: Add exclusion rule to `dev/golangci.toml` instead:
     ```toml
     [[linters.exclusions.rules]]
     path = "internal/tui/shared/"
     text = "var-naming.*avoid meaningless package names"
     linters = ['revive']
     ```

6. **Parameter Naming (varnamelen)**
   - Avoid single-letter parameter names unless they're idiomatic (like `i` in loops)
   - Use descriptive names for parameters in functions with significant scope
   - Example: Rename `s string` to `changeTypeStr string` in parsing functions

### Best Practices

- Always run `mage check` before committing
- Use `//nolint:linter-name` sparingly and with explanatory comments
- Prefer fixing issues over suppressing them with nolint
- For package-wide linter exclusions, modify `dev/golangci.toml`
- Let gofmt and the mage reorder tool handle formatting and declaration order
