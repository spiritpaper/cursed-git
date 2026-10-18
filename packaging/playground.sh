#!/bin/bash
# The Docker image's front door. With arguments, it's just cg (in /work, if you mounted a
# project there). Without, a playground: a throwaway project with some history and a branch
# that's going to conflict with yours, Miranda already moved in, and a shell.
set -e

if [ $# -gt 0 ]; then
  [ -d /work ] && cd /work
  exec cg "$@"
fi

git config --global user.name "${CG_NAME:-player}"
git config --global user.email "${CG_EMAIL:-player@playground.local}"
git config --global init.defaultBranch main

mkdir -p ~/playground && cd ~/playground
if [ ! -d .git ]; then
  git init -q
  printf '# Our song\n\nverse one\nchorus\nverse two\n' > song.md
  printf 'package main\n\nfunc main() {}\n' > main.go
  git add . && git commit -qm "the song we started together"
  git switch -qc his-version
  sed -i 's/chorus/chorus (his version, obviously better)/' song.md
  git -c user.name=Chad -c user.email=chad@gym.example commit -qam "rewrote the chorus my way"
  git switch -q main
  sed -i 's/chorus/chorus (my version)/' song.md
  git commit -qam "the chorus the way I hear it"
  cg curse on --preset "${CG_PRESET:-psycho}"
fi

cat <<'MSG'

  Welcome to the Cursed GIT playground. Nothing here touches your machine.
  Use cg wherever you'd type git. Some things to try:

    cg status                 she has opinions
    cg commit -m "fix"        she has more opinions
    cg merge his-version      a boss fight (fix song.md, cg add, cg commit)
    cg quest                  learn git, for XP
    cg mood · cg card         how it's going between you two

  exit when you've had enough. (She'll remember.)

MSG
exec bash
