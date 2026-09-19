// Command omnigear is the hardware layer of the OmniGear Omarchy plugin.
//
// Every command prints one JSON object on stdout and nothing else, so the QML
// side can parse a whole reply without framing. Failures are JSON too: an
// {"ok": false, "error": …} object and a non-zero exit, never a bare panic.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/arbitrari/omarchy-omnigear/internal/catalog"
	"github.com/arbitrari/omarchy-omnigear/internal/discovery"
	"github.com/arbitrari/omarchy-omnigear/internal/model"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidpp"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidraw"
)

// version is the CLI's own version. Kept in step with manifest.json.
const version = "0.1.0"

// schema is bumped when the JSON shape changes in a way that would break a
// reader.
const schema = 1

const usage = `omnigear — read and write peripheral settings

USAGE:
    omnigear list                          every catalogued device present, with state
    omnigear get <device>                  one device, with state
    omnigear set <device> <key> <value>    change a setting:
                                             dpi <n> | polling-rate <hz>
                                             profile-mode onboard|host
    omnigear catalog                       the support matrix, hardware or not
    omnigear probe                         diagnostics: hidraw nodes and what answered
    omnigear call <device> <feature> <fn> [byte...]
                                           raw HID++ call, for driver development
    omnigear version

<device> is a device id from ` + "`list`" + `, or any unambiguous part of one:
    mouse/logitech/pro-x2-superstrike#5f-ba-c9-65
    mouse/logitech/pro-x2-superstrike
    pro-x2-superstrike
`

// reply is what every command returns: a JSON object, or an error to render as
// one.
type reply map[string]any

func main() {
	args := os.Args[1:]

	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(os.Stderr, usage)
		return
	}

	result, err := run(args)
	if err != nil {
		emit(reply{"ok": false, "error": err.Error()})
		os.Exit(1)
	}
	emit(result)
}

func run(args []string) (reply, error) {
	switch args[0] {
	case "version", "-V", "--version":
		return reply{"ok": true, "version": version}, nil
	case "list":
		return cmdList()
	case "get":
		if len(args) != 2 {
			return nil, fmt.Errorf("usage: omnigear get <device>")
		}
		return cmdGet(args[1])
	case "set":
		if len(args) != 4 {
			return nil, fmt.Errorf("usage: omnigear set <device> <key> <value>")
		}
		return cmdSet(args[1], args[2], args[3])
	case "catalog":
		return cmdCatalog()
	case "probe":
		return cmdProbe()
	case "call":
		if len(args) < 4 {
			return nil, fmt.Errorf("usage: omnigear call <device> <feature> <fn> [byte...]")
		}
		return cmdCall(args[1], args[2], args[3], args[4:])
	default:
		return nil, fmt.Errorf("unknown command %q — try `omnigear help`", args[0])
	}
}

func emit(value reply) {
	encoded, err := json.Marshal(value)
	if err != nil {
		fmt.Printf(`{"ok":false,"error":%q}`+"\n", err.Error())
		return
	}
	fmt.Println(string(encoded))
}

// --- commands --------------------------------------------------------------

func cmdList() (reply, error) {
	found := discovery.Devices()
	devices := make([]model.DeviceJSON, 0, len(found))
	for i := range found {
		devices = append(devices, readDevice(&found[i]))
	}
	return reply{"ok": true, "schema": schema, "devices": devices}, nil
}

func cmdGet(selector string) (reply, error) {
	found := discovery.Devices()
	device, err := catalog.Resolve(found, selector)
	if err != nil {
		return nil, err
	}
	return reply{"ok": true, "schema": schema, "device": readDevice(device)}, nil
}

// cmdSet applies a setting, then reads the device back and says what actually
// happened.
//
// A device can accept a write and quietly ignore it — a report rate stored in
// an onboard profile does exactly that. Reporting success on the strength of
// an un-refused write would make the UI lie, so the value is always verified
// against hardware before this returns.
func cmdSet(selector, key, value string) (reply, error) {
	setting, err := model.ParseSetting(key, value)
	if err != nil {
		return nil, err
	}

	found := discovery.Devices()
	device, err := catalog.Resolve(found, selector)
	if err != nil {
		return nil, err
	}
	if device.Entry.Driver == nil {
		return nil, fmt.Errorf("%s has no driver yet", device.Entry.Model)
	}
	driver := device.Entry.Driver

	beforeState := driver.Read(device)
	before, _ := beforeState.Reading(setting.Key)

	if err := driver.Write(device, setting); err != nil {
		return nil, err
	}

	afterState := driver.Read(device)
	after, known := afterState.Reading(setting.Key)

	switch {
	case known && after == setting.Value:
		return reply{
			"ok": true, "schema": schema,
			"applied": after,
			"device":  device.JSON(afterState),
		}, nil
	case known && after != before:
		// The device rounded to something it can actually do. That is a
		// success, but the caller should hear the real number.
		return reply{
			"ok": true, "schema": schema,
			"requested": setting.Value,
			"applied":   after,
			"note":      "device rounded the value",
			"device":    device.JSON(afterState),
		}, nil
	case known:
		message := fmt.Sprintf("device accepted the change but kept %d", after)
		// The most common cause is another HID++ client on the same node
		// undoing the write. Name it rather than leave the caller guessing.
		if holders := hidraw.OtherHolders(device.Node.Path); len(holders) > 0 {
			message += fmt.Sprintf(" — %s also has %s open, which can revert writes",
				hidraw.HolderList(holders), device.Node.Path)
		}
		return nil, fmt.Errorf("%s", message)
	default:
		return nil, fmt.Errorf("device accepted the change but will not report %s back", setting.Key)
	}
}

func cmdCatalog() (reply, error) {
	all := catalog.All()
	entries := make([]model.EntryJSON, 0, len(all))
	for i := range all {
		entries = append(entries, all[i].JSON())
	}
	return reply{"ok": true, "schema": schema, "catalog": entries}, nil
}

// nodeReport is one hidraw node as `probe` describes it.
type nodeReport struct {
	Path       string          `json:"path"`
	Vendor     string          `json:"vendor"`
	Product    string          `json:"product"`
	Name       string          `json:"name"`
	Driver     string          `json:"driver"`
	Uniq       string          `json:"uniq"`
	Link       hidraw.Link     `json:"link"`
	Connection string          `json:"connection"`
	OpenedBy   []hidraw.Holder `json:"openedBy"`
	Catalogued string          `json:"catalogued,omitempty"`
	DeviceIdx  *int            `json:"hidppDeviceIndex"`
	HIDPPName  string          `json:"hidppName,omitempty"`
	Features   []featureReport `json:"features"`
}

type featureReport struct {
	Feature string `json:"feature"`
	ID      string `json:"id"`
	Index   byte   `json:"index"`
}

// knownFeatures are probed by name so a `probe` reads like a checklist.
var knownFeatures = []struct {
	Label string
	ID    uint16
}{
	{"device-name", hidpp.FeatureDeviceName},
	{"unified-battery", hidpp.FeatureUnifiedBattery},
	{"battery-voltage", hidpp.FeatureBatteryVoltage},
	{"battery-status", hidpp.FeatureBatteryStatus},
	{"adjustable-dpi", hidpp.FeatureAdjustableDPI},
	{"extended-adjustable-dpi", hidpp.FeatureExtendedAdjustDPI},
	{"report-rate", hidpp.FeatureReportRate},
	{"extended-report-rate", hidpp.FeatureExtendedReportRate},
	{"onboard-profiles", hidpp.FeatureOnboardProfiles},
	{"hits", hidpp.FeatureHITS},
}

// cmdProbe lists every hidraw node, whether the catalog claims it, and for a
// claimed one which HID++ features the device actually implements — the
// fastest way to find out why a device is not showing up.
func cmdProbe() (reply, error) {
	nodes := hidraw.Enumerate()
	reports := make([]nodeReport, 0, len(nodes))

	for _, node := range nodes {
		report := nodeReport{
			Path:       node.Path,
			Vendor:     fmt.Sprintf("0x%04X", node.Vendor),
			Product:    fmt.Sprintf("0x%04X", node.Product),
			Name:       node.Name,
			Driver:     node.Driver,
			Uniq:       node.Uniq,
			Link:       node.Link,
			Connection: model.ConnectionOf(node).Label,
			OpenedBy:   hidraw.OtherHolders(node.Path),
			Features:   []featureReport{},
		}

		if entry := catalog.FindByUSB(node.Vendor, node.Product); entry != nil {
			report.Catalogued = entry.Model
			if link, err := hidpp.Open(node); err == nil {
				index := int(link.Index())
				report.DeviceIdx = &index
				if name, err := link.Name(); err == nil {
					report.HIDPPName = name
				}
				for _, feature := range knownFeatures {
					if featureIndex, err := link.FeatureIndex(feature.ID); err == nil {
						report.Features = append(report.Features, featureReport{
							Feature: feature.Label,
							ID:      fmt.Sprintf("0x%04X", feature.ID),
							Index:   featureIndex,
						})
					}
				}
				link.Close()
			}
		}

		reports = append(reports, report)
	}

	return reply{"ok": true, "schema": schema, "nodes": reports}, nil
}

// cmdCall sends one raw HID++ request, for working out a decoder against real
// hardware. Feature is a hex id (0x2202), function a decimal index, params hex
// bytes.
func cmdCall(selector, feature, function string, params []string) (reply, error) {
	featureID, err := strconv.ParseUint(strings.TrimPrefix(feature, "0x"), 16, 16)
	if err != nil {
		return nil, fmt.Errorf("%q is not a hex feature id", feature)
	}
	functionIndex, err := strconv.ParseUint(function, 10, 8)
	if err != nil {
		return nil, fmt.Errorf("%q is not a function index", function)
	}

	bytes := make([]byte, 0, len(params))
	for _, param := range params {
		b, err := strconv.ParseUint(strings.TrimPrefix(param, "0x"), 16, 8)
		if err != nil {
			return nil, fmt.Errorf("%q is not a hex byte", param)
		}
		bytes = append(bytes, byte(b))
	}

	found := discovery.Devices()
	device, err := catalog.Resolve(found, selector)
	if err != nil {
		return nil, err
	}

	link, err := hidpp.Open(device.Node)
	if err != nil {
		return nil, err
	}
	defer link.Close()

	index, err := link.FeatureIndex(uint16(featureID))
	if err != nil {
		return nil, err
	}
	result, err := link.Call(index, byte(functionIndex), bytes...)
	if err != nil {
		return nil, err
	}

	hex := make([]string, 0, len(result))
	for _, b := range result {
		hex = append(hex, fmt.Sprintf("%02X", b))
	}
	return reply{
		"ok":           true,
		"featureIndex": index,
		"hex":          strings.Join(hex, " "),
		"bytes":        result,
	}, nil
}

// --- shared ----------------------------------------------------------------

func readDevice(device *model.Device) model.DeviceJSON {
	if device.Entry.Driver == nil {
		state := model.NewDeviceState()
		state.Errors = append(state.Errors,
			fmt.Sprintf("%s is catalogued but not driven yet", device.Entry.Model))
		return device.JSON(state)
	}
	return device.JSON(device.Entry.Driver.Read(device))
}
