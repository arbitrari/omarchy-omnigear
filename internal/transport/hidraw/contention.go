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
	return Holders(path)[path]
}

// Holders answers the same question as OtherHolders for several paths at once,
// walking /proc a single time.
//
// The walk is the expensive part — every pid, every open descriptor — and the
// bar asks this on every poll. Doing it once per device would multiply that by
// the number of devices for an answer that comes out of the same scan.
func Holders(paths ...string) map[string][]Holder {
	found := make(map[string][]Holder, len(paths))
	if len(paths) == 0 {
		return found
	}

	wanted := make(map[string]bool, len(paths))
	for _, path := range paths {
		wanted[path] = true
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return found
	}
	self := os.Getpid()

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == self {
			continue
		}
		held := heldPaths(pid, wanted)
		if len(held) == 0 {
			continue
		}
		// One name lookup per process, not per path it holds.
		holder := Holder{PID: pid, Name: processName(pid)}
		for _, path := range held {
			found[path] = append(found[path], holder)
		}
	}
	return found
}

// heldPaths returns which of the wanted paths this process has open.
func heldPaths(pid int, wanted map[string]bool) []string {
	fdDir := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	fds, err := os.ReadDir(fdDir)
	if err != nil {
		// Another user's process, or it exited while we looked. Not ours to see.
		return nil
	}
	var held []string
	seen := make(map[string]bool)
	for _, fd := range fds {
		target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
		if err != nil || !wanted[target] || seen[target] {
			continue
		}
		seen[target] = true
		held = append(held, target)
	}
	return held
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
