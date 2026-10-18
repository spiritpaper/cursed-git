# devnote

Stuff that bit me while building Cursed GIT, so future me (and anyone reading this) doesn't step on the same rakes. Newest at the bottom of each section. Format is roughly: what happened → why → what I do now.

---

## Git internals

**My `add` wrote `main.go` into `.git/index`.**
I was testing `cg add main.go` *inside my own project folder*. The first version of `Add()` just read the file and wrote its bytes to `.git/index`. Real git then refused to do anything: `error: bad signature 0x6b636170 / fatal: index file corrupt`. That hex is literally "kcap", "pack" from `package main` read backwards as a number. Cute. Not cute.
→ Two lessons in one. The index is a binary format (`DIRC` header, big-endian, entries, SHA-1 trailer), not "the staged file". And **never run your own git clone inside the repo that holds its source code.** Every test now runs in `t.TempDir()`.

**I counted the index entry wrong.**
I had `StatData [62]byte` and then mode, hash and flags on top. 62 is the size of the *whole* fixed part: 10 stat fields × 4 bytes (40) + 20 bytes SHA-1 + 2 bytes flags. Stat data alone is 40.
→ Before writing a struct for a binary format, add the byte counts up on paper and compare with `xxd` output from a file real git made.

**Hashing just the content gives the wrong hash.**
Git hashes `blob <size>\0<content>`, not the content. Without the header, `hello\n` doesn't come out as `ce013625…`.
→ Keep a couple of known hashes in a test from day one (`e69de29…` for the empty blob, `4b825dc…` for the empty tree).

**Git's objects are read-only on purpose.**
Git writes object files with mode 0444. My first idea for "write the same object twice" was just to write it again, which fails on a read-only file. But you never need to: same hash = same content.
→ If `objects/xx/yyyy` exists, return early. Write new ones to a temp file in the same folder and `rename` it into place, so a crash halfway never leaves half an object that looks valid.

**Short hashes are a search, not a slice.**
`cg cat-file -p acca208` has to look in `objects/ac/` for files starting with `ca208`. Two matches means ambiguous, which is a real error, not "pick the first one".
→ At least 4 hex chars, check every char is hex, and test the ambiguous case on purpose.

**A tree stores raw bytes, and git prints it differently from how it stores it.**
In the tree the hash is 20 raw bytes (not 40 hex chars) and a folder's mode is `40000`. But `git cat-file -p` prints `040000`. If I'd compared my output with git by eye I'd never have noticed the missing zero.
→ Compare outputs byte for byte in tests (`out != want`), never "looks the same".

**The index padding is "1 to 8", not "0 to 7".**
Each index entry is padded with NULs to a multiple of 8, but there's always at least one NUL (it ends the path). So an entry that's already 64 bytes gets 8 more, not 0. The one-liner that gets it right: `(n + 8) &^ 7`.
→ Test with file names of 1 to 9 characters so every padding length shows up at least once.

**Don't trust the name length in the index flags.**
It's 12 bits, so it caps at 4095. A longer path stores `0xFFF` there and you have to find the end of the name by looking for the NUL.

**Old files vs fresh files: "racily clean".**
A file modified in the same second the index is written could change again without its mtime changing, so git treats those entries with suspicion. Git may change how it stores those entries, so an "is my index byte-identical to git's" test is only reliable if the files are clearly older than the index. I set the test files' mtimes an hour into the past.

**`.gitignore` is not a list of globs.**
`/cg` only matches at the root, `cg` matches at any depth, `node_modules/` only matches folders, `*` doesn't cross `/` but `**` does, `!` re-includes, and once a folder is ignored nothing inside it can come back. I wrote each rule down as a test case before writing the matcher.
→ The real oracle: run `git add .` and `cg add .` on two copies of the same tree and diff `ls-files -s`.

**Extensions are optional caches.**
After a commit git adds a `TREE` extension to the index. Reading: skip any extension you don't know (4-byte signature + 4-byte size). Writing: you're allowed to leave them out, and git rebuilds them.

**`index.lock` is the whole concurrency story.**
Create it with `O_EXCL` (fails if it exists), write, rename over `index`. If cg and git both try at once, one of them gets a clear "another process is running" error instead of a mangled index.

**The size in the header is free corruption detection.**
`blob 99\0short` says 99 bytes but has 5. Checking that on every read caught my own broken test objects.

**Folders sort as if their name ended in `/`.**
In a tree, `foo.txt` comes *before* the folder `foo`, because git compares `foo/` with `foo.txt` and `/` (0x2F) is bigger than `.` (0x2E). Sort plain names and your tree hash is wrong, but only when a file and a folder share a prefix, so it hides for a long time.
→ Test with `foo.txt`, `foo/bar`, `foo-bar`, `foo0` and compare with `git write-tree`.

**Tree modes have no leading zero, ls-tree output does.**
Stored: `40000`. Printed: `040000`. Same for writing: `fmt.Sprintf("%o", mode)` gives `100644` and `40000`, which is exactly what goes in the tree.

**Build trees bottom-up, and the index already groups folders for you.**
A folder's tree needs its subfolders' hashes first. Because the index is sorted by path, everything in `src/` sits next to each other, so one pass with recursion on each run of `src/...` entries is enough.

**An "unborn" branch is not an error.**
After `init`, HEAD says `ref: refs/heads/main` but that file doesn't exist. Resolving HEAD has to say "valid name, no commit yet", which is how the first commit knows it's a root commit (and has no parent line).

**Moving a branch is a compare-and-swap.**
Lock `refs/heads/main.lock`, read the current value *while holding the lock*, check it's still the commit you built on, then rename. If you read first and lock later, two commits at the same time can silently lose one of them.

**`git pack-refs` makes branches disappear from `refs/heads/`.**
They move into `.git/packed-refs`. A resolver that only reads `refs/heads/main` breaks the first time git decides to tidy up. Loose ref first, then packed-refs.

**A commit hash includes the clock.**
Same tree, same message, one second later = different commit. To reproduce the hashes from the video I pinned `GIT_AUTHOR_DATE` / `GIT_COMMITTER_DATE` (and name/email). The timezone is part of it too: `+0330` and `+0000` give different hashes for the same moment.

**"Nothing to commit" is one hash comparison.**
Build the tree from the index, compare its hash with the parent commit's tree hash. Equal = nothing changed. Content addressing strikes again.

**`git log` is not "follow the first parent".**
With merges, history is a graph. `git log` keeps a queue of commits to show and always prints the one with the newest *committer* date next, then queues its parents. Following only first parents would hide the whole merged-in branch.
→ A priority queue (`container/heap`) on committer date, with "found first" as the tie-breaker. Commits made in the same second are common in tests, so the tie-breaker matters.

**git indents blank lines in `log` too.**
Every message line gets 4 spaces in front, including empty ones (`"    \n"`). An editor that strips trailing whitespace from the test fixtures would break the comparison.

**`HEAD~2` and `HEAD^2` are different things.**
`~2` = go back two commits along first parents. `^2` = the second parent of a merge (the branch that got merged in). `HEAD~` and `HEAD^` both mean one step back. `^0` is the commit itself. They chain: `HEAD~1^2~1`.
→ Test a list of them against `git rev-parse` on a repo with a merge, including ones that must fail (`HEAD^3`, `HEAD~99`).

**After `git gc`, every object vanishes from `objects/xx/`.**
Git packs them into `objects/pack/*.pack`. Pulls and clones do the same. My loose-only reader would say "object not found" for things that obviously exist.
→ Until cg reads packfiles itself, fall back to `git cat-file --batch`, which takes a name on stdin and prints `<hash> <type> <size>` and the content. Read the content with `io.ReadFull`, because a single `Read` can return less than you asked for.

**Two GitHub accounts, one machine.**
This repo is published from a second account. The identity lives in the repo's own `.git/config` (`git config user.name` without `--global`), the push goes through an SSH host alias for the second key, and the reflog stores the committer name too. If a commit goes out with the wrong identity, fixing it means rewriting the commit *and* cleaning the reflog, so set the identity before the first commit.

**One git process per object was fine until it wasn't.**
The packed-object fallback started `git cat-file` for every object. Fine for `cat-file`, painful for `log` or ahead/behind on a long history (hundreds of processes).
→ Start `git cat-file --batch` once per repo and keep talking to it: write a name, read `<hash> <type> <size>`, then exactly size+1 bytes (the content plus a newline). If anything looks off, throw that process away and start a new one.

**"Racily clean" is real, not theory.**
Rewrite a file with the same size in the same second the index was written, and size + mtime look unchanged. Trusting the stat data would say "clean".
→ Only trust the stat shortcut when the file's mtime is older than the index file's. Otherwise hash it. The test fakes it with `os.Chtimes` back to the original time.

**`git status` has opinions about blank lines.**
Sections are separated by a blank line, except the first one right after a bare `On branch main` (no upstream line). "No commits yet" always gets one above it. I only found this by diffing against git; reading the output by eye I'd have sworn they were the same.

**Untracked folders are shown once, as `folder/`.**
If a folder has no tracked files, git prints `newdir/` instead of every file in it. And a folder containing only ignored files (build output) isn't shown at all, so you have to look inside before deciding.

**git status detects renames, cg doesn't (yet).**
`git mv a b` shows up in git as `R  a -> b`, and in cg as a delete plus an add. My tests compare against `git -c status.renames=false status` so they test what cg actually claims to do.

**Passthrough: send the whole command, not just the unknown part.**
First idea: unknown *commands* go to git. Better: if a command cg knows gets an option it doesn't (`commit -a`, `log --graph`, `rev-parse --abbrev-ref`), hand the *entire* command to git instead of erroring. Then cg never makes you switch to `git`.
→ Pass `os.Stdin` and the real `os.Stdout`/`os.Stderr` (not buffers) so git sees a terminal (colors, pager, editor all work), return git's exit code unchanged, and ignore Ctrl-C in cg while git runs so cg doesn't exit first.

## Her (the curse engine)

**`/dev/null` is a character device.**
"Am I at a terminal?" was `stdin.Mode() & ModeCharDevice != 0`. `go test` gives the test binary `/dev/null` as stdin, which *is* a character device, so she woke up inside my test suite and started refusing commands and rewriting commit messages. Very on brand. Very broken.
→ Ask the terminal driver (`ioctl TCGETS` on Linux, `TIOCGETA` on macOS), which is what `isatty` does. Only a real terminal answers.

**Tests must not depend on how they're started.**
Even with a correct check, `go test` from a real terminal *does* have a terminal on stdin. So a `TestMain` pins her to "quiet" for every test, and only the curse tests switch her on, with a fake clock and fixed luck.

**Inject the world: clock, luck, keyboard.**
A 30-minute deadline can't be tested by waiting 30 minutes. Everything she gets from outside (`Now`, `Roll`, `Pick`, stdin, stderr, `Sleep`) comes in through one `Env` struct. Tests move time with `h.wait(31 * time.Minute)` and set her luck to 0 (every random curse fires) or 0.99 (none do). I first tried a fake `math/rand.Source` and learned that `Intn` can loop forever on a constant source.

**Side effects only after the thing actually happened.**
She punished a commit that never happened: the message was rewritten and I paid toxicity, but the commit then failed because nothing was staged. Before the command, decide what to do; after it, *and only if it succeeded*, record it.

**A diary instead of a state file.**
Her state isn't stored, it's replayed from `events.log` every time. No state file to edit, no "state and log disagree" bugs, and the history explains every number in `cg mood`. Each line holds the SHA-1 of the line before (a hash chain), and the last hash also lives in a blob that `refs/cursed/heart` points at. Delete a line, edit one, or `rm -rf .git/cursed`, and she notices.

**A ref must point at a real object.**
My first idea was to put the raw chain hash in `refs/cursed/heart`. But `git fsck` checks that refs point at objects that exist. So the heart is a tiny blob containing the hash, and the ref points at the blob.

**"Never refuse twice in a row" needs memory, and most commands leave none.**
A "last command was refused" flag only resets when something new is written to the diary, which most commands never do. So after one refusal she could never refuse again. A time window (no second refusal within 10 minutes) says what I actually meant.

**The graveyard is just commits.**
Whatever she takes is saved as a normal commit under `refs/cursed/graveyard/<time>`. Refs keep objects alive, so git never garbage-collects them, and even without cg you can get files back with `git checkout <ref> -- <file>`. "Nothing is truly deleted" was a design rule from day one; here it's 40 lines of code.

**Break your own code to test your tests.**
Every curse test passed on the first run, which made me suspicious. So I broke the wipe on purpose (Tier 3 acting like Tier 2) and checked that the deadline test failed. It did. A test that can't fail is decoration.

**Her line pack must never break git.**
A typo in `lines/default.json` made `cg init` fail. Her dialogue is decoration: a broken pack now makes her mute, and git keeps working.

**`git gc` looks exactly like someone deleting your refs.**
I wanted her hook to block any change to `refs/cursed/*`. So I logged what the `reference-transaction` hook actually receives. `git pack-refs` (part of `gc`) moves every ref into `packed-refs`: a "create" in one transaction, then a "delete" of the loose file in another. And a real `git update-ref -d` just says `0000… 0000…`: no old value at all. Blocking deletions would have broken `git gc` for everyone.
→ The hook only blocks *forgery* (changing her ref to a different value). Deleting is allowed, and she notices afterwards. Hooks are an alarm, not a lock. Log what a hook really gets before writing rules for it.

**A hook that calls `os.Executable()` from a test calls the test binary.**
The hook script runs "the cg binary". In a test, `os.Executable()` is `cli.test`, so git would have run my whole test suite as a hook, from inside a test. Recursively.
→ Never point a hook at a `*.test` binary. `TestMain` builds a real `cg` once, and the hook tests use that, so they're true end-to-end tests: real git, a real hook, the real cg.

**The reflog is a free spy.**
To catch plain `git` used behind her back I almost built a whole ledger. But git already writes every HEAD move to `.git/logs/HEAD`, with a message ("reset: moving to HEAD~1"). She remembers how many lines it had when she last saw you; new lines that appeared while cg wasn't running happened without her, and the message names the crime. Git commands started *by* cg set `CG_VIA=1` so her hook stays quiet for those.

**Rot must be obvious, never sneaky.**
I wanted decay to swap look-alike characters (`o`→`0`). That's a real bug factory: invisible, compiles sometimes, and one `git add .` away from being committed. Rot is ░▒▓ only: obviously wrong, never compiles, easy to detect. Only *clean* files (exactly what's committed) ever rot, and a `pre-commit` hook refuses to commit it.

**Two questions, one function, one bug.**
"When was the last commit?" (a fact, for `cg mood`) and "when does the deadline clock start?" (her rule: from the last commit *or* from when she arrived) were the same function. `cg mood` said "last commit -9105h ago". Different questions get different functions, even when they look alike.

**Her stats come from the repo, not her memory.**
Commits made while she was quiet (from a script) never reached her diary, so `cg mood` said "no commits yet" in a repo with commits. That breaks her first rule. Facts come from git; her diary only holds her *feelings* about the facts.

**A silent fix loses the reaction.**
`cg add` restored rotted files before staging, silently. By the time you committed, there was nothing left to restore, so she never said "Fine. It lives." React where the thing happens.

**Never overwrite someone's hook.**
If `.git/hooks/pre-commit` exists and isn't hers (no marker comment), she leaves it alone. People's own hooks run their linters and tests.

**My boss-fight tests were too polite.**
The fight starts at Tier 3, and every test failed with "no fight". The setup made three commits with nice messages, three good commits in a row lowered her temper, and she was at Tier 2 by the time I merged. The game was right; my test had been *too good to her*.
→ Test fixtures use neutral messages ("song v1"): long enough not to be lazy, too short to count as good.

**Losing a fight started a new one.**
After a loss the conflict is still there, so the very next check saw "conflict, no fight, Tier 3" and started another. She remembers the last fight now, and a conflict on the same HEAD stays a normal conflict.

**A frozen clock buried two things in the same grave.**
Graveyard refs are named after the time in nanoseconds. In a test the clock doesn't move, so losing a fight and then `cg beg` (which saves what's on disk first) both wanted the same ref. Real clocks would almost never collide, but "almost" isn't how you keep someone's work. `Bury` now takes the next free name.

**She left before checking whether she was allowed to burn the house down.**
Ending B logged "she's gone" and *then* asked "is Death March on?". Being gone turns Death March off, so she never deleted anything, even when she should have. The test caught it on the first run. Decide first, then act.
→ When an action changes the state its own rules depend on, read the rules before the action.

**The test suite must not be able to shut down my laptop.**
The shutdown goes through a `Machine` interface, and `TestMain` installs a fake one for every test, not just the shutdown tests. Same for her data folder: archives and trophies go to a temp dir. A forgotten fake in one test would otherwise mean a real `systemctl poweroff` in the middle of `go test`.

**Winning calmed her down too early.**
Phase 2 (she pulls again after a Tier 4 win) never started: the win's temper −1 had already put her back at Tier 3 when she checked. Capture "the tier she fought at" before recording the win.

**Two engines, one command, one stale memory.**
`cg restraining-order` runs on its own engine; the outer one, created before it, still thinks she's on. Its end-of-command save would have written "she's on" right over the restraining order. Her memory is now written from a fresh replay of the diary, never from whatever an engine happened to load at the start.

**Her cross-project memory leaked between tests.**
Once she remembered projects, test B saw test A's repo: "You were in 001 while I waited here." Every test now gets its own data folder.

**The quest test found a bug in Phase 1.**
The README says "try `cg cat-file -p HEAD`". It never worked: cat-file only took hashes, and "HEAD" isn't one. Nobody noticed because every test used hashes. The quest test was the first to type it like a person would. cat-file now resolves revisions first, and passes git's richer syntax (`HEAD:file`, `main^{tree}`) to git.
→ Test the commands the docs tell people to type, exactly as written.

**CI on a Mac found what months on Linux couldn't.**
On macOS, cg filled the index's stat data with a "portable" fallback: ctime copied from mtime, no inode, no uid. `git status` re-hashes files whose stat data looks off, so it never showed. But `git diff-files` trusts the stat data, and said every file I'd staged was modified. Now macOS reads its real stat struct (`Ctimespec`, `Mtimespec`), like Linux does. The other macOS failure was my test: `Foo` and `foo` are the same file on a case-insensitive disk.
→ CI on every OS I claim to support, and make CI print the failures somewhere public: the logs need admin rights, annotations don't.

**On Windows, "is this a terminal?" is GetConsoleMode.**
The Unix check (an ioctl only a terminal answers) has a Windows twin: `GetConsoleMode` only succeeds on a real console. Her face and screen clearing use ANSI escapes, which old Windows consoles only understand after `SetConsoleMode(... ENABLE_VIRTUAL_TERMINAL_PROCESSING)`. Git Bash's mintty talks to programs through pipes, so there she can't tell it's a person, and stays quiet. Better quiet than cursing a script. (Written and cross-compiled on Linux, so it needs a real Windows test.)

**A pull's merge message names a URL, not the remote.**
`git pull` writes "Merge branch 'main' of /path/to/repo". To call the boss "Chad from origin" she looks the URL up in `remote.*.url`.

## Branches, checkout, diff

**Twin repos are the best test I've written.**
Build a history with git, copy the whole folder, then do the same operation with git in one copy and cg in the other. Compare *everything*: every file's content and permissions, the index, HEAD, branches, the reflog, stdout, the exit code. Every bug below was found this way, in an afternoon.

**Git doesn't pick permissions. Your umask does.**
cg wrote files with `0644`; git's came out `664`. Git creates files with `0666` (or `0777` for executables) and lets the umask remove bits. Hard-coding `0644` silently overrules the user's umask, which breaks shared folders on teams. Also fixed in her graveyard and decay code, which had the same habit.

**Checkout is validate-then-act.**
For every file that differs between the two commits: if you have local changes to it, refuse; if an untracked file is in the way, refuse. Only when *every* file passes does anything get written. A half-done checkout is worse than a refusal.

**Don't invent rules git doesn't have.**
I made "an untracked file in the way, but with identical content" allowed, because it seemed harmless. Git refuses anyway: it's not tracked, so it's not git's to overwrite. The twin test caught me.

**Git logs things that don't happen.**
`git switch main` while already on main still writes `checkout: moving from main to main` to the reflog. Byte-identical means copying the quirks too.

**`restore --source` also deletes.**
Restoring a folder from another commit makes it match that commit, so tracked files the commit doesn't have are removed. Untracked files are never touched.

**A file becoming a symlink isn't a mode change.**
`git diff` shows it as a deleted file plus a new one, because the content means something completely different.

**Myers finds *a* shortest diff, not *the* diff.**
`a b a` → `a`: delete lines 1–2 or 2–3? Both are 2 deletions. Git runs a compaction pass afterwards that slides each block of changes down as far as possible, then back up to line up with a change in the other file. With that, all my hand-picked cases match git byte for byte. On random repetitive input about 1 in 20 still differs, because git's divide-and-conquer Myers and its indent heuristic choose differently. The patch is still correct (`git apply` agrees), which is what matters.

**I rebuilt git's merge before re-reading what the project is.**
The roadmap said "three-way merge, diff3, stages", so I ported xdiff's merge, merge-base and the diffstat, matched git byte for byte… and then remembered cg is a *game on top of a real project*. Git already merges perfectly; the boss fight only needs to read the conflict git leaves behind (index stages + `MERGE_HEAD`). It all went back to passthrough. Two things survived because they're needed no matter who merges: `cg status` that understands conflicts, and `cg commit` that makes a two-parent commit when `MERGE_HEAD` exists. Without that last one, finishing a git merge with `cg commit` would have quietly made a normal commit and lost the merge.
→ Before building a big piece, ask: is this the game, or is it git? If it's git, pass it through.

## Go

**One folder = one package.**
I had `core/commands/*.go` and `core/util/*.go` both saying `package core` and then imported `cursedgit.com/core`, a folder with no Go files in it. Go doesn't merge folders into one package; the import path *is* the folder.
→ `internal/<thing>/` per package, import `cursedgit.com/internal/<thing>`.

**`fmt.Errorf(...)` on its own line does nothing.**
I wrote `fmt.Errorf("Init: failed to create ...")` thinking it reported the error. It just builds an error value and throws it away. `mkdir` could fail and `init` happily continued.
→ `return fmt.Errorf("...: %w", err)`. And run `go vet ./...`, which flags exactly this ("result of fmt.Errorf call not used").

**`cg init` with no folder crashed.**
`main.go` checked `len(args) > 1` for the target folder… and then right after did `if allArgs[1] == "."` without the check. `panic: index out of range [1] with length 1`.
→ Parse arguments once, in one place, with a default. Write a test for the no-argument case, because that's the one you never type while developing.

**Errors were printed, but the exit code was still 0.**
`init` on an existing repo or a read-only folder printed something (or nothing) and exited 0. Scripts, CI and `&&` chains all trust the exit code, not the text.
→ `main` is now one line: `os.Exit(cli.Run(args, stdout, stderr))`. `Run` returns the code instead of exiting, which also means tests can call it directly. Errors go to stderr.

**A test that runs `init` in "the current folder" ran it in my package folder.**
Go runs tests with the package directory as the working directory. A test for `cg init` with no arguments would have created `internal/cli/.git`. Same mistake as the index one, just sneakier.
→ `t.Chdir(t.TempDir())` (Go 1.24+) before testing anything that uses the current folder.

**`cg init -q` made a folder called `-q`.**
Anything after `init` was taken as the folder name, including flags. Found it by accident while typing the git way.
→ When parsing args by hand: things starting with `-` are options, and unknown options are errors, never paths.

**My test was wrong, not my code.**
I planted a fake object to test the "ambiguous short hash" error, and it didn't trigger. Turned out the fake file name was 36 chars instead of 38, so my resolver (correctly!) ignored it as not-an-object.
→ When a test fails, read the test as suspiciously as the code.

**Tests that run real git inherit my git config.**
My global `~/.gitconfig` (name, email, default branch, hooks…) leaks into tests that call `git`. Fine on my machine, different hashes on someone else's.
→ Set `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_SYSTEM=/dev/null` and fixed `GIT_AUTHOR_*` / `GIT_COMMITTER_*` env vars, and the commit hashes come out identical everywhere.

**The binary in the repo was older than the code.**
`./cg log` said "unknown command" right after I implemented `log` and all the tests passed. The tests build fresh code; `./cg` was the binary from the last phase.
→ `go build -o cg ./cmd/cg` after every change I want to try by hand, or just use `go run ./cmd/cg <command>`.

**`go install` names the binary after the folder.**
To publish, the module became `github.com/spiritpaper/cursed-git` (a made-up domain like `cursedgit.com` can't be `go install`ed unless the domain serves a redirect page). But with `main.go` at the root, `go install` would have produced a command called `cursed-git`. The fix everyone uses: `cmd/cg/main.go`, and the folder name becomes the binary name.

**Standard library first.**
In one sitting I hand-wrote an octal formatter and an insertion sort. `fmt.Sprintf("%o")` and `sort.Strings` were right there. (And later an `itoa`. `strconv.Itoa` has existed since Go 1.0.)

**My diff lied because my `sed` lied.**
Comparing `cg status` with `git status | sed 's/"git /"cg /'` showed one different line. The line had two `"git` in it, and `sed` without `g` only replaces the first. The tests use `strings.ReplaceAll`, so they were right all along.

**`gofmt` re-aligns struct fields.**
I add struct fields with find-and-replace scripts, and one failed because `gofmt` had changed the spacing in the struct since I last looked. Scripts that edit code should match loosely (a regex on `\s+`), or just use the editor.

**Clever code is a bug magnet.**
I wrote the config parser's escape handling as a one-liner with a map lookup and a bitwise OR. It probably worked, but I couldn't convince myself by reading it.
→ If I can't read a line in 5 seconds, it becomes a `switch`.

**Unused dependency hanging around.**
`go.mod` still required `gorilla/mux`, an HTTP router I never used.
→ `go mod tidy` before every commit. With zero dependencies it leaves an empty `go.sum`, which can just be deleted.

## Testing

**Let real git be the judge.**
The best test for "is this a valid repo" isn't my own checks, it's `git status` and `git fsck` run on the result. If real git is happy, I'm compatible. The tests call git through `exec.Command` and skip if git isn't installed.

## Her (the ASCII girl)

**Drawing ASCII art by hand: the right edge never lines up.**
My first two drafts of her had face rows two chars too narrow, and she looked like a robot in a helmet.
→ Draw only the left half and mirror it in code (swap `/`↔`\`, `(`↔`)`, `<`↔`>`). The face gets a fixed 6-char slot per mood. A test checks every mood has the same width and the frame is perfectly symmetric. Now I can redraw her without fear.

**`strings.NewReplacer` swaps in one pass.**
Swapping `/` and `\` with two `strings.ReplaceAll` calls in a row turns everything into one of them. `NewReplacer` does all pairs in a single pass, so swaps work.

## Animations / video tooling

**Headless screenshots lied about my CSS transitions.**
`chromium --headless --virtual-time-budget=…` advances JS timers but not CSS transitions, so screenshots showed elements frozen halfway. The page was fine.
→ For real checks, use Playwright with `page.clock` and step `document.getAnimations()` by hand (that's what `record.mjs` does).

**VT323 isn't as monospaced as I thought.**
I padded terminal rows with spaces to make columns, and they still didn't line up.
→ Use fixed-width `inline-block` spans for columns instead of space padding.

**Playwright has no Chromium build for Ubuntu 26.04.**
`npx playwright install chromium` just errors out.
→ Point it at the system one: `CHROMIUM_PATH=/snap/bin/chromium`. The tools read that variable.

**Background `&` in a shell one-liner dropped my `cd`.**
`cd dir && cmd1 & cmd2 &` runs the `cd` only for the first command, so the second wrote its output file into the project root.
→ Use absolute paths in anything that runs in the background.

---

## Things I learned (the short version)

- Git is a content-addressed key/value store with a few text files (refs, HEAD) on top. Once that clicked, everything else got easier.
- The index is a cache of the next commit plus file stat data, which is why `git status` is fast.
- In Go, return errors up and decide what to do with them in one place (the CLI layer). Don't print and continue.
- Make the program testable first (`Run(args, stdout, stderr) int`), then write the feature.
- When copying a format, check against the real thing at the byte level. Guessing costs more time than `xxd`.
- Content addressing is magic: dedup, integrity checks and cheap "does it exist" all fall out of naming things by their hash.
- Real git is the best test oracle I could ask for. Every test that says "git agrees" is worth ten that check my own assumptions.
- The toxic ex is a fun character to build and a terrible thing to want in real life. She stays in the terminal.
