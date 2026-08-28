// Package browser opens URLs in the user's web browser.
package browser

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Browser opens URLs.
type Browser interface {
	Browse(url string) error
}

type systemBrowser struct {
	launcher string
	stdout   io.Writer
	stderr   io.Writer
}

// New returns a Browser honouring $GV_BROWSER, then $BROWSER, then the
// platform default.
func New(launcher string, stdout, stderr io.Writer) Browser {
	if launcher == "" {
		launcher = os.Getenv("GV_BROWSER")
	}
	if launcher == "" {
		launcher = os.Getenv("BROWSER")
	}
	return &systemBrowser{launcher: launcher, stdout: stdout, stderr: stderr}
}

func (b *systemBrowser) Browse(url string) error {
	cmd, err := b.command(url)
	if err != nil {
		return err
	}
	cmd.Stdout = b.stdout
	cmd.Stderr = b.stderr
	return cmd.Run()
}

func (b *systemBrowser) command(url string) (*exec.Cmd, error) {
	if b.launcher != "" {
		fields := strings.Fields(b.launcher)
		exe, err := exec.LookPath(fields[0])
		if err != nil {
			return nil, err
		}
		return exec.Command(exe, append(fields[1:], url)...), nil
	}

	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{url}
	case "windows":
		name, args = "cmd", []string{"/c", "start", strings.ReplaceAll(url, "&", "^&")}
	default:
		name, args = "xdg-open", []string{url}
	}
	exe, err := exec.LookPath(name)
	if err != nil {
		return nil, fmt.Errorf("could not open browser: %w (set $GV_BROWSER)", err)
	}
	return exec.Command(exe, args...), nil
}

// Stub is a Browser that records the URLs it was asked to open.
type Stub struct {
	URLs []string
}

func (b *Stub) Browse(url string) error {
	b.URLs = append(b.URLs, url)
	return nil
}
