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
