// internal/ignore/ignore.go
//
// .gitignore matching. The rules, in the order that bites people:
//   - blank lines and lines starting with # are ignored; "\#" and "\!" escape them
//   - "!" in front re-includes something an earlier pattern excluded (last match wins)
//   - a trailing "/" means "only directories"
//   - a pattern with a "/" at the start or in the middle is anchored to the .gitignore's folder;
//     without one it matches a name at any depth ("*.mp4", "node_modules/")
//   - "*" and "?" never match "/", "**" does ("a/**/b", "**/logs", "build/**")
//   - once a folder is ignored, nothing inside it can be re-included
//
// Patterns come from .git/info/exclude and every .gitignore from the root down to the path.
package ignore

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type rule struct {
	base    string // folder the pattern came from, relative to the repo root ("" for the root)
	re      *regexp.Regexp
	negate  bool
	dirOnly bool
	byName  bool // no slash in the pattern: match the last path component only
}

// Matcher answers "is this path ignored?" for one working tree.
type Matcher struct {
	root   string
	rules  map[string][]rule // per folder; "" also holds .git/info/exclude
	loaded map[string]bool
}

func New(workTree string) *Matcher {
	m := &Matcher{root: workTree, rules: map[string][]rule{}, loaded: map[string]bool{}}
	m.rules[""] = parseFile(filepath.Join(workTree, ".git", "info", "exclude"), "")
	return m
}

func (m *Matcher) load(dir string) []rule {
	if !m.loaded[dir] {
		m.loaded[dir] = true
		m.rules[dir] = append(m.rules[dir], parseFile(filepath.Join(m.root, filepath.FromSlash(dir), ".gitignore"), dir)...)
	}
	return m.rules[dir]
}

// Ignored reports whether a repo-relative, slash-separated path is ignored,
// including because one of its parent folders is.
func (m *Matcher) Ignored(rel string, isDir bool) bool {
	parts := strings.Split(rel, "/")
	for i := 1; i < len(parts); i++ {
		if m.match(strings.Join(parts[:i], "/"), true) {
			return true
		}
	}
	return m.match(rel, isDir)
}

// match checks one path against the rules of every folder above it; the last matching rule wins.
func (m *Matcher) match(rel string, isDir bool) bool {
	dirs := []string{""}
	parts := strings.Split(rel, "/")
	for i := 1; i < len(parts); i++ {
		dirs = append(dirs, strings.Join(parts[:i], "/"))
	}
	ignored := false
	for _, d := range dirs {
		for _, r := range m.load(d) {
			if r.dirOnly && !isDir {
				continue
			}
			sub := rel
			if r.base != "" {
				sub = strings.TrimPrefix(rel, r.base+"/")
			}
			target := sub
			if r.byName {
				target = path.Base(sub)
			}
			if r.re.MatchString(target) {
				ignored = !r.negate
			}
		}
	}
	return ignored
}

func parseFile(file, base string) []rule {
	f, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer f.Close()
	var rules []rule
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if r, ok := parseLine(sc.Text(), base); ok {
			rules = append(rules, r)
		}
	}
	return rules
}

func parseLine(line, base string) (rule, bool) {
	line = strings.TrimRight(line, "\r")
	if !strings.HasSuffix(line, `\ `) { // trailing spaces are ignored unless escaped
		line = strings.TrimRight(line, " ")
	}
	if line == "" || strings.HasPrefix(line, "#") {
		return rule{}, false
	}
	r := rule{base: base}
	if strings.HasPrefix(line, "!") {
		r.negate = true
		line = line[1:]
	} else if strings.HasPrefix(line, `\!`) || strings.HasPrefix(line, `\#`) {
		line = line[1:]
	}
	if strings.HasSuffix(line, "/") {
		r.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	r.byName = !strings.Contains(line, "/")
	line = strings.TrimPrefix(line, "/")
	if line == "" {
		return rule{}, false
	}
	r.re = regexp.MustCompile("^" + globToRegexp(line) + "$")
	return r, true
}

func globToRegexp(g string) string {
	var b strings.Builder
	for i := 0; i < len(g); i++ {
		switch c := g[i]; {
		case strings.HasPrefix(g[i:], "**/"):
			b.WriteString("(?:.*/)?") // "**/x": x at any depth, including the top
			i += 2
		case strings.HasPrefix(g[i:], "/**") && i+3 == len(g):
			b.WriteString("/.*") // "x/**": everything inside x
			i += 2
		case strings.HasPrefix(g[i:], "**"):
			b.WriteString(".*")
			i++
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		case c == '[':
			if j := strings.IndexByte(g[i+1:], ']'); j >= 0 {
				class := g[i+1 : i+1+j]
				if strings.HasPrefix(class, "!") {
					class = "^" + class[1:]
				}
				b.WriteString("[" + strings.ReplaceAll(class, `\`, `\\`) + "]")
				i += j + 1
			} else {
				b.WriteString(`\[`)
			}
		case c == '\\' && i+1 < len(g):
			i++
			b.WriteString(regexp.QuoteMeta(string(g[i])))
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return b.String()
}
