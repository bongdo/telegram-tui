// Package audio plays voice notes through a persistent mpv process controlled
// over its JSON IPC socket. mpv covers Ogg/Opus voice notes and the audio track
// of round video notes, and gives us pause/seek for free. The UI only needs
// wall-clock progress, so we do not subscribe to mpv property events.
package audio

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type State struct {
	Path     string
	Duration time.Duration
	Position time.Duration
	Progress float64
	Playing  bool
	Paused   bool
}

type Player struct {
	mu sync.Mutex

	cmd  *exec.Cmd
	conn net.Conn
	dir  string

	path        string
	duration    time.Duration
	startedAt   time.Time
	pausedAt    time.Time
	pausedTotal time.Duration
	playing     bool
	paused      bool
}

// New verifies that mpv is available. The process itself starts lazily so the
// TUI can open without spawning an audio player.
func New() (*Player, error) {
	if _, err := exec.LookPath("mpv"); err != nil {
		return nil, errors.New("mpv not found in PATH (install mpv for voice notes)")
	}
	return &Player{}, nil
}

func (p *Player) ensureLocked() error {
	if p.conn != nil {
		return nil
	}
	dir, err := os.MkdirTemp("", "tg-matrix-mpv-")
	if err != nil {
		return err
	}
	sock := filepath.Join(dir, "ipc.sock")
	cmd := exec.Command("mpv",
		"--idle=yes",
		"--no-video",
		"--no-terminal",
		"--really-quiet",
		"--force-window=no",
		"--input-ipc-server="+sock,
	)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return err
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			p.cmd = cmd
			p.conn = conn
			p.dir = dir
			go func() {
				sc := bufio.NewScanner(conn)
				for sc.Scan() {
					// Responses are not needed; drain so mpv never blocks.
				}
			}()
			return nil
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
			os.RemoveAll(dir)
			return fmt.Errorf("mpv IPC socket did not appear: %w", err)
		}
		time.Sleep(40 * time.Millisecond)
	}
}

func (p *Player) send(args ...any) error {
	req, err := json.Marshal(map[string]any{"command": args})
	if err != nil {
		return err
	}
	_, err = p.conn.Write(append(req, '\n'))
	return err
}

func (p *Player) resetLocked() {
	p.path = ""
	p.duration = 0
	p.playing = false
	p.paused = false
	p.startedAt = time.Time{}
	p.pausedAt = time.Time{}
	p.pausedTotal = 0
}

// Play replaces the current stream with path. duration may be zero, in which
// case progress is not reported.
func (p *Player) Play(path string, duration time.Duration) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.ensureLocked(); err != nil {
		return err
	}
	if err := p.send("loadfile", path, "replace"); err != nil {
		return err
	}
	_ = p.send("set", "pause", "no")
	p.path = path
	p.duration = duration
	p.startedAt = time.Now()
	p.pausedAt = time.Time{}
	p.pausedTotal = 0
	p.playing = true
	p.paused = false
	return nil
}

func (p *Player) TogglePause() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.playing || p.conn == nil {
		return nil
	}
	now := time.Now()
	if p.paused {
		p.pausedTotal += now.Sub(p.pausedAt)
		p.paused = false
	} else {
		p.pausedAt = now
		p.paused = true
	}
	return p.send("cycle", "pause")
}

// Seek takes a fraction of the duration, 0..1.
func (p *Player) Seek(fraction float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.playing || p.conn == nil || p.duration <= 0 {
		return nil
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	pos := time.Duration(float64(p.duration) * fraction)
	if err := p.send("seek", pos.Seconds(), "absolute"); err != nil {
		return err
	}
	now := time.Now()
	p.startedAt = now.Add(-pos - p.pausedTotal)
	if p.paused {
		p.pausedAt = now
	}
	return nil
}

func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		_ = p.send("stop")
	}
	p.resetLocked()
}

func (p *Player) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.playing {
		return State{}
	}
	elapsed := time.Since(p.startedAt) - p.pausedTotal
	if p.paused {
		elapsed = p.pausedAt.Sub(p.startedAt) - p.pausedTotal
	}
	if elapsed < 0 {
		elapsed = 0
	}
	if p.duration > 0 && elapsed >= p.duration {
		p.resetLocked()
		return State{}
	}
	progress := 0.0
	if p.duration > 0 {
		progress = float64(elapsed) / float64(p.duration)
	}
	return State{
		Path:     p.path,
		Duration: p.duration,
		Position: elapsed,
		Progress: progress,
		Playing:  true,
		Paused:   p.paused,
	}
}

func (p *Player) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		_ = p.send("quit")
		_ = p.conn.Close()
		p.conn = nil
	}
	if p.cmd != nil {
		_ = p.cmd.Process.Kill()
		_, _ = p.cmd.Process.Wait()
		p.cmd = nil
	}
	if p.dir != "" {
		_ = os.RemoveAll(p.dir)
		p.dir = ""
	}
	p.resetLocked()
}
