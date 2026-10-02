package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/arbitrari/omarchy-omnigear/internal/catalog"
	"github.com/arbitrari/omarchy-omnigear/internal/discovery"
	"github.com/arbitrari/omarchy-omnigear/internal/model"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidpp"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidraw"
)

// issueBase is where a request for a new device goes.
const issueBase = "https://github.com/arbitrari/omarchy-omnigear/issues/new"

// cmdReport writes up a device so somebody who does not own one can add
// support for it.
//
// Named with no device, it describes every hidraw node on the machine. That
// is the path for a mouse this project cannot even identify — a Razer, say,
// which speaks no protocol here and so never appears as a device at all. The
// dump is the only thing that helps in that case, and it is the same
// information a maintainer would ask for first.
//
// The body is deliberately everything a driver author needs and nothing about
// the person sending it: device ids, what answered, kernel and version. No
// hostname, no username, no serial.
func cmdReport(selector string) (reply, error) {
	var body strings.Builder
	var title string

	if selector == "" {
		title = "Device support request"
		writeMachine(&body)
		writeAllNodes(&body)
	} else {
		found := append(discovery.Devices(), discovery.Unknown()...)
		device, err := catalog.Resolve(found, selector)
		if err != nil {
			return nil, err
		}
		title = fmt.Sprintf("Device support request: %s", device.Entry.Model)
		writeMachine(&body)
		writeDevice(&body, device)
	}

	text := body.String()

	return reply{
		"ok": true, "schema": schema,
		"title":  title,
		"report": text,
		"url":    issueURL(title, text),
	}, nil
}

// maxIssueURL bounds the prefilled link.
//
// A full feature table plus a state dump percent-encodes to well over ten
// kilobytes, and a query string that long is rejected by some hops between
// here and GitHub. The link is a convenience; the report text is the real
// artefact and is always returned whole, so the link carries what fits and
// says where the rest is.
const maxIssueURL = 8000

func issueURL(title, text string) string {
	if link := encodeIssue(title, text); len(link) <= maxIssueURL {
		return link
	}

	const note = "\n\n_Truncated to fit a link. Run `omnigear report` and paste the full output._\n"
	// Percent-encoding inflates by a factor that depends on the content, so
	// close in on a length that fits rather than trying to compute one.
	for trimmed := len(text); trimmed > 0; {
		trimmed = trimmed * 3 / 4
		cut := text[:trimmed]
		if line := strings.LastIndex(cut, "\n"); line > 0 {
			cut = cut[:line]
		}
		if link := encodeIssue(title, cut+note); len(link) <= maxIssueURL {
			return link
		}
	}
	return encodeIssue(title, note)
}

func encodeIssue(title, body string) string {
	return issueBase + "?" + url.Values{
		"title":  {title},
		"body":   {body},
		"labels": {"device-request"},
	}.Encode()
}

func writeMachine(body *strings.Builder) {
	fmt.Fprintf(body, "## Environment\n\n")
	fmt.Fprintf(body, "- OmniGear: %s\n", buildLabel())
	fmt.Fprintf(body, "- Kernel: %s\n", kernelRelease())
	fmt.Fprintf(body, "- Arch: %s\n\n", runtime.GOARCH)
}

func buildLabel() string {
	switch {
	case branch != "" && commit != "":
		return branch + " " + commit
	case version != "":
		return "v" + version
	default:
		return "unknown"
	}
}

// writeDevice describes one device: what it calls itself, how it is attached,
// and every HID++ feature it admits to.
func writeDevice(body *strings.Builder, device *model.Device) {
	entry := device.Entry
	fmt.Fprintf(body, "## Device\n\n")
	fmt.Fprintf(body, "- Name: `%s`\n", entry.Model)
	if entry.USBLabel != "" {
		fmt.Fprintf(body, "- USB id: `%s`\n", entry.USBLabel)
	} else {
		fmt.Fprintf(body, "- USB id: `%04X:%04X`\n", device.Node.Vendor, device.Node.Product)
	}
	fmt.Fprintf(body, "- Connection: %s\n", model.ConnectionOf(device.Node).Label)
	fmt.Fprintf(body, "- Kernel driver: `%s`\n", device.Node.Driver)
	fmt.Fprintf(body, "- Catalogued: %v\n", !entry.Discovered)
	if len(entry.Capabilities) > 0 {
		names := make([]string, 0, len(entry.Capabilities))
		for _, capability := range entry.Capabilities {
			names = append(names, string(capability))
		}
		fmt.Fprintf(body, "- Detected capabilities: %s\n", strings.Join(names, ", "))
	}
	body.WriteString("\n")

	writeFeatures(body, device)

	if entry.Driver != nil {
		fmt.Fprintf(body, "## What it reported\n\n```json\n%s\n```\n\n", stateJSON(device))
	}
}

// writeFeatures lists the device's own feature table, which is the single
// most useful thing in the report: it says exactly what a driver could drive.
func writeFeatures(body *strings.Builder, device *model.Device) {
	link, err := openDevice(device)
	if err != nil {
		fmt.Fprintf(body, "## Features\n\nCould not open the device: %v\n\n", err)
		return
	}
	defer link.Close()

	features, err := link.Features()
	if err != nil {
		fmt.Fprintf(body, "## Features\n\nCould not read the feature table: %v\n\n", err)
		return
	}

	fmt.Fprintf(body, "## HID++ features\n\n| id | name | index | flags |\n|----|------|-------|-------|\n")
	for _, feature := range features {
		name := featureNames[feature.ID]
		if name == "" {
			name = "unknown"
		}
		flags := []string{}
		if feature.Hidden() {
			flags = append(flags, "hidden")
		}
		if feature.Obsolete() {
			flags = append(flags, "obsolete")
		}
		fmt.Fprintf(body, "| `0x%04X` | %s | %d | %s |\n",
			feature.ID, name, feature.Index, strings.Join(flags, ", "))
	}
	body.WriteString("\n")
}

// writeAllNodes is the no-device case: every hidraw node, whether this project
// understands it or not. A device nothing here can identify still shows up
// with its ids, which is what a maintainer needs to catalogue it.
func writeAllNodes(body *strings.Builder) {
	fmt.Fprintf(body, "## What is plugged in\n\n")
	fmt.Fprintf(body, "No single device was named, so this lists every HID node on the machine.\n\n")
	fmt.Fprintf(body, "| node | usb id | name | driver | connection | catalogued | hid++ |\n")
	fmt.Fprintf(body, "|------|--------|------|--------|------------|------------|-------|\n")

	for _, node := range hidraw.Enumerate() {
		catalogued := "no"
		if entry := catalog.FindByUSB(node.Vendor, node.Product); entry != nil {
			catalogued = entry.Model
		}
		fmt.Fprintf(body, "| `%s` | `%04X:%04X` | %s | `%s` | %s | %s | %v |\n",
			node.Path, node.Vendor, node.Product, node.Name, node.Driver,
			model.ConnectionOf(node).Label, catalogued, hidpp.Speaks(node))
	}
	body.WriteString("\n")
	fmt.Fprintf(body, "Please say which row is the device you want supported, and what it is called on the box.\n\n")
}

func openDevice(device *model.Device) (*hidpp.Device, error) {
	if device.Index != 0 {
		return hidpp.OpenAt(device.Node, device.Index)
	}
	return hidpp.Open(device.Node)
}

func stateJSON(device *model.Device) string {
	encoded, err := marshalIndent(device.Entry.Driver.Read(device))
	if err != nil {
		return "{}"
	}
	return encoded
}

// marshalIndent keeps the report's JSON readable in an issue body.
func marshalIndent(value any) (string, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	return string(encoded), err
}

// kernelRelease is uname -r, for the "which kernel binds this device" question
// that comes up whenever a receiver is not expanded.
func kernelRelease() string {
	var buf unix.Utsname
	if err := unix.Uname(&buf); err != nil {
		return "unknown"
	}
	return string(bytes.TrimRight(buf.Release[:], "\x00"))
}
