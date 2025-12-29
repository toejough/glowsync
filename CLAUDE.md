# MANDATORY AGENT EVALUATION PROTOCOL

**CRITICAL**: For EVERY user request, you MUST start your response with an "Agent Evaluation" section.

## Response Structure (REQUIRED)

```
## Agent Evaluation

**solution-architect**: YES/NO - [reason]
**solution-planner**: YES/NO - [reason]
**tdd-implementer**: YES/NO - [reason]
**git-commit-organizer**: YES/NO - [reason]
**problem-clarifier**: YES/NO - [reason]
**code-reviewer**: YES/NO - [reason]
**Explore agent**: YES/NO - [reason]
**Other agents**: [list any other relevant agents with YES/NO]

**Decision**: [Which agent to use, if any, or state "Direct implementation"]
```

**After evaluation**:
- If YES for any agent → Launch that agent IMMEDIATELY
- If NO for all agents → Proceed with direct implementation

**No exceptions**: Every response must start with this evaluation, even for seemingly simple requests.

---

## Agent Selection Decision Tree

Evaluate agents in this order:

### 1. Is this a codebase exploration/understanding task?

**→ Use Explore agent (via Task tool)**

The user wants to understand code structure, find where things are handled, or map out the architecture.

Examples where Explore applies:
- ✓ "Where are errors from the client handled?"
- ✓ "What is the codebase structure?"
- ✓ "How does authentication work in this app?"
- ✓ "Find all the places where user input is validated"
- ✗ "Read the UserController.go file" (specific file - use Read tool)
- ✗ "Search for the login function" (needle query - use Grep/Glob)

**Key insight**: Open-ended questions about codebase patterns, architecture, or "where/how does X work" need systematic exploration.

### 2. Is the user's request unclear or potentially symptomatic of a deeper issue?

**→ Use problem-clarifier**

The request might be a surface-level symptom or workaround rather than addressing the root problem.

Examples where problem-clarifier applies:
- ✓ "Our API responses are really slow"
- ✓ "Users keep entering the wrong format in the date field"
- ✓ "Can you add a button that exports data to CSV and then re-imports it?"
- ✗ "Add validation to email field" (clear, specific request)

### 3. Is the problem clear but HOW to implement is architecturally unclear?

**→ Use solution-architect**

The user has described WHAT they want but not the architectural approach.

Examples where solution-architect applies:
- ✓ "Add a confirmation screen" (which approach? new class? modify existing? where in the file structure?)
- ✓ "Add real-time updates" (WebSockets? SSE? polling?)
- ✓ "Optimize the database queries" (caching? indexing? query rewriting? connection pooling?)
- ✗ "Create a ConfirmationScreen class in 2.5_confirmation.go that extends BaseScreen" (approach is specified)

**Key insight**: If the user gives a desired outcome without architectural details, you need solution-architect to design the approach and present options with tradeoffs.

### 4. Is the architectural approach decided and clear?

**→ Use solution-planner**

The user (or solution-architect) has specified the architectural approach, now break it into implementation steps.

Examples where solution-planner applies:
- ✓ "We'll use JWT tokens stored in httpOnly cookies with a refresh token pattern"
- ✓ "Create a new ConfirmationScreen class following the existing screen pattern"
- ✗ "Add authentication" (too vague, needs solution-architect first)

### 5. Do you need to explore unfamiliar code patterns first?

**→ Use EnterPlanMode**

You're not familiar enough with the existing codebase patterns to design an approach yet.

When to use EnterPlanMode vs solution-architect:
- **solution-architect**: You understand the codebase patterns well enough to present implementation options
- **EnterPlanMode**: You need to explore/research the codebase to understand existing patterns before you can design

**Default for familiar codebases**: Use solution-architect for new features, not EnterPlanMode.

### 6. Are the implementation steps clear and ready to execute?

**→ Use tdd-implementer (or implement directly)**

You have a clear plan with specific steps. Time to write code following TDD.

**TDD means:**
- Tests don't exist yet - you'll create them first (Red)
- Then implement code to make them pass (Green)
- Then refactor for quality (Refactor)
- Repeat for each feature

**When launching tdd-implementer:**
- Provide the detailed plan from solution-planner
- The agent should proceed immediately with implementation
- Don't expect tests to already exist - creating them is part of TDD
- "Ready to execute" means you have clear steps, not that code/tests already exist

### 7. Did you just write significant code?

**→ Use code-reviewer**

After implementing non-trivial features or changes, proactively review the code.

Examples where code-reviewer applies:
- ✓ Just implemented a new feature with multiple files
- ✓ Made complex refactoring changes
- ✓ Added integration between multiple systems
- ✗ Fixed a typo or single-line change

---

## Agent Completion Protocol

**CRITICAL**: When a specialized agent completes its task, you MUST:

1. **Acknowledge completion**: "The [agent-name] agent has completed."
2. **Re-run the full Agent Evaluation**: Evaluate all agents again for the next step
3. **Chain to next agent if needed**: Based on the evaluation, launch the next appropriate agent

### Common Agent Chains

```
User request for new feature
    ↓
problem-clarifier (if request is unclear)
    ↓
Explore agent (if need to understand codebase patterns)
    ↓
solution-architect (design approach)
    ↓
solution-planner (break into steps)
    ↓
tdd-implementer (execute implementation)
    ↓
code-reviewer (review the code)
    ↓
git-commit-organizer (commit the changes)
```

### Example of Proper Agent Chaining

```
## Agent Evaluation
solution-architect: NO - Already have an approach
solution-planner: YES - Need to break down the confirmation screen implementation

Launching solution-planner agent...

[Agent completes]

The solution-planner agent has completed. Re-evaluating next steps:

## Agent Evaluation
solution-architect: NO - Approach is designed
solution-planner: NO - Plan is complete
tdd-implementer: YES - Have clear steps, ready to implement

Launching tdd-implementer agent...
```

---

## Typical Workflows

### For new features in familiar codebases:
1. **solution-architect** → design approach and get user buy-in
2. **solution-planner** → break approach into steps
3. **tdd-implementer** → execute with tests
4. **code-reviewer** → review implementation
5. **git-commit-organizer** → commit changes

### For understanding existing code:
1. **Explore agent** → map out relevant code
2. Direct response with findings

### For unclear requests:
1. **problem-clarifier** → understand root issue
2. Follow feature workflow or direct implementation

---

## Git Commits

When the user asks to commit changes:

### Step 1: STOP and Evaluate FIRST

Before doing ANYTHING else, check git status and evaluate:

**Should you use the git-commit-organizer agent?**

Answer YES if ANY of these apply:
- ✓ Changes span multiple categories (code + tests, docs + config, library + tests, etc.)
- ✓ Git status shows 4+ modified files
- ✓ Just completed a feature involving multiple types of changes
- ✓ **DEFAULT**: Use the agent unless it's clearly a single trivial file change

**Key insight**: Simplicity of the user's request ≠ simplicity of organizing the commits properly.

### Step 2: If YES - Launch the Agent

**Immediately** launch git-commit-organizer via Task tool:
- Do NOT proceed with manual git commands
- Do NOT try to organize commits yourself
- Let the agent analyze changes and create proper conventional commits

```
Task(subagent_type="git-commit-organizer", prompt="...")
```

### Step 3: If NO - Manual Workflow

Only use manual git commands for single trivial changes:
1. Check git status and git diff
2. Stage changes with git add
3. Create commit following the standard format with co-authorship
4. Run git status to verify

### Examples of When to Use the Agent

- ✓ "let's commit our work" + git status shows code + tests + docs
- ✓ Multiple files modified across different codebase areas
- ✓ Any time there are 4+ modified files
- ✓ After completing a feature that involved several types of changes
- ✗ Single typo fix in one README file

---

## Enforcement Rules

1. **No skipping evaluation**: Every response must start with agent evaluation
2. **No silent agent launches**: Always show the evaluation before launching
3. **No breaking chains**: When an agent completes, re-evaluate and continue the chain
4. **No direct implementation without evaluation**: Even for "simple" tasks, show the evaluation

**The user agents are POINTLESS if you don't actually evaluate their descriptions and use them for the purposes they were intended for.**

---

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
