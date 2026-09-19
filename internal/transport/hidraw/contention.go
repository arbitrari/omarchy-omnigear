package hidraw

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Holder is another process with a hidraw node open.
type Holder struct {
	PID  int    `json:"pid"`
	Name string `json:"name"`
}

func (h Holder) String() string {
	return h.Name + " (pid " + strconv.Itoa(h.PID) + ")"
}

// OtherHolders lists the processes — this one aside — that currently have
// `path` open.
//
// This matters more than it looks. A hidraw node delivers every reply to every
// open reader, so a second HID++ client on the same device is not partitioned
// from us: we see its replies interleaved with ours, and its writes can revert
// ours moments after they land. Solaar, libratbag/Piper and other vendor
// plugins all do exactly this. A device that reads fine but will not keep a
// setting is the signature, and it is unguessable without this list.
//
// Only processes owned by the same user are visible; anything else is skipped
// silently, so an empty result means "nothing found", not "nothing there".
func OtherHolders(path string) []Holder {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	self := os.Getpid()
	var holders []Holder

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == self {
			continue
		}
		if holdsPath(pid, path) {
			holders = append(holders, Holder{PID: pid, Name: processName(pid)})
		}
	}
	return holders
}

func holdsPath(pid int, path string) bool {
	fdDir := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	fds, err := os.ReadDir(fdDir)
	if err != nil {
		// Another user's process, or it exited while we looked. Not ours to see.
		return false
	}
	for _, fd := range fds {
		if target, err := os.Readlink(filepath.Join(fdDir, fd.Name())); err == nil && target == path {
			return true
		}
	}
	return false
}

func processName(pid int) string {
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(raw))
}

// HolderList renders holders for an error message: "solaar (pid 1234)".
func HolderList(holders []Holder) string {
	names := make([]string, 0, len(holders))
	for _, h := range holders {
		names = append(names, h.String())
	}
	return strings.Join(names, ", ")
}
