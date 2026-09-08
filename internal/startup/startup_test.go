package startup

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestEditorProcessReporting(t *testing.T) {
	for _, mode := range []string{"normal", "panic"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("STRONGDMM_REPORT_TEST", mode)
			file, err := os.Create(filepath.Join(t.TempDir(), "session.log"))
			if err != nil {
				t.Fatal(err)
			}
			code, processErr := runEditor(os.Args[0], []string{"-test.run=^TestReportChild$"}, file)
			file.Close()
			data, err := os.ReadFile(file.Name())
			if err != nil {
				t.Fatal(err)
			}
			output := string(data)
			if !strings.Contains(output, "editor output") || !strings.Contains(output, "editor error stream") {
				t.Fatalf("missing captured output: %s", output)
			}
			if mode == "normal" && (code != 0 || processErr != nil) {
				t.Fatalf("normal close reported as a crash: %d %v", code, processErr)
			}
			if mode == "panic" && (code == 0 || !strings.Contains(output, "panic: report test") || !strings.Contains(output, "goroutine")) {
				t.Fatalf("background panic was not retained: %d %s", code, output)
			}
		})
	}
}

func TestReportChild(t *testing.T) {
	mode := os.Getenv("STRONGDMM_REPORT_TEST")
	if mode == "" {
		return
	}
	fmt.Fprintln(os.Stdout, "editor output")
	fmt.Fprintln(os.Stderr, "editor error stream")
	if mode == "panic" {
		go func() { panic("report test") }()
		time.Sleep(5 * time.Second)
		os.Exit(3)
	}
	os.Exit(0)
}

func TestSessionLogFallback(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows profile fallback")
	}
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", blocked)
	tmp := t.TempDir()
	t.Setenv("TMP", tmp)
	t.Setenv("TEMP", tmp)
	t.Setenv("TMPDIR", tmp)
	file, err := sessionLog()
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	if !strings.HasPrefix(file.Name(), tmp+string(os.PathSeparator)) {
		t.Fatalf("expected temporary fallback, got %s", file.Name())
	}
}
