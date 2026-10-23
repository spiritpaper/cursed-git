package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtGallery(t *testing.T) {
	t.Setenv("CURSED_GIT_HOME", t.TempDir())
	t.Chdir(t.TempDir()) // no repository needed
	if out := mustRun(t, "art"); !strings.Contains(out, "miranda\n") {
		t.Errorf("Miranda's portrait ships with cg:\n%s", out)
	}
	if out := mustRun(t, "art", "miranda"); !strings.Contains(out, "⣿⣿⣿") {
		t.Errorf("cg art miranda should show her:\n%s", out)
	}

	// yours: from a file, cleaned of anything that talks to the terminal
	os.WriteFile("cat.txt", []byte("\n /\\_/\\  \n( o.o )\x1b[2J\r\n > ^ <\n\n"), 0644)
	mustRun(t, "art", "add", "Cat", "cat.txt")
	if out := mustRun(t, "art"); !strings.Contains(out, "cat  (yours)") {
		t.Errorf("your picture should be listed as yours:\n%s", out)
	}
	if out := mustRun(t, "art", "cat"); out != " /\\_/\\\n( o.o )[2J\n > ^ <\n" {
		t.Errorf("cg art cat = %q", out)
	}

	// yours covers the one in cg, and removing it brings hers back
	os.WriteFile("me.txt", []byte("not her\n"), 0644)
	mustRun(t, "art", "add", "miranda", "me.txt")
	if out := mustRun(t, "art", "miranda"); out != "not her\n" {
		t.Errorf("your miranda.txt should win: %q", out)
	}
	mustRun(t, "art", "rm", "miranda")
	if out := mustRun(t, "art", "miranda"); !strings.Contains(out, "⣿⣿⣿") {
		t.Errorf("after rm, the shipped portrait is back:\n%s", out)
	}

	for _, args := range [][]string{
		{"art", "nobody"},                      // no such picture
		{"art", "rm", "miranda"},               // the shipped one can't be removed
		{"art", "add", "../escape", "cat.txt"}, // names can't leave the art folder
		{"art", "add", "blank", os.DevNull},    // nothing in it
		{"art", "add"},                         // usage
	} {
		if code, _, _ := run(args...); code == 0 {
			t.Errorf("cg %v should fail", args)
		}
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("CURSED_GIT_HOME"), "escape.txt")); err == nil {
		t.Error("a bad name wrote a file")
	}
}

func TestPortraitAtInit(t *testing.T) {
	withHer(t, tuesday, lucky)
	t.Chdir(t.TempDir())
	_, _, errs := run("init", "--preset", "psycho")
	if !strings.Contains(errs, "⣿⣿⣿") {
		t.Errorf("she shows her portrait when she arrives:\n%s", errs)
	}
	if out := mustRun(t, "face"); strings.Contains(out, "⣿") || !strings.Contains(out, "^  ^") {
		t.Errorf("after that, cg face is the little face (no mood art yet):\n%s", out)
	}
}

func TestMoodArt(t *testing.T) {
	withHer(t, tuesday, lucky)
	newRepo(t, "psycho")
	for _, mood := range []string{"content", "unimpressed", "judging", "hostile", "furious", "love", "crying"} {
		os.WriteFile(mood+".txt", []byte("<"+mood+" art>\n"), 0644)
		mustRun(t, "art", "add", "miranda-"+mood, mood+".txt")
	}
	if out := mustRun(t, "face", "furious"); !strings.Contains(out, "<furious art>") || strings.Contains(out, ">  <") {
		t.Errorf("cg face furious shows her furious art:\n%s", out)
	}
	if _, out, _ := run("mood"); !strings.Contains(out, " art>") {
		t.Errorf("cg mood shows the art for how she feels:\n%s", out)
	}
	if _, out, _ := run("help"); strings.Contains(out, " art>") {
		t.Errorf("the help screen keeps the little face:\n%s", out)
	}
	mustRun(t, "art", "rm", "miranda-furious")
	if out := mustRun(t, "face", "furious"); !strings.Contains(out, ">  <") {
		t.Errorf("without the art, the little face is back:\n%s", out)
	}
}
