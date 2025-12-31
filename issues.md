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
   - status: review (tests passing, ready for integration testing)
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
2. create a way to ignore files on the server side from deletion during sync
   - status: open
3. there's no border around the app in the analysis screen
   - status: open
