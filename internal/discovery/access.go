package discovery

import (
	"errors"
	"io/fs"
	"os"

	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidpp"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidraw"
)

// Unreadable lists nodes that carry a protocol this project speaks but cannot
// be opened for want of permission.
//
// This exists because the failure it describes is otherwise completely
// silent. hidraw nodes are root-only by default; a device the user cannot
// open simply never appears, and the plugin looks broken rather than
// unprivileged. It is the single most likely reason a freshly plugged mouse
// shows nothing at all.
//
// The fix is the udev rule in udev/60-omnigear.rules. Until it is installed —
// or after it is installed but before the device is replugged, because a node
// keeps the permissions it was given when it was added — this is what the
// panel reports instead of staying quiet.
func Unreadable() []string {
	var blocked []string
	for _, node := range hidraw.Enumerate() {
		if !hidpp.Speaks(node) {
			continue
		}
		file, err := os.OpenFile(node.Path, os.O_RDWR, 0)
		if err == nil {
			file.Close()
			continue
		}
		// Only permission is worth reporting. A node that is busy, or that
		// vanished between listing and opening, is not something the user can
		// act on and not something a udev rule would fix.
		if errors.Is(err, fs.ErrPermission) {
			blocked = append(blocked, node.Path)
		}
	}
	return blocked
}
