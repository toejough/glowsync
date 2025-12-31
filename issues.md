# Issue Tracker

A simple md issue tracker.

## Statuses

- backlog (to choose from)
- selected (to work on next)
- in progress (currently being worked on)
- review (ready for review/testing)
- done (completed)
- cancelled (not going to be done, for whatever reason, should have a reason)
- blocked (waiting on something else)

## Issues

1. try to use SSH/SFTP to sync files
   - status: done
   - started: 2025-12-30
   - completed: 2025-12-31
   - implementation: All core features implemented and tested
     - SFTP connection management with SSH agent/key auth
     - URL parser for sftp://user@host:port/path format
     - Dual filesystem support (local-to-SFTP, SFTP-to-local, SFTP-to-SFTP)
     - Integration with sync engine and TUI
   - usage:
     - Local to SFTP: glowsync -s /local/path -d sftp://user@host/remote/path
     - SFTP to local: glowsync -s sftp://user@host/remote/path -d /local/path
     - SFTP to SFTP: glowsync -s sftp://user@host1/path1 -d sftp://user@host2/path2
   - path conventions:
     - Single slash (relative to home): sftp://user@host/Pictures → ~/Pictures
     - Double slash (absolute): sftp://user@host//var/log → /var/log
   - updates:
     - 2025-12-30 23:15 EST: Implementation complete, committed in 9 logical commits
     - 2025-12-30 23:20 EST: Running mage check, fixing test compilation issues
     - 2025-12-30 23:25 EST: Added mustNewEngine helper, fixing syncengine tests
     - 2025-12-30 23:33 EST: All tests passing, fixed redeclaration errors in screen tests
     - 2025-12-30 23:49 EST: Fixed SSH agent auth bug - now checks for keys before using agent
     - 2025-12-30 23:55 EST: Fixed SFTP path handling - single slash now relative to home directory
     - 2025-12-31 00:06 EST: Fixed dual filesystem bug - dest scans now use correct filesystem
     - 2025-12-31 00:08 EST: Fixed file deletion bug - dest file removes now use correct filesystem
     - 2025-12-31 00:22 EST: Verified working end-to-end, marking as complete (16 commits total)
2. create a way to ignore files on the server side from deletion during sync
   - status: backlog
3. there's no border around the app in the analysis screen
   - status: backlog
4. fix impgen V1 deprecation warnings in mage check
   - status: backlog
   - created: 2025-12-30 23:36 EST
   - description: Update impgen directives to use V2 syntax
   - affected files:
     - internal/config/config.go - V1 callable wrapper
     - internal/syncengine/sync.go - V1 callable wrapper
     - internal/syncengine/sync_test.go - V1 interface mock
     - pkg/fileops/fileops_di.go - V1 callable wrapper
     - pkg/fileops/fileops_di_test.go - V1 interface mock
     - pkg/filesystem/filesystem_test.go - V1 interface mock
   - migration: Use --target flag for callable wrappers, --dependency flag for interface mocks
5. add SFTP documentation to help text
   - status: backlog
   - created: 2025-12-30 23:39 EST
   - description: Document SFTP support in CLI help and README
   - required content:
     - Path format: sftp://user@host:port/path (port optional, defaults to 22)
     - Authentication: SSH agent and key files (~/.ssh/id\_\*)
     - Usage examples:
       - Local to remote: glowsync -s /local/path -d sftp://user@server/remote/path
       - Remote to local: glowsync -s sftp://user@server/remote/path -d /local/path
       - Remote to remote: glowsync -s sftp://user@server1/path -d sftp://user@server2/path
     - Note about SSH key setup and agent configuration
   - files to update:
     - CLI help text (--help flag output)
     - README.md with SFTP usage section
6. there's a duplicate (less precise) percentage after the file progress bars `22% (22.5%)`
   - status: backlog
   - description: Remove redundant percentage display in file progress bars. I'd like to keep the second one, and remove
     the first.
7. the file progress bars section frequently shows a higher number of workers than files being synced
   - status: done
   - started: 2025-12-31 01:34 EST
   - completed: 2025-12-31 02:16 EST
   - description: I would expect that if we have 5 workers, we should be syncing 5 files at a time, most of the time.
     However, frequently I see that we have more workers than files being synced, e.g. 5 workers, but only 2 files
     being synced.
   - root cause identified: GetStatus() filtering bug (sync.go:332-346)
     - When worker starts a file, it's added to CurrentFiles and status set to "opening"
     - GetStatus() iterates BACKWARDS from end of FilesToSync array
     - Only includes last 20 files matching status criteria (opening/copying/finalizing/complete/error)
     - If FilesToSync has 500+ files and worker picks up file #50:
       * File #50 is in CurrentFiles (worker tracking)
       * GetStatus() starts from file #500, finds last 20 active files, stops
       * File #50 never makes it into the returned FilesToSync array
       * UI sees 4 workers but only 2 files (those in the last 20)
     - Log evidence: "Workers: 4 | Files to display: 20 (copying:2 ...) | CurrentFiles: 4"
       * 4 workers active, 4 files in CurrentFiles, but only 2 have "copying" status in FilesToSync
   - solution implemented: Priority-based GetStatus() filtering (Option 1)
     - Step 1: Add ALL files from CurrentFiles first (actively being worked on)
     - Step 2: Fill remaining slots (up to 20 total) with recently completed files for context
     - Uses O(1) map lookup to avoid duplicates between steps
   - TDD workflow:
     - RED phase (02:08 EST): Wrote 3 failing tests
       * TestGetStatus_IncludesAllCurrentFiles: Verifies all CurrentFiles in result
       * TestGetStatus_PrioritizesCurrentFilesOverRecent: Verifies priority over recent files
       * TestGetStatus_EmptyCurrentFiles: Verifies behavior when no CurrentFiles
     - GREEN phase (02:12 EST): Implementation in sync.go:332-370
       * Two-pass algorithm: CurrentFiles first, then recent files
       * All 3 new tests pass + all existing tests pass
   - updates:
     - 2025-12-31 01:34 EST: Initial investigation - thought it was missing display states
     - 2025-12-31 01:41 EST: Code review shows all states already displayed. Adding debug logging to find actual root cause.
     - 2025-12-31 01:50 EST: Debug logging implemented and tested. Logs worker count vs file states every render.
     - 2025-12-31 02:02 EST: Root cause identified via log analysis. GetStatus() doesn't include all CurrentFiles in FilesToSync.
     - 2025-12-31 02:08 EST: RED phase - wrote 3 failing tests for GetStatus() CurrentFiles priority.
     - 2025-12-31 02:12 EST: GREEN phase - implemented two-pass algorithm. All tests pass.
     - 2025-12-31 02:16 EST: Fix complete. UI will now always show all actively copying files.
8. adaptive worker count never seems to go down
   - status: done
   - started: 2025-12-31 00:41 EST
   - completed: 2025-12-31 00:50 EST
   - description: The adaptive worker count seems to only ever go up, never down. I would expect that if the system
     is under load, the worker count would go down to reduce load.
   - root cause identified: Three problems found:
     1. MakeScalingDecision (sync.go:362) only adds workers, never decrements desiredWorkers when speed drops
     2. startWorkerControl (sync.go:1523) only handles add=true, never implements worker removal
     3. worker() function (sync.go:1965) has no mechanism to gracefully exit for scale-down
   - solution implemented: Atomic CAS-based worker scale-down:
     - Added desiredWorkers int32 field to Engine (atomic)
     - MakeScalingDecision decrements desiredWorkers when per-worker speed decreases (speedRatio < 0.9)
     - Workers check desiredWorkers vs activeWorkers after each job using CAS loop
     - Winner of CAS race decrements activeWorkers and exits, losers retry - prevents stampede
     - Changed ActiveWorkers from int to int32 for atomic operations
   - testing: Full TDD approach - RED (3 failing tests), GREEN (all passing), all tests pass
   - updates:
     - 2025-12-31 00:41 EST: Root cause analysis complete, started TDD implementation
     - 2025-12-31 00:43 EST: RED phase - wrote 3 failing tests for scale-down behavior
     - 2025-12-31 00:46 EST: GREEN phase - implementing MakeScalingDecision scale-down and worker CAS exit
     - 2025-12-31 00:50 EST: All tests passing, implementation complete
     - 2025-12-31 01:00 EST: Tested working in production - confirmed workers scale down
     - 2025-12-31 01:03 EST: Committed (9445ca5)
9. when cancelling a sync, the TUI reports the sync _failed_ and shows error messages
   - status: backlog
10. the per worker speed seems to fluctuate wildly. we should use a smoother average.
    - status: backlog
