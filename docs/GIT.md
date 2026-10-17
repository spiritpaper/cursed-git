# Cursed GIT — The Git Part

This document describes how `cg` stores data so that **real `git` and `cg` can work on the same repository without conflict**. That is the one hard rule: every byte `cg` writes inside `.git/` must be something real git would have written itself. If you follow it, anything `cg` doesn't implement (push, pull, rebase, …) can be handed off to `git`, and the two tools never fight.

> **How to check your work:** after any `cg` operation, run `git fsck`, `git status`, `git cat-file -p <hash>` and `git ls-files --stage`. If git is happy, you're compatible. If your hashes differ from git's for the same input, you have a bug.

---

## 1. The `.git` directory

```
.git/
├── HEAD              "ref: refs/heads/master\n"  (a symbolic ref)
├── config            INI-style settings
├── description
├── index             the staging area (binary, see §4)
├── objects/          every blob/tree/commit, content-addressed
│   ├── 2a/c9f1...    loose objects: first 2 hex chars = folder
│   ├── info/
│   └── pack/         packfiles (git creates these on gc/clone/pull)
├── refs/
│   ├── heads/        one file per branch, contains a commit hash
│   └── tags/
├── packed-refs       (optional) refs git has compacted into one file
├── hooks/  info/
└── cursed/           ← cg-only state (see CURSES.md). Git ignores unknown files here.
```

**Finding the repo:** start at the current directory and walk up until you find a `.git` folder. Every command except `init` needs this. Real git does the same, which is why `git status` works from a subfolder.

---

## 2. Objects

Everything git stores is one of four object types: **blob** (file contents), **tree** (directory listing), **commit**, **tag**.

### Storage format

```
<type> <size-in-bytes>\0<raw content>
```

1. Build that header + content.
2. **SHA-1 of the whole thing** (header included) is the object's ID.
3. **zlib-compress** the whole thing.
4. Write it to `.git/objects/<first 2 hex>/<remaining 38 hex>`.

Test values, so you can check your hash function:

| Input | Command | Hash |
|---|---|---|
| empty file | `git hash-object /dev/null` | `e69de29bb2d1d6434b8b29ae775ad8c2e48c5391` |
| `hello\n` | `echo hello \| git hash-object --stdin` | `ce013625030ba8dba906f756967f9e9ca394464a` |
| empty tree | `git hash-object -t tree /dev/null` | `4b825dc642cb6eb9a060e54bf8d69288fbee4904` |

Common mistake: hashing just the content and forgetting the `blob 6\0` header.

Go packages: `crypto/sha1`, `compress/zlib`, `encoding/hex`.

Write objects safely by writing to a temp file in the same folder and then `os.Rename`ing it into place. If the object already exists, skip it: same hash means same content.

### Blob
The content is the raw file bytes. There's no filename and no permissions. Two identical files anywhere in history are the same blob.

### Tree
The content is a sequence of **binary** entries, with no newlines between them:

```
<mode as ASCII> <space> <name> \0 <20 raw SHA-1 bytes>
```

- Modes: `100644` normal file, `100755` executable, `120000` symlink, `40000` subdirectory (**no leading zero**).
- The hash is the **raw 20 bytes**, not the 40-character hex string.
- Entries are **sorted by name**. The tricky rule is that directories sort as if their name ended in `/`. So `foo.txt` comes before the directory `foo`, because `.` (0x2E) sorts before `/` (0x2F). If you get this wrong, your tree hashes won't match git's.
- A tree only covers one directory level. Subdirectories are separate tree objects, so you build them **bottom-up**.

`git cat-file -p <tree>` pretty-prints a tree. `git cat-file tree <tree> | xxd` shows the raw bytes.

### Commit
Plain text:

```
tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904
parent 3f1c...            (0 lines for first commit, 2+ for merges)
author Your Name <you@example.com> 1727712000 +0200
committer Your Name <you@example.com> 1727712000 +0200

Commit message here
```

- The timestamp is Unix seconds followed by the timezone offset (`+0200`, not `+02:00`).
- Name and email come from `.git/config`, falling back to `~/.gitconfig` (`[user] name = …`).
- To test exactly, set `GIT_AUTHOR_DATE` / `GIT_COMMITTER_DATE` so that git and cg produce **identical** hashes.

### Tag (annotated)
Rarely needed. Same idea as a commit, with `object`, `type`, `tag`, `tagger` headers. Lightweight tags are just refs (§3).

### Packfiles, the compatibility trap
When you hand `pull`, `fetch` or `clone` off to real git, git will usually store the new objects in `.git/objects/pack/*.pack` (with a `.idx`) instead of as loose files. **Your object reader will then fail to find objects that definitely exist.** You have two options:
1. **Early:** if a loose object isn't found, fall back to `git cat-file -p <hash>`.
2. **Later (great video episode):** implement a pack reader: `.idx` lookup, object headers, zlib streams, and OFS_DELTA/REF_DELTA reconstruction.

You never need to *write* packs. Loose objects are always valid.

---

## 3. Refs and HEAD

- A branch is a file: `.git/refs/heads/master` contains `<40 hex>\n`.
- `HEAD` normally contains `ref: refs/heads/master\n`, which is a *symbolic ref*.
- **Detached HEAD:** `HEAD` holds a raw hash instead.
- **Unborn branch:** a fresh repo's HEAD points at `refs/heads/master`, but that file doesn't exist yet. The first commit creates it.
- `packed-refs`: git may move refs into this single file (lines of `<hash> <refname>`). When resolving a ref, check the loose file first, then `packed-refs`.
- To update a ref safely, write `refs/heads/x.lock` and then rename it over `refs/heads/x`. That's the same locking git uses, so the two tools won't clobber each other.
- Optional: append to `.git/logs/HEAD` (the reflog) so `git reflog` shows your commits. Handy for curse recovery, see CURSES.md.

---

## 4. The index (staging area), `.git/index`

This is the hardest file to get right, and it's where `cg` currently breaks (see ROADMAP Phase 0). It's a **binary file, all integers big-endian** (`encoding/binary.BigEndian`).

### Header, 12 bytes
| Bytes | Value |
|---|---|
| 4 | signature `DIRC` |
| 4 | version, use `2` |
| 4 | number of entries |

### Each entry
| Bytes | Field |
|---|---|
| 4 | ctime seconds |
| 4 | ctime nanoseconds |
| 4 | mtime seconds |
| 4 | mtime nanoseconds |
| 4 | dev |
| 4 | inode |
| 4 | mode (`0o100644`, `0o100755`, `0o120000`) |
| 4 | uid |
| 4 | gid |
| 4 | file size |
| 20 | blob SHA-1 (raw bytes) |
| 2 | flags: bit 15 assume-valid, bit 14 extended (0 in v2), bits 13–12 **stage**, bits 11–0 name length (max `0xFFF`) |
| n | path, relative to repo root, `/`-separated, no leading `./` |
| 1–8 | NUL padding so the **entry's total length is a multiple of 8** (always at least one NUL) |

That's 40 bytes of stat data plus 20 hash bytes plus 2 flag bytes, which makes **62 fixed bytes**, then the path and padding. Note that the current `IndexEntry` struct in `add.go` has `StatData [62]byte` *plus* mode, hash and flags, so it double-counts.

Get the stat fields from `os.Lstat(path)` and then `info.Sys().(*syscall.Stat_t)` on Linux.

- Entries are sorted by path (byte order), then by stage.
- **Stage** is 0 normally. During a merge conflict the same path appears up to three times: stage 1 = common ancestor, 2 = ours, 3 = theirs. This is how git knows a file is "unmerged".
- The file may contain **extensions** after the entries (`TREE`, `REUC`, …). Each is a 4-byte signature, a 4-byte size, then data. When reading, skip the ones you don't understand. When writing, you can leave them out.
- **Trailer:** the SHA-1 of everything above it, 20 raw bytes.
- To write safely, write `.git/index.lock` and then rename it to `.git/index`. If `index.lock` already exists, another git process is running, so refuse to continue.

Check your work with `git ls-files --stage` and `git ls-files --debug`.

---

## 5. Commands

Status legend: ✅ done · 🟡 started · ⬜ todo · ↪ passthrough to git

### Plumbing (build these first; porcelain is built on them)
| Command | Does | Status |
|---|---|---|
| `cg hash-object [-w] [-t type] [--stdin] <file>` | print blob hash, `-w` also stores it | ✅ |
| `cg cat-file -p\|-t\|-s\|-e\|<type> <hash>` | read and decompress an object (short hashes work) | ✅ |
| `cg ls-files [-s]` | print the index (relative to the current folder) | ✅ |
| `cg write-tree` | index → tree objects, prints root tree hash | ✅ |
| `cg commit-tree <tree> [-p parent] -m msg` | create a commit object | ✅ |
| `cg update-ref <ref> <hash> [<old>]` / `symbolic-ref` / `rev-parse <name>` | write and resolve refs (`HEAD~n` comes in Phase 4) | ✅ |

### Porcelain
| Command | Built from | Status |
|---|---|---|
| `init [-q] [dir]` | directories + HEAD/config/description | ✅ |
| `add [-f] <paths>` | hash-object -w + update index; folders, deletions, `.gitignore` | ✅ |
| `commit -m` | write-tree + commit-tree + update-ref HEAD | ✅ |
| `status [-s] [-b] [--porcelain]` | HEAD ↔ index ↔ working tree, untracked, ahead/behind | ✅ |
| `log [--oneline] [-n N] [<rev>...]` | walk parents, newest first; same output as `git log` | ✅ |
| `branch`, `switch`, `checkout` | refs + write tree to working dir + index; refuses to overwrite local work | ✅ |
| `diff [--cached] [<a> [<b>]]` | Myers diff + git's compaction, unified format | ✅ |
| `restore`, `reset --soft/--mixed/--hard` | index/working tree manipulation | ✅ (`rm` passes through to git) |
| `merge` | merge-base + 3-way merge + conflict stages | ⬜ (needed for boss fights) |
| `push`, `pull`, `fetch`, `clone`, `remote`, `rebase`, `stash`, … | ↪ `git` | ✅ passthrough |

### How `status` works
It compares three snapshots of the repo:
- **HEAD tree vs index** → "Changes to be committed"
- **index vs working tree** → "Changes not staged for commit". Compare the stat data first (mtime and size); only re-hash the file if the stat data differs.
- **files not in the index** → "Untracked". Respect `.gitignore`, which is a simple version first and full glob rules later.

### Passthrough
Anything `cg` doesn't recognise runs `git` with the same arguments:

```go
cmd := exec.Command("git", args...)
cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
err := cmd.Run()
// propagate the exit code: if exitErr, ok := err.(*exec.ExitError) → os.Exit(exitErr.ExitCode())
```

Curses still get to run before and after passthrough commands. For example, a `git pull` that *you* ran counts as "bringing someone home", see CURSES.md.

---

## 6. Glossary
- **Porcelain / plumbing:** user-facing commands vs low-level building blocks.
- **Loose object:** one zlib file per object in `objects/xx/`.
- **Merge base:** the most recent commit that two branches share.
- **Unmerged path:** a path that has index entries at stage 1, 2 or 3.
