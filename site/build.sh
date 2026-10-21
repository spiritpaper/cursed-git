#!/bin/sh
# Gathers the docs website's source into one folder, for Jekyll to build:
#
#   site/build.sh _site_src      (from the repository root; the Pages workflow runs it)
#
# The Markdown files stay exactly as they are on GitHub. Each page gets its title and its
# place in the menu added on top here, and a few links are adjusted for the website.
set -eu

out="$1"
rm -rf "$out"
mkdir -p "$out/docs/lessons" "$out/docs/media" "$out/tools" "$out/cursed-git-video"
cp site/_config.yml "$out/"
cp -R site/_sass "$out/"
cp docs/media/* "$out/docs/media/"
cp tools/*.html tools/*.ini "$out/tools/"
cp cursed-git-video/*.html "$out/cursed-git-video/"

# page <source> <destination> <title> <menu position>
page() {
  {
    printf -- '---\ntitle: "%s"\nnav_order: %s\n---\n' "$3" "$4"
    printf '{%% raw %%}\n' # her lines are full of {braces}: Liquid must leave them alone
    # the README is the site's home page; LICENSE and the video tools stay on GitHub
    sed -e 's#](\.\./README\.md#](../#g' \
      -e 's#](LICENSE)#](https://github.com/spiritpaper/cursed-git/blob/main/LICENSE)#g' \
      -e 's#](cursed-git-video/README\.md)#](https://github.com/spiritpaper/cursed-git/tree/main/cursed-git-video)#g' "$1"
    printf '\n{%% endraw %%}\n'
  } >"$out/$2"
}

page README.md                         index.md                          "Home"                   1
page docs/PLAYING.md                   docs/PLAYING.md                   "How to play"            2
page docs/CURSES.md                    docs/CURSES.md                    "How the game works"     3
page docs/MIRANDA.md                   docs/MIRANDA.md                   "Miranda"                4
page docs/CHARACTERS.md                docs/CHARACTERS.md                "Characters"             5
page docs/lessons/writing-a-character.md docs/lessons/writing-a-character.md "Writing a character" 6
page docs/EVENTS.md                    docs/EVENTS.md                    "Running an event"       7
page docs/GIT.md                       docs/GIT.md                       "cg and git"             8
page site/animations.md                cursed-git-video/index.md         "Animations"             9
page CONTRIBUTING.md                   CONTRIBUTING.md                   "Contributing"           10
page CODE_OF_CONDUCT.md                CODE_OF_CONDUCT.md                "Code of conduct"        11
page docs/ROADMAP.md                   docs/ROADMAP.md                   "Roadmap"                12
page DEVNOTE.md                        DEVNOTE.md                        "Dev notes"              13

echo "site source in $out"
