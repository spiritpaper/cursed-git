# How cg and git share a repository

`cg` is two things at once: a game, and a small git written from scratch in Go. This page is
about the second part: what cg does itself, what it hands to git, and how git stores
everything on disk. It's also a decent tour of git's insides if you've always wondered what's
in `.git/`.

## The one rule

**Every byte cg writes inside `.git/` must be something real git would have written itself.**

That's what lets you mix the two freely: `cg commit`, then `git log`, then `cg status`, then
`git rebase`. Neither tool ever sees anything strange, because there's nothing strange to see.
Anything cg doesn't do itself goes straight to git, so the two never fight.

The test suite holds cg to it. Most tests build **twin repositories**, do the same thing in
both (git in one, cg in the other) and then compare everything: the files, the modes, the
index, the refs, the reflog, the hashes, the output. If cg's commit hash differs from git's
for the same input, that's a bug.

You can check by hand, too. After any cg command, `git fsck`, `git status`,
`git cat-file -p <hash>` and `git ls-files --stage` should all be perfectly happy.

## What cg does itself, and what it hands to git

| cg does it itself | Handed to git (↪) |
|---|---|
| `init`, `add`, `commit`, `status`, `diff`, `log` | `push`, `pull`, `fetch`, `clone`, `remote` |
| `branch`, `switch`, `checkout`, `restore`, `reset` | `merge`, `rebase`, `cherry-pick`, `revert`, `stash` |
| plumbing: `hash-object`, `cat-file`, `ls-files`, `write-tree`, `commit-tree`, `update-ref`, `symbolic-ref`, `rev-parse` | `rm`, `mv`, `tag`, `show`, `blame`, `bisect`, … everything else |

The rule for the left column: the common, everyday commands, and the ones that teach you
how git works. The right column is git's: networking, and the big history-rewriting tools,
which git already does perfectly.

Handing over is simple. cg runs `git` with exactly your arguments, connects it to your
terminal, and passes its exit code back. Any **option** cg doesn't know (say
`cg log --graph`) is handed over the same way, so you never hit a wall.

The game still runs around those commands. Miranda gets to look before and after
`cg merge` or `cg pull`, which is how a conflict from git turns into a boss fight. She
reacts to what git did; she never pretends to be git.

**Finishing a merge** is the one place where the two meet in the middle. Git does the merge
and leaves `MERGE_HEAD` and the conflict stages in the index (see below). cg's own `status`
and `commit` understand both, so you can resolve and finish with either tool.

---

## 1. Inside `.git`

```
.git/
├── HEAD              "ref: refs/heads/main\n": which branch you're on
├── config            settings, INI style (her [cursed] section lives here too)
├── description
├── index             the staging area (binary, see §4)
├── objects/          every file version, folder and commit, named by its hash
│   ├── 2a/c9f1...    loose objects: the first 2 hex characters are the folder
│   ├── info/
│   └── pack/         packfiles (git makes these on clone, fetch, pull and gc)
├── refs/
│   ├── heads/        one file per branch, holding a commit hash
│   ├── tags/
│   └── cursed/       hers: the graveyard, sacrificed commits, her heart (CURSES.md)
├── packed-refs       refs git has compacted into one file
├── logs/             the reflog: where every ref used to point
├── hooks/  info/
└── cursed/           her diary and state. Git ignores files it doesn't know.
```

Git finds the repository by starting in the current folder and walking up until it finds a
`.git`. cg does the same, which is why both work from any subfolder.

---

## 2. Objects

Everything git stores is one of four kinds of object: a **blob** (a file's contents), a
**tree** (a folder listing), a **commit**, or a **tag**.

### How an object is stored

```
<type> <size in bytes>\0<content>
```

1. Put that header in front of the content.
2. The **SHA-1 of the whole thing**, header included, is the object's name.
3. Compress the whole thing with zlib.
4. Save it as `.git/objects/<first 2 hex>/<other 38 hex>`.

Some values to check against:

| Input | Command | Hash |
|---|---|---|
| empty file | `git hash-object /dev/null` | `e69de29bb2d1d6434b8b29ae775ad8c2e48c5391` |
| `hello\n` | `echo hello \| git hash-object --stdin` | `ce013625030ba8dba906f756967f9e9ca394464a` |
| empty tree | `git hash-object -t tree /dev/null` | `4b825dc642cb6eb9a060e54bf8d69288fbee4904` |

The classic mistake is hashing just the content and forgetting the `blob 6\0` header.

Because the name *is* the content's hash, the same content is always the same object. Two
identical files anywhere in your history are stored once. (Miranda uses that, too: XP is paid
per blob, so re-committing the same content pays nothing.)

cg writes objects safely: to a temporary file in the same folder, then renamed into place.
If the object already exists, there's nothing to do; same hash, same content.

### Blob
The raw bytes of a file. No name, no permissions; those live in the tree.

### Tree
One folder, as a list of **binary** entries with nothing between them:

```
<mode in ASCII> <space> <name> \0 <20 raw SHA-1 bytes>
```

- Modes: `100644` a normal file, `100755` executable, `120000` a symlink, `40000` a folder
  (**no leading zero**).
- The hash is the **raw 20 bytes**, not the 40 hex characters.
- Entries are **sorted by name**, with a twist: folders sort as if their name ended in `/`.
  So the file `foo.txt` comes before the folder `foo`, because `.` (0x2E) sorts before `/`
  (0x2F). Get that wrong and your tree hashes won't match git's.
- A tree only describes one level. Each subfolder is its own tree object, so they're built
  from the bottom up.

`git cat-file -p <tree>` shows a tree nicely; `git cat-file tree <tree> | xxd` shows the real bytes.

### Commit
Plain text:

```
tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904
parent 3f1c...            (none for the first commit, two or more for a merge)
author Your Name <you@example.com> 1727712000 +0200
committer Your Name <you@example.com> 1727712000 +0200

The commit message
```

- The time is Unix seconds plus the time zone (`+0200`, not `+02:00`).
- Your name and email come from the repository's config, then `~/.gitconfig`, and the
  `GIT_AUTHOR_*` / `GIT_COMMITTER_*` environment variables beat both, as in git.
- The parent's hash is *part of the commit*. Change any commit and every commit after it gets
  a new hash. That's why Miranda's commit sacrifice changes the IDs of everything after the
  victim, and why rewriting pushed history causes so much trouble.

### Tag (annotated)
Like a commit, with `object`, `type`, `tag` and `tagger` headers. A lightweight tag is just a
ref (§3).

### Packfiles
When git fetches, clones or tidies up, it packs many objects into
`.git/objects/pack/*.pack` (with an `.idx` index) instead of keeping one file each. cg reads
loose objects itself, and for anything packed it asks git, through one long-running
`git cat-file --batch` process per repository (starting git once per object would make `cg log`
crawl). cg never needs to *write* packs: loose objects are always valid. Reading packs
directly is on the roadmap as a stretch goal.

---

## 3. Refs and HEAD

- **A branch is a file.** `.git/refs/heads/main` holds a commit hash and a newline. That's it.
  Creating a branch is creating a 41-byte file.
- **HEAD** usually holds `ref: refs/heads/main`: a *symbolic ref*, pointing at a branch.
- **Detached HEAD:** HEAD holds a commit hash directly instead of naming a branch.
- **Unborn branch:** in a brand-new repository HEAD points at `refs/heads/main`, but that file
  doesn't exist yet. The first commit creates it.
- **`packed-refs`:** git may move refs into this one file (lines of `<hash> <name>`). To find
  a ref, look for the loose file first, then in `packed-refs`.
- **Safe updates:** write `refs/heads/x.lock`, then rename it over `refs/heads/x`. That's git's
  own locking, so the two tools can't clobber each other.
- **The reflog** (`.git/logs/`) records where every ref used to point. cg writes it the way
  git does, so `git reflog` shows cg's commits. It's also how Miranda notices plain git
  behind her back: a reflog line she didn't write.

---

## 4. The index (the staging area)

`.git/index` is the list of what your next commit will contain: every tracked file, with its
blob hash and enough file-system details to notice quickly when a file changes. It's binary,
with **every number big-endian**.

### Header, 12 bytes
| Bytes | Value |
|---|---|
| 4 | the signature `DIRC` |
| 4 | the version, `2` |
| 4 | the number of entries |

### Each entry
| Bytes | Field |
|---|---|
| 4 + 4 | ctime: seconds, nanoseconds |
| 4 + 4 | mtime: seconds, nanoseconds |
| 4 | dev |
| 4 | inode |
| 4 | mode (`0o100644`, `0o100755`, `0o120000`) |
| 4 | uid |
| 4 | gid |
| 4 | file size |
| 20 | the blob's SHA-1 (raw bytes) |
| 2 | flags: bit 15 assume-valid, bit 14 extended (0 in v2), bits 13–12 the **stage**, bits 11–0 the name's length |
| n | the path, from the repository root, with `/`, no leading `./` |
| 1–8 | NUL bytes, so the **whole entry is a multiple of 8 bytes** (always at least one) |

That's 62 fixed bytes, then the path and the padding. cg fills the file-system fields from
the real file (Linux and macOS have their own code for it; other systems get a portable
version), because git uses them to decide whether a file needs re-hashing.

- Entries are sorted by path (byte order), then by stage.
- **The stage** is 0 normally. During a merge conflict the same path can appear three times:
  stage 1 is the common ancestor, 2 is yours, 3 is theirs. That's what "unmerged" means to
  git, and it's how Miranda detects a boss fight: by stages, not by looking for `=======` in
  your files (a file can legitimately contain that).
- **Extensions** may follow the entries (`TREE`, `REUC`, …): a 4-byte signature, a 4-byte
  size, then data. cg skips the ones it doesn't use.
- **The trailer** is the SHA-1 of everything above it.
- **Safe writes:** `.git/index.lock`, then rename. If the lock already exists, another git is
  running, and cg stops rather than guess.

`git ls-files --stage` and `git ls-files --debug` show you what's in it.

---

## 5. How `status` works

It compares three snapshots:

- **HEAD's tree vs the index** → "Changes to be committed"
- **The index vs your files** → "Changes not staged for commit". It compares the file-system
  details first (size, mtime) and only re-hashes a file when they differ. That's why
  `status` is fast even in a big project.
- **Files the index doesn't know** → "Untracked", minus whatever `.gitignore` says to ignore.

`log` walks from a commit to its parents, newest first. `diff` uses the Myers algorithm with
git's own clean-up pass, so its output matches `git diff` line for line.

---

## 6. Glossary

- **Porcelain / plumbing:** the friendly commands people type vs the small building blocks
  they're made of (`hash-object`, `write-tree`, `commit-tree`, `update-ref`). `cg commit` is
  literally the plumbing in a row.
- **Loose object:** one zlib-compressed file per object, in `objects/xx/`.
- **Merge base:** the newest commit two branches share. A merge compares both sides to it.
- **Unmerged path:** a path with index entries at stage 1, 2 or 3: a conflict.
- **Fast-forward:** a "merge" where your branch has nothing new, so it just moves forward.
  No merge commit, no conflict possible.

The animations `git-explained.html` and `git-internals.html` in `cursed-git-video/` show most
of this moving.
