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
// is showing. Off is a picture OmniGear draws, dark or a screensaver, so
// whether the screen is off is OmniGear's belief, and the timeout and the
// screensaver are OmniGear's own settings. All are kept in files.
//
// The belief has two writers, `set` and `watch`. Each writes the whole file
// and each write records something that just happened to the screen, so the
// last one written is the truth and nothing needs locking.

type screenTimeout struct {
	slug, label string
	after       time.Duration
}

// screenTimeouts are the timeouts offered, by setting slug.
var screenTimeouts = []screenTimeout{
	{"never", "Never", 0},
	{"1-min", "1 Min", time.Minute},
	{"5-min", "5 Min", 5 * time.Minute},
	{"10-min", "10 Min", 10 * time.Minute},
	{"15-min", "15 Min", 15 * time.Minute},
	{"30-min", "30 Min", 30 * time.Minute},
	{"1-hour", "1 Hour", time.Hour},
}

// screenConfig is the timeout and the screensaver, kept with the rest of
// OmniGear's settings.
type screenConfig struct {
	Timeout string `json:"timeout"`
	Saver   string `json:"saver"`
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

// readScreenConfig is never and blank, until others are chosen. A file from
// before there were screensavers has only the timeout.
func readScreenConfig() (screenConfig, error) {
	config := screenConfig{Timeout: screenTimeouts[0].slug, Saver: screenSavers[0].slug}
	return config, readJSON(screenConfigPath(), &config)
}

func findScreenTimeout(slug string) (screenTimeout, bool) {
	for _, t := range screenTimeouts {
		if t.slug == slug {
			return t, true
		}
	}
	return screenTimeout{}, false
}

func (c screenConfig) timeout() (screenTimeout, error) {
	if t, ok := findScreenTimeout(c.Timeout); ok {
		return t, nil
	}
	return screenTimeout{}, fmt.Errorf("%s: unknown screen timeout %q", screenConfigPath(), c.Timeout)
}

func (c screenConfig) saver() (*screenSaver, error) {
	if s, ok := findScreenSaver(c.Saver); ok {
		return s, nil
	}
	return nil, fmt.Errorf("%s: unknown screensaver %q", screenConfigPath(), c.Saver)
}

// readScreenTimeout is the timeout's slug and length.
func readScreenTimeout() (string, time.Duration, error) {
	config, err := readScreenConfig()
	if err != nil {
		return "", 0, err
	}
	t, err := config.timeout()
	return t.slug, t.after, err
}

func writeScreenTimeout(slug string) error {
	if _, ok := findScreenTimeout(slug); !ok {
		return fmt.Errorf("no screen timeout %q", slug)
	}
	return updateScreenConfig(func(c *screenConfig) { c.Timeout = slug })
}

func writeScreenSaver(slug string) error {
	if _, ok := findScreenSaver(slug); !ok {
		return fmt.Errorf("no screensaver %q", slug)
	}
	return updateScreenConfig(func(c *screenConfig) { c.Saver = slug })
}

// updateScreenConfig changes one setting and keeps the other. A file that
// cannot be read is written over, since choosing a setting is how to put it
// right.
func updateScreenConfig(change func(*screenConfig)) error {
	config, _ := readScreenConfig()
	change(&config)
	return writeJSON(screenConfigPath(), config)
}

// readScreenBelief is on, until OmniGear has turned the screen off.
func readScreenBelief() (screenBelief, error) {
	var belief screenBelief
	return belief, readJSON(screenBeliefPath(), &belief)
}

func recordScreen(off bool) error {
	return writeJSON(screenBeliefPath(), screenBelief{Off: off, At: time.Now()})
}

// readScreen fills in what it can: a bad setting still leaves the switch and
// the other setting.
func readScreen() (*model.Screen, error) {
	belief, beliefErr := readScreenBelief()
	screen := &model.Screen{On: !belief.Off}
	config, err := readScreenConfig()
	if err != nil {
		return screen, errors.Join(beliefErr, err)
	}
	timeout, timeoutErr := config.timeout()
	if timeoutErr == nil {
		screen.Timeout = &model.Choice{Current: timeout.slug}
		for _, t := range screenTimeouts {
			screen.Timeout.Options = append(screen.Timeout.Options, model.ChoiceOption{Slug: t.slug, Label: t.label})
		}
	}
	saver, saverErr := config.saver()
	if saverErr == nil {
		screen.Saver = &model.Choice{Current: saver.slug}
		for _, s := range screenSavers {
			screen.Saver.Options = append(screen.Saver.Options, model.ChoiceOption{Slug: s.slug, Label: s.label})
		}
	}
	return screen, errors.Join(beliefErr, timeoutErr, saverErr)
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

const (
	// screenRecheck bounds how long watch takes to notice a timeout, or a
	// switch or a screensaver set by another process.
	screenRecheck = 5 * time.Second
	// saverFrame is 25 frames a second. A frame takes 36 ms to send, so
	// this keeps the link about nine tenths busy.
	saverFrame = 40 * time.Millisecond
	// saverRecheck bounds how long a moving screensaver takes to notice the
	// panel turning the screen on or choosing another, without rereading
	// the files every frame.
	saverRecheck = 500 * time.Millisecond
)

// screenWatch is watch's half of the screen: it counts down from the last
// time the base station was touched, turns the screen off at the timeout,
// and moves the screensaver for as long as the screen is off.
//
// Touching the base station is known from the reports it sends: 07 25 for
// each step of the volume knob, 07 45 for the ChatMix dial, 07 bb for the
// mute button. Any report 07 counts as a touch, and as the screen being back,
// since touching the base station brings back its own screen. Pressing the
// knob without turning it brings it back too but sends nothing, so the
// belief stays off until the next report, and a moving screensaver covers
// the screen again at its next frame.
//
// Only watch moves a screensaver, since it is the one process that stays.
// `set screen off` draws the screen dark, and watch puts the screensaver on
// it when it next reads the belief.
type screenWatch struct {
	conn screenConn
	// touched is when the countdown last started.
	touched time.Time
	// timeout is the slug the countdown is running for. A new one starts it
	// again, so choosing a shorter one does not turn the screen off at once.
	timeout string
	// saver is the screensaver on the screen, nil unless one is moving.
	saver *screenSaver
	// started is when the saver's first frame was due. Frame n is due
	// n frames later, so the rate holds however long each takes to draw.
	started time.Time
	// checked is when the files were last read while the saver moves.
	checked time.Time
}

// screenConn is what screenWatch draws with: an *arctis.Conn.
type screenConn interface {
	Draw(*arctis.Frame) error
	Blank() error
	Unblank() error
}

// startScreenWatch starts the countdown on connecting, with the screen
// handed back. A new connection is usually a power-up, which draws the base
// station's own screen anyway, but a restarted bar is not: the last frame of
// a screensaver would stay up, still, with nothing left to move it.
func startScreenWatch(conn screenConn) *screenWatch {
	_ = conn.Unblank()
	_ = recordScreen(false)
	slug, _, _ := readScreenTimeout()
	return &screenWatch{conn: conn, touched: time.Now(), timeout: slug}
}

// touch is a report from the base station.
func (w *screenWatch) touch(now time.Time) error {
	w.touched = now
	if w.saver != nil {
		w.saver = nil
		// The base station has drawn its own screen already, but a frame
		// sent as it did could have covered it again. Handing the screen
		// back makes sure.
		if err := w.conn.Unblank(); err != nil {
			return err
		}
		_ = recordScreen(false)
		return nil
	}
	if belief, err := readScreenBelief(); err == nil && belief.Off {
		_ = recordScreen(false)
	}
	return nil
}

// tick draws the screensaver's next frame while one moves, and otherwise
// runs the countdown. It says how long until it should be called again.
func (w *screenWatch) tick(now time.Time) (time.Duration, error) {
	if w.saver != nil {
		return w.animate(now)
	}
	config, err := readScreenConfig()
	if err != nil {
		return screenRecheck, nil
	}
	timeout, err := config.timeout()
	if err != nil {
		return screenRecheck, nil
	}
	if timeout.slug != w.timeout {
		w.timeout, w.touched = timeout.slug, now
	}
	// A screensaver that cannot be read shows as Blank, which is what there
	// was before there were others.
	saver, err := config.saver()
	if err != nil {
		saver = &screenSavers[0]
	}
	belief, err := readScreenBelief()
	if err != nil {
		return screenRecheck, nil
	}
	if belief.Off {
		if saver.art == nil {
			return screenRecheck, nil
		}
		return w.start(now, saver)
	}
	if timeout.after == 0 {
		return screenRecheck, nil
	}
	// Turning the screen on from the panel is a touch too, and the belief is
	// the only way watch hears of it.
	if belief.At.After(w.touched) {
		w.touched = belief.At
	}
	if idle := now.Sub(w.touched); idle < timeout.after {
		return min(timeout.after-idle, screenRecheck), nil
	}
	wait := screenRecheck
	if saver.art != nil {
		wait, err = w.start(now, saver)
	} else {
		err = w.conn.Blank()
	}
	if err != nil {
		return 0, err
	}
	_ = recordScreen(true)
	return wait, nil
}

// start puts a screensaver on the screen, from its first frame.
func (w *screenWatch) start(now time.Time, saver *screenSaver) (time.Duration, error) {
	w.saver, w.started, w.checked = saver, now, now
	return w.draw(now)
}

// animate draws the next frame, unless the screen has been turned on or
// another screensaver chosen since the files were last read.
func (w *screenWatch) animate(now time.Time) (time.Duration, error) {
	if now.Sub(w.checked) < saverRecheck {
		return w.draw(now)
	}
	w.checked = now
	if belief, err := readScreenBelief(); err == nil && !belief.Off {
		// `set screen on` handed the screen back already, but a frame drawn
		// since covers it again, and nothing would hand it back after.
		w.saver = nil
		return 0, w.conn.Unblank()
	}
	if config, err := readScreenConfig(); err == nil {
		if saver, err := config.saver(); err == nil && saver != w.saver {
			if saver.art == nil {
				w.saver = nil
				return screenRecheck, w.conn.Blank()
			}
			w.saver, w.started = saver, now
		}
	}
	return w.draw(now)
}

// draw sends the frame due now and says how long until the next one is due.
// A frame drawn late skips the ones it missed, so the art keeps its pace.
func (w *screenWatch) draw(now time.Time) (time.Duration, error) {
	n := int(now.Sub(w.started) / saverFrame)
	if err := w.conn.Draw(w.saver.frame(n)); err != nil {
		return 0, err
	}
	return w.started.Add(time.Duration(n+1) * saverFrame).Sub(now), nil
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
