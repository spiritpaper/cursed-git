// internal/config/config.go
//
// Just enough of git's config to find out who you are: an INI-like format where
// keys are "section.key" (or "section.subsection.key"), case-insensitive, and later
// files override earlier ones: ~/.gitconfig, then .git/config.
package config

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Config map[string]string

// Load reads the global config and the repo's config. GIT_CONFIG_GLOBAL works like in git
// (tests set it to /dev/null so your personal settings don't leak in).
func Load(gitDir string) Config {
	c := Config{}
	global := os.Getenv("GIT_CONFIG_GLOBAL")
	if global == "" {
		if home, err := os.UserHomeDir(); err == nil {
			xdg := os.Getenv("XDG_CONFIG_HOME")
			if xdg == "" {
				xdg = filepath.Join(home, ".config")
			}
			c.readFile(filepath.Join(xdg, "git", "config"))
			global = filepath.Join(home, ".gitconfig")
		}
	}
	c.readFile(global)
	if gitDir != "" {
		c.readFile(filepath.Join(gitDir, "config"))
	}
	return c
}

func (c Config) Get(key string) string { return c[strings.ToLower(key)] }

// Parse reads config text in git's format (an event file, for example).
func Parse(text string) Config {
	c := Config{}
	c.read(strings.NewReader(text))
	return c
}

func (c Config) readFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	c.read(f)
}

func (c Config) read(r io.Reader) {
	section := ""
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		if line[0] == '[' {
			end := strings.IndexByte(line, ']')
			if end < 0 {
				continue
			}
			// [section] or [section "subsection"]: the section is case-insensitive, the subsection isn't
			name, sub, hasSub := strings.Cut(line[1:end], " ")
			section = strings.ToLower(name)
			if hasSub {
				section += "." + strings.Trim(strings.TrimSpace(sub), `"`)
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			value = "true" // a bare key means true
		}
		value = stripComment(strings.TrimSpace(value))
		c[section+"."+strings.ToLower(strings.TrimSpace(key))] = value
	}
}

// stripComment removes a trailing "# ..." or "; ..." outside quotes, and the quotes themselves.
func stripComment(v string) string {
	var b strings.Builder
	quoted := false
	for i := 0; i < len(v); i++ {
		switch c := v[i]; {
		case c == '"':
			quoted = !quoted
		case c == '\\' && i+1 < len(v):
			i++
			switch v[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(v[i]) // \" and \\ are just the character
			}
		case (c == '#' || c == ';') && !quoted:
			return strings.TrimSpace(b.String())
		default:
			b.WriteByte(c)
		}
	}
	return strings.TrimSpace(b.String())
}
