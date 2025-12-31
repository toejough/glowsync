# Issues

A lightweight issue tracking system for the project.

1. try to use SSH/SFTP to sync files
   - status: complete (implementation done, tests need updating)
   - started: 2025-12-30
   - completed: 2025-12-30
   - implementation: All core features implemented
     * SFTP connection management with SSH agent/key auth
     * URL parser for sftp://user@host:port/path format
     * Dual filesystem support (local-to-SFTP, SFTP-to-local, SFTP-to-SFTP)
     * Integration with sync engine and TUI
   - note: Some unit tests need updating to handle NewEngine error return
   - usage: glowsync -s sftp://user@host/remote/path -d /local/path
2. create a way to ignore files on the server side from deletion during sync
   - status: open
3. there's no border around the app in the analysis screen
   - status: open
