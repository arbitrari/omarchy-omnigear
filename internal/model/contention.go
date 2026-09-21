package model

import (
	"strings"

	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidraw"
)

// Contender is another program holding a device's hidraw node.
//
// This is not a warning about politeness. A hidraw node delivers every reply
// to every open reader, so a second HID++ client is not partitioned from
// OmniGear: its replies arrive interleaved with ours, and its writes can
// revert ours moments after they land. The symptom is a device that reads
// perfectly and will not keep a setting, which is unguessable from the
// outside — hence saying so plainly in the panel.
type Contender struct {
	PID int `json:"pid"`
	// Process is what the kernel calls it.
	Process string `json:"process"`
	// Label is a recognised program's proper name, or Process when it is not
	// one we know by sight.
	Label string `json:"label"`
	// Known separates "this is Solaar" from "something has this open". An
	// unknown holder is still worth reporting: it is still sharing the node.
	Known bool `json:"known"`
}

// contenderLabels are the programs known to drive the same peripherals this
// project does.
//
// Keys are matched against /proc/<pid>/comm lowercased. The kernel caps comm
// at 15 characters, so a longer program name is keyed by its truncation —
// openrazer-daemon arrives as "openrazer-daemo" and is spelled that way here
// on purpose.
var contenderLabels = map[string]string{
	"solaar":          "Solaar",
	"ratbagd":         "ratbagd (libratbag)",
	"piper":           "Piper",
	"logid":           "logiops",
	"logiops":         "logiops",
	"openrgb":         "OpenRGB",
	"ckb-next-daemon": "ckb-next",
	"openrazer-daemo": "OpenRazer",
	"polychromatic":   "Polychromatic",
	"razergenie":      "RazerGenie",
	"libratbag":       "libratbag",
}

// ownProcess is this project's own binary. A second OmniGear is the bar's poll
// overlapping a command run by hand: both are short-lived, neither reverts
// anything, and reporting it would be crying wolf about ourselves.
const ownProcess = "omnigear"

// Contenders describes the holders of one node, dropping our own processes.
func Contenders(holders []hidraw.Holder) []Contender {
	out := make([]Contender, 0, len(holders))
	for _, holder := range holders {
		name := strings.ToLower(strings.TrimSpace(holder.Name))
		if name == ownProcess {
			continue
		}
		label, known := contenderLabels[name]
		if !known {
			label = holder.Name
		}
		out = append(out, Contender{
			PID:     holder.PID,
			Process: holder.Name,
			Label:   label,
			Known:   known,
		})
	}
	return out
}
