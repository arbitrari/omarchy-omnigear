package steelseries

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/arbitrari/omarchy-omnigear/internal/model"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/arctis"
)

// The base station's screen has no off and no timer, and cannot say what it
// is showing. Off is a dark picture OmniGear draws, so whether the screen is
// off is OmniGear's belief, and the timeout is OmniGear's own setting. Both
// are kept in files.
//
// The belief has two writers, `set` and `watch`. Each writes the whole file
// and each write records something that just happened to the screen, so the
// last one written is the truth and nothing needs locking.

// screenTimeouts are the timeouts offered, by setting slug.
var screenTimeouts = []struct {
	slug, label string
	after       time.Duration
}{
	{"never", "Never", 0},
	{"1-min", "1 Min", time.Minute},
	{"5-min", "5 Min", 5 * time.Minute},
	{"10-min", "10 Min", 10 * time.Minute},
	{"15-min", "15 Min", 15 * time.Minute},
	{"30-min", "30 Min", 30 * time.Minute},
	{"1-hour", "1 Hour", time.Hour},
}

// screenConfig is the timeout, kept with the rest of OmniGear's settings.
type screenConfig struct {
	Timeout string `json:"timeout"`
}

// screenBelief is whether OmniGear last left the screen off, and when. It is
// kept only until logout: a base station that loses power draws its own
// screen when it comes back.
type screenBelief struct {
	Off bool      `json:"off"`
	At  time.Time `json:"at"`
}

func screenConfigPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "omnigear", "screen.json")
}

func screenBeliefPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), fmt.Sprintf("omnigear-%d", os.Getuid()))
	}
	return filepath.Join(dir, "omnigear", "screen.json")
}

// readScreenTimeout is the timeout's slug and length. Never, until one is
// chosen.
func readScreenTimeout() (string, time.Duration, error) {
	config := screenConfig{Timeout: screenTimeouts[0].slug}
	if err := readJSON(screenConfigPath(), &config); err != nil {
		return "", 0, err
	}
	for _, t := range screenTimeouts {
		if t.slug == config.Timeout {
			return t.slug, t.after, nil
		}
	}
	return "", 0, fmt.Errorf("%s: unknown screen timeout %q", screenConfigPath(), config.Timeout)
}

func writeScreenTimeout(slug string) error {
	for _, t := range screenTimeouts {
		if t.slug == slug {
			return writeJSON(screenConfigPath(), screenConfig{Timeout: slug})
		}
	}
	return fmt.Errorf("no screen timeout %q", slug)
}

// readScreenBelief is on, until OmniGear has turned the screen off.
func readScreenBelief() (screenBelief, error) {
	var belief screenBelief
	return belief, readJSON(screenBeliefPath(), &belief)
}

func recordScreen(off bool) error {
	return writeJSON(screenBeliefPath(), screenBelief{Off: off, At: time.Now()})
}

// readScreen fills in what it can: a bad timeout file still leaves the switch.
func readScreen() (*model.Screen, error) {
	belief, beliefErr := readScreenBelief()
	screen := &model.Screen{On: !belief.Off}
	slug, _, err := readScreenTimeout()
	if err == nil {
		screen.Timeout = &model.Choice{Current: slug}
		for _, t := range screenTimeouts {
			screen.Timeout.Options = append(screen.Timeout.Options, model.ChoiceOption{Slug: t.slug, Label: t.label})
		}
	}
	return screen, errors.Join(beliefErr, err)
}

// writeScreen turns the screen off or hands it back, and records which.
func writeScreen(conn *arctis.Conn, on bool) error {
	write := conn.Blank
	if on {
		write = conn.Unblank
	}
	if err := write(); err != nil {
		return err
	}
	return recordScreen(!on)
}

// readJSON leaves v as it is when the file does not exist.
func readJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// writeJSON replaces a file by renaming a finished copy over it, so a reader
// never sees half of one.
func writeJSON(path string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
