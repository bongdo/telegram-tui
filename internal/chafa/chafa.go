// Package chafa renders images as ANSI text through the chafa CLI. It is only
// used for the media viewer; normal chat rendering never shells out.
package chafa

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var ErrUnavailable = errors.New("chafa not found in PATH")

func Available() bool {
	_, err := exec.LookPath("chafa")
	return err == nil
}

// Render runs chafa for path and returns at most height lines, each at most
// width cells wide. chafa owns palette selection; the caller owns layout.
func Render(path string, width, height int) ([]string, error) {
	if !Available() {
		return nil, ErrUnavailable
	}
	if width < 2 || height < 2 {
		return nil, errors.New("image area is too small")
	}
	cmd := exec.Command("chafa",
		"--format=symbols",
		"--colors=256",
		fmt.Sprintf("--size=%dx%d", width, height),
		"--stretch",
		"--polite=on",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("chafa: %s", strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	text := strings.TrimRight(string(out), "\r\n")
	if text == "" {
		return nil, errors.New("chafa returned an empty image")
	}
	lines := strings.Split(text, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	return lines, nil
}
