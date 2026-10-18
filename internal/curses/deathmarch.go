// internal/curses/deathmarch.go
//
// Death March: the game mode where she's allowed to really hurt. Off unless YOU turn it on, by
// hand, in .git/config (no cg command does it for you):
//
//	[cursed]
//	    deathMarch = true
//
// Then, at a terminal, she shows what that means and waits for you to type the oath. Until you
// do, nothing changes. With Death March on (and accepted):
//
//	the shutdown is real         (Tier 4 punishments, breakup ending A)
//	ending B really deletes      (after a verified archive; `cg closure` brings it all back)
//
// Without it, the shutdown is an animation and ending B only writes the receipt.
// Removing the line, or `cg restraining-order`, ends it at once.
package curses

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spiritpaper/cursed-git/internal/config"
)

const deathMarchOath = "I accept the death march"

// DeathMarchConfigured: the player set cursed.deathMarch = true themselves.
func DeathMarchConfigured(gitDir string) bool {
	switch strings.ToLower(config.Load(gitDir).Get("cursed.deathMarch")) {
	case "true", "yes", "on", "1":
		return true
	}
	return false
}

// DeathMarch: configured AND accepted at the warning. Only then is anything real.
func (e *Engine) DeathMarch() bool {
	return e.st.On && e.st.DeathMarchAccepted && DeathMarchConfigured(e.gitDir)
}

// deathMarchGate runs before every command: show the warning once it's configured, and forget
// the oath when it's turned off (turning it on again means accepting again).
func (e *Engine) deathMarchGate() {
	configured := DeathMarchConfigured(e.gitDir)
	switch {
	case !configured && e.st.DeathMarchAccepted:
		e.log(Event{Type: "deathmarch_off"})
	case configured && !e.st.DeathMarchAccepted:
		out := e.env.Out
		fmt.Fprint(out, `
  ╔══════════════════════════════════════════════════════════════════════╗
  ║  ☠  DEATH MARCH                                                      ║
  ║                                                                      ║
  ║  You turned on cursed.deathMarch in this repository's .git/config.   ║
  ║  From now on she does what she threatens, for real:                  ║
  ║                                                                      ║
  ║   · At Tier 4 she can SHUT DOWN THIS COMPUTER, with no countdown.    ║
  ║     Unsaved work in other programs (editor, browser) is lost.        ║
  ║   · A toxic breakup DELETES THIS PROJECT FOLDER. It's archived and   ║
  ║     verified first, and `+"`cg closure`"+` in the empty folder restores it.   ║
  ║                                                                      ║
  ║  Play it somewhere safe: a hackathon machine, a throwaway project.   ║
  ║  To stop: remove the setting, or `+"`cg restraining-order`"+`.                ║
  ╚══════════════════════════════════════════════════════════════════════╝
`)
		fmt.Fprintf(out, "  Type %q to begin, anything else to keep playing without it:\n  > ", deathMarchOath)
		answer, _ := e.env.In.ReadString('\n')
		if strings.TrimSpace(answer) == deathMarchOath {
			e.log(Event{Type: "deathmarch_on"})
			e.say("deathmarch_on", nil)
		} else {
			fmt.Fprintln(out, "  Not yet, then. She'll ask again next time.")
		}
	}
}

// ---- the machine: everything outside git, behind an interface so tests never touch it ----

// Machine is what she can do to the computer itself.
type Machine interface {
	Shutdown() error    // power off, for real
	Lock() error        // lock the session
	DesktopDir() string // where the letter goes
}

// TheMachine is the real computer. Tests replace it.
var TheMachine Machine = realMachine{}

type realMachine struct{}

func (realMachine) Shutdown() error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("systemctl", "poweroff").Run()
	case "darwin":
		return exec.Command("osascript", "-e", `tell app "System Events" to shut down`).Run()
	case "windows":
		return exec.Command("shutdown", "/s", "/t", "0").Run()
	}
	return fmt.Errorf("no shutdown on %s", runtime.GOOS)
}

func (realMachine) Lock() error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("loginctl", "lock-session").Run()
	case "darwin":
		return exec.Command("pmset", "displaysleepnow").Run()
	}
	return fmt.Errorf("no lock on %s", runtime.GOOS)
}

func (realMachine) DesktopDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("USERPROFILE"), "Desktop")
	}
	if out, err := exec.Command("xdg-user-dir", "DESKTOP").Output(); err == nil {
		if d := strings.TrimSpace(string(out)); d != "" {
			return d
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop")
}

// shutdown: real in Death March (at most once a day), an animation otherwise.
func (e *Engine) shutdown(now time.Time, why string) {
	real := e.DeathMarch() && now.Unix()-e.st.LastShutdownAt > 24*3600
	e.log(Event{Type: "shutdown", Text: why})
	e.fakeShutdown()
	if !real {
		return
	}
	if err := TheMachine.Shutdown(); err != nil {
		TheMachine.Lock() // not allowed to power off: the next best thing
	}
}

// fakeShutdown: the screen goes black and she says goodbye.
func (e *Engine) fakeShutdown() {
	out := e.env.Out
	e.env.Sleep(time.Second)
	fmt.Fprint(out, "\033[2J\033[H") // clear the screen
	for _, s := range []string{"\n\n\n          Goodbye.", "\n\n          ...", "\n\n          (she turned off the lights)\n\n\n"} {
		e.env.Sleep(1200 * time.Millisecond)
		fmt.Fprint(out, s)
	}
}
