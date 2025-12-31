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
   - status: review (ready for integration testing)
   - started: 2025-12-30
   - implementation: All core features implemented
     - SFTP connection management with SSH agent/key auth
     - URL parser for sftp://user@host:port/path format
     - Dual filesystem support (local-to-SFTP, SFTP-to-local, SFTP-to-SFTP)
     - Integration with sync engine and TUI
   - usage: glowsync -s sftp://user@host/remote/path -d /local/path
   - updates:
     - 2025-12-30 23:15 EST: Implementation complete, committed in 9 logical commits
     - 2025-12-30 23:20 EST: Running mage check, fixing test compilation issues
     - 2025-12-30 23:25 EST: Added mustNewEngine helper, fixing syncengine tests
     - 2025-12-30 23:33 EST: All tests passing, fixed redeclaration errors in screen tests
     - 2025-12-30 23:49 EST: Fixed SSH agent auth bug - now checks for keys before using agent
     - 2025-12-30 23:55 EST: Fixed SFTP path handling - single slash now relative to home directory
     - 2025-12-31 00:06 EST: Fixed dual filesystem bug - dest scans now use correct filesystem
     - 2025-12-31 00:08 EST: Fixed file deletion bug - dest file removes now use correct filesystem
2. create a way to ignore files on the server side from deletion during sync
   - status: open
3. there's no border around the app in the analysis screen
   - status: open
4. fix impgen V1 deprecation warnings in mage check
   - status: open
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
   - status: open
   - created: 2025-12-30 23:39 EST
   - description: Document SFTP support in CLI help and README
   - required content:
     - Path format: sftp://user@host:port/path (port optional, defaults to 22)
     - Authentication: SSH agent and key files (~/.ssh/id_*)
     - Usage examples:
       - Local to remote: glowsync -s /local/path -d sftp://user@server/remote/path
       - Remote to local: glowsync -s sftp://user@server/remote/path -d /local/path
       - Remote to remote: glowsync -s sftp://user@server1/path -d sftp://user@server2/path
     - Note about SSH key setup and agent configuration
   - files to update:
     - CLI help text (--help flag output)
     - README.md with SFTP usage section
