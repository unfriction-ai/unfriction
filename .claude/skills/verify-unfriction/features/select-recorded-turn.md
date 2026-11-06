# Select recorded work

`unfriction select`, run inside a git worktree, opens a terminal picker that discovers locally recorded Codex and Claude Code sessions whose recorded cwd belongs to any worktree of the current repository. The user searches by prompt, earlier context, branch, or path, moves through turns, and opens full context. Enter prints a quoted `unfriction inspect ... --turn ...` command to stdout without running it. Escape cancels.

## Sub-features

- `select-discover` loads both families and shows `Discovery complete · N sessions · M skipped`.
- `select-group` lists current-worktree sessions first and other worktrees after them.
- `select-search` filters on typed text, including ancestry, so `validation` matches the `Go` turn through earlier context.
- `select-navigate` moves between matches with Up and Down. Left and Right clear the search and move to the neighboring turn. Ctrl+U clears the search.
- `select-details` uses Tab to open `Full turn context` and Tab again to return.
- `select-accept` uses Enter to print one quoted inspect command (exit 0). The command works when run.
- `select-cancel` uses Escape or Ctrl+C to exit 130 with empty stdout.
- `select-format` uses `--format codex|claude-code` to limit discovery to one family.
- `select-output` emits literal zsh quoting with `--output zsh`. The default is `posix`.
- `select-empty` exits 1 with `no supported recordings found for this repository` when there is nothing to show.
- `select-no-search-match` shows `No search matches; Ctrl+U clears the search.`
- `select-changed-source` makes Enter after the file changes show `refresh and reselect`.

## How to get to it (user POV)

- Run `unfriction select` in a terminal inside a git worktree.
- Run `unfriction select --format codex`, `unfriction select --format claude-code`, or `unfriction select --output zsh`.

## Driving it with verify-unfriction

Preconditions:

- `$V/doctor.sh "$RUN_ID"` exits 0 and reports `isolated recordings: codex=2 claude=2`.
- Drives run through `tui.sh`, whose default cwd is the synthetic repo (current worktree).

- **Discover and search, then accept (Codex).** Run `$V/tui.sh "$RUN_ID" select-codex -expect "Discovery complete" -send validation -expect "Search: validation" -key down -expect "User: Go" -key enter -want-exit 0 -- unfriction select --format codex`. The run prints `ptydrive: OK exit=0 tty-restored=true`. `$E/select-codex/stdout.txt` is exactly one line, `unfriction inspect --format 'codex' --turn 'codex:second' -- '<world>/codex-home/sessions/2026/10/29/current.jsonl'`.
- **Run the emitted command.** Run `$V/emitted.sh "$RUN_ID" select-codex-inspect select-codex`. It exits `0`. Its stdout contains `Selected turn:`, `Task:` followed by `Go`, and `SELECTED_PAYLOAD`. It does not contain `EARLIER_PAYLOAD`.
- **Same for Claude.** Repeat with label `select-claude` and `-- unfriction select --format claude-code`. Stdout contains `--turn 'claude-code:second'` and `claude-home/projects/encoded-name-is-not-identity/current.jsonl`. Then run `$V/emitted.sh "$RUN_ID" select-claude-inspect select-claude`.
- **Both families.** Run `$V/tui.sh "$RUN_ID" select-all -expect "Discovery complete · 4 sessions" -key esc -want-exit 130 -- unfriction select`. Exit 130 proves that both families loaded (2 sessions each).
- **Current worktree first.** Move to the top row and accept. Run `$V/tui.sh "$RUN_ID" select-repo-top -expect "Discovery complete" -key up -key up -key up -key up -key enter -want-exit 0 -- unfriction select --format codex`. The emitted path ends in `current.jsonl` with `--turn 'codex:first'`.
- **From the other worktree.** Run `$V/tui.sh "$RUN_ID" select-other-top -C other -expect "Discovery complete" -key up -key up -key up -key up -key enter -want-exit 0 -- unfriction select --format codex`. The emitted path ends in `other.jsonl`, because that worktree is now the current one and its sessions come first.
- **Details view.** Run `$V/tui.sh "$RUN_ID" select-details -expect "Discovery complete" -key tab -expect "Full turn context" -key tab -expect-new "Discovery complete" -key esc -want-exit 130 -- unfriction select`. The detail frame lists `Session:`, `Source:`, `Turn:`, `Association:`.
- **No search match.** Run `$V/tui.sh "$RUN_ID" select-no-match -expect "Discovery complete" -send volcano -expect "No search matches" -key ctrl-u -expect-new "Search: " -key esc -want-exit 130 -- unfriction select`.
- **Cancel.** Run `$V/tui.sh "$RUN_ID" select-cancel -expect "Discovery complete" -key esc -want-exit 130 -- unfriction select`. `stdout.txt` is empty and `tty-restored.txt` is `true`. Repeat with `-key ctrl-c` (label `select-cancel-ctrlc`). It also exits 130.
- **zsh output.** Run `$V/tui.sh "$RUN_ID" select-zsh -expect "Discovery complete" -key up -key up -key up -key up -key enter -want-exit 0 -- unfriction select --format codex --output zsh`. Stdout holds one quoted command. Running it through `emitted.sh` succeeds.
- **Empty discovery.** Run `mkdir -p "$E/../../scratch/$RUN_ID/world/empty" && git init -q "$E/../../scratch/$RUN_ID/world/empty"`, then `$V/tui.sh "$RUN_ID" select-empty -C "$E/../../scratch/$RUN_ID/world/empty" -expect "no supported recordings" -want-exit 1 -- unfriction select`. Stdout is empty.
- **Proof.** For each label keep `steps.log`, `screen.txt`, `stdout.txt`, `exit.txt`, and `tty-restored.txt`. For accepts, also keep the `emitted.sh` label.

## Gotchas

- **The highlighted row right after discovery is not deterministic.** The cursor appears to stay on whichever entry loaded first while later sources sort above it. In the 2026-11-06 proof run, four identical drives that pressed Enter straight after `Discovery complete` emitted `current.jsonl` twice and `other.jsonl` twice. Before you accept, reach a known row by search (`-send validation -key down` gives `User: Go`) or by pressing `-key up` enough times to reach the top.
- `screen.txt` is the ANSI-stripped byte stream, not a rendered final frame. Unchanged lines are not redrawn, so you cannot read the `> ` cursor position or the full row order from it reliably. Prove selection by the emitted command instead.
- Type only after `Discovery complete` appears. Keys sent while summaries load can be lost or can apply to a partial list.
- Bubble Tea redraws only the lines that changed. Use `-expect` for text that may already be on screen, and `-expect-new` only for text that must be redrawn after the last key.
- The picker opens `/dev/tty`. Running `select` through `cli.sh` (no terminal) exits 2 by design. See `diagnostics.md`.
- The synthetic sessions have no `timestamp`, so rows show `unknown timestamp`. That is expected, not a failure.
- The changed-source path needs a write to `<world>/codex-home/sessions/2026/10/29/current.jsonl` while the picker is open, which `tui.sh` cannot do mid-drive. It is covered by `go test ./acceptance -run TestSelectTerminalOutcomes`. Report it as not driven through this harness.
- Never run the picker outside `tui.sh`. Without `CODEX_HOME` and `CLAUDE_CONFIG_DIR` it reads the user's real recordings.
