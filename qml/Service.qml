import QtQuick
import Quickshell
import Quickshell.Io
import "Model.js" as Model

// Runs the `omnigear` CLI and holds its last reply.
//
// The UI never speaks to hardware; it speaks to this, and this speaks JSON to
// one short-lived process per poll. Nothing is cached across a failure — a
// device that stops answering goes empty rather than showing a stale battery.
Item {
    id: root

    property var settings: ({})

    readonly property var state: internal.state
    readonly property var devices: internal.state.devices
    readonly property bool ok: internal.state.ok
    readonly property string error: internal.state.error
    readonly property var primary: Model.primary(internal.state.devices, root.preferredDeviceId)

    // Which mouse the user picked to speak for the bar. Empty means "decide
    // for me". Scoped per kind of device, so a keyboard or headset battery can
    // be chosen independently once those are reported too.
    readonly property string preferredDeviceId: {
        var chosen = settings ? settings.primaryMouse : "";
        return chosen ? String(chosen) : "";
    }
    readonly property bool busy: listProcess.running || setProcess.running

    // What the binary was built from. Fixed for the life of the process, so it
    // is asked once at startup rather than riding along on every poll.
    readonly property var build: internal.build

    readonly property int pollIntervalSec: {
        var configured = settings ? Number(settings.pollInterval) : NaN;
        return Math.max(5, configured || 20);
    }

    // True while the panel is open and the user is actually looking at the
    // controls. Only then is a full read worth what it costs.
    property bool detailed: false

    // How often a full HID++ read happens with the panel shut.
    //
    // A full read is sixty-odd round trips, and every one of them makes a
    // wireless mouse power its radio to answer — on the ordinary poll timer
    // that is a mouse which never gets to sleep. Charge comes from the kernel
    // instead, which costs nothing and wakes nothing; this is only for what
    // the kernel cannot say, notably an exact percentage on devices whose
    // battery feature it reports as a coarse word.
    readonly property int fullReadIntervalSec: 15 * 60

    // Resolved from this file's own location rather than the plugin id, so the
    // path survives a rename — `omarchy plugin clone` installs the same code
    // under a different directory name.
    readonly property string binary: {
        var url = Qt.resolvedUrl("../bin/omnigear").toString();
        return url.indexOf("file://") === 0 ? url.substring(7) : url;
    }

    signal applied(string deviceId, string key, bool success, string message)

    function refresh() {
        if (listProcess.running)
            return;
        // Remember how many writes had landed when this read started, so a
        // reply that was already in flight when one landed can be spotted.
        internal.readGeneration = internal.writes;
        listProcess.running = true;
    }

    // Ask the CLI to write a setting. It verifies against hardware and replies
    // with the device as it actually ended up, so the reply is the new truth
    // and there is no need to poll again behind it.
    //
    // A write already in flight does not cancel this one: moving two sliders
    // in quick succession used to drop the second silently, which read exactly
    // like the device refusing it. The newest request is held and sent when
    // the current one finishes.
    function apply(deviceId, key, value) {
        if (setProcess.running) {
            internal.queued = {
                deviceId: deviceId,
                key: key,
                value: String(value)
            };
            return;
        }
        internal.send(deviceId, key, value);
    }

    QtObject {
        id: internal
        property var state: Model.emptyState()
        property var build: ({ branch: "", commit: "", label: "" })
        property string pendingId: ""
        property string pendingKey: ""

        // Counts completed writes. A read that started before a write finished
        // is carrying pre-write values, and applying it would undo what the
        // user just did.
        property int writes: 0
        property int readGeneration: 0

        // At most one deferred write: if the user moves a slider three times
        // while a write is out, only the last position is worth sending.
        property var queued: null

        function send(deviceId, key, value) {
            pendingId = deviceId;
            pendingKey = key;
            setProcess.command = [root.binary, "set", deviceId, key, String(value)];
            setProcess.running = true;
        }

        function sendQueued() {
            if (!queued)
                return;
            var next = queued;
            queued = null;
            send(next.deviceId, next.key, next.value);
        }

        // When the last full read happened, so the slow timer can tell whether
        // one is due without a second timer to keep in step.
        property double lastFullRead: 0

        function applyBatteries(readings) {
            if (!readings || readings.length === 0)
                return;
            var byId = {};
            readings.forEach(function (reading) { byId[reading.id] = reading; });

            var next = state.devices.map(function (device) {
                return byId[device.id] ? Model.mergeBattery(device, byId[device.id]) : device;
            });
            state = {
                ok: state.ok,
                devices: next,
                error: state.error,
                note: state.note,
                unreadable: state.unreadable
            };
        }

        // Replace one device in place, leaving the others alone. A `set` reply
        // carries only the device it wrote; the rest of the list is still good.
        function merge(device) {
            if (!device)
                return;
            var next = state.devices.map(function (existing) {
                return existing.id === device.id ? device : existing;
            });
            state = {
                ok: state.ok,
                devices: next,
                error: state.error,
                note: state.note,
                unreadable: state.unreadable
            };
        }
    }

    // Refresh charge without touching the hardware. What comes back is folded
    // into the devices from the last full read rather than replacing them:
    // this reply knows nothing about DPI, buttons or anything else.
    function refreshBattery() {
        if (batteryProcess.running || listProcess.running)
            return;
        batteryProcess.running = true;
    }

    Process {
        id: batteryProcess
        command: [root.binary, "battery"]
        stdout: StdioCollector {
            id: batteryOut
            waitForEnd: true
            onStreamFinished: {
                internal.applyBatteries(Model.parseBatteries((batteryOut.text || "").trim()));
            }
        }
    }

    // Ask the CLI to write up a device, then hand the prefilled issue to the
    // browser. The report is assembled in Go because it is the side that can
    // talk to the device; this only opens what comes back.
    function report(deviceId) {
        if (reportProcess.running)
            return;
        reportProcess.command = deviceId
            ? [root.binary, "report", deviceId]
            : [root.binary, "report"];
        reportProcess.running = true;
    }

    signal reportFailed(string message)

    Process {
        id: reportProcess
        stdout: StdioCollector {
            id: reportOut
            waitForEnd: true
            onStreamFinished: {
                var reply = Model.parseReport((reportOut.text || "").trim());
                if (reply.url !== "")
                    Qt.openUrlExternally(reply.url);
                else
                    root.reportFailed(reply.error || "Could not write the report");
            }
        }
    }

    Process {
        id: versionProcess
        command: [root.binary, "version"]
        running: true
        stdout: StdioCollector {
            id: versionOut
            waitForEnd: true
            onStreamFinished: {
                internal.build = Model.parseBuild((versionOut.text || "").trim());
            }
        }
    }

    Process {
        id: listProcess
        command: [root.binary, "list"]
        stdout: StdioCollector {
            id: listOut
            waitForEnd: true
            onStreamFinished: {
                if (internal.readGeneration !== internal.writes) {
                    // A write landed while this read was out, so it is stale by
                    // exactly the value the user just changed. Discard it and
                    // read again rather than flickering back.
                    Qt.callLater(root.refresh);
                    return;
                }
                internal.state = Model.parse((listOut.text || "").trim());
            }
        }
    }

    Process {
        id: setProcess
        stdout: StdioCollector {
            id: setOut
            waitForEnd: true
            onStreamFinished: {
                var reply = Model.parse((setOut.text || "").trim());
                var deviceId = internal.pendingId;
                var key = internal.pendingKey;
                internal.pendingId = "";
                internal.pendingKey = "";
                internal.writes++;

                if (reply.ok && reply.devices.length > 0) {
                    internal.merge(reply.devices[0]);
                    root.applied(deviceId, key, true, reply.note);
                } else if (reply.ok) {
                    // A write the CLI could not read back, because the device
                    // is no longer here to read: switching Easy-Switch host
                    // hands the mouse to another computer. It went through.
                    // Re-read so the card shows it as gone rather than frozen
                    // on its last known state.
                    root.applied(deviceId, key, true, reply.note);
                    root.refresh();
                } else {
                    // The write was refused, or went through and changed
                    // nothing. Re-read so the panel shows the device as it is
                    // rather than as it was asked to be.
                    root.applied(deviceId, key, false, reply.error);
                    root.refresh();
                }

                internal.sendQueued();
            }
        }
    }

    // One timer, two jobs. With the panel open the user is watching live
    // controls and a full read is what they are asking for; with it shut only
    // the charge on the bar is on screen, and that comes from the kernel.
    // A full read still happens occasionally, for what the kernel cannot say.
    Timer {
        interval: root.pollIntervalSec * 1000
        running: true
        repeat: true
        triggeredOnStart: true
        onTriggered: {
            var now = Date.now();
            var due = now - internal.lastFullRead > root.fullReadIntervalSec * 1000;
            if (root.detailed || due || internal.lastFullRead === 0) {
                internal.lastFullRead = now;
                root.refresh();
            } else {
                root.refreshBattery();
            }
        }
    }
}
