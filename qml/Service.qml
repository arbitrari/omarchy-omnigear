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
    readonly property var primary: Model.primary(internal.state.devices)
    readonly property bool busy: listProcess.running || setProcess.running

    readonly property int pollIntervalSec: {
        var configured = settings ? Number(settings.pollInterval) : NaN;
        return Math.max(5, configured || 20);
    }

    // Resolved from this file's own location rather than the plugin id, so the
    // path survives a rename — `omarchy plugin clone` installs the same code
    // under a different directory name.
    readonly property string binary: {
        var url = Qt.resolvedUrl("../bin/omnigear").toString();
        return url.indexOf("file://") === 0 ? url.substring(7) : url;
    }

    signal applied(string deviceId, string key, bool success, string message)

    function refresh() {
        if (!listProcess.running)
            listProcess.running = true;
    }

    // Ask the CLI to write a setting. It verifies against hardware and replies
    // with the device as it actually ended up, so the reply is the new truth
    // and there is no need to poll again behind it.
    function apply(deviceId, key, value) {
        if (setProcess.running)
            return;
        internal.pendingId = deviceId;
        internal.pendingKey = key;
        setProcess.command = [root.binary, "set", deviceId, key, String(value)];
        setProcess.running = true;
    }

    QtObject {
        id: internal
        property var state: Model.emptyState()
        property string pendingId: ""
        property string pendingKey: ""

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
                note: state.note
            };
        }
    }

    Process {
        id: listProcess
        command: [root.binary, "list"]
        stdout: StdioCollector {
            id: listOut
            waitForEnd: true
            onStreamFinished: internal.state = Model.parse((listOut.text || "").trim())
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

                if (reply.ok && reply.devices.length > 0) {
                    internal.merge(reply.devices[0]);
                    root.applied(deviceId, key, true, reply.note);
                } else {
                    // The write was refused, or went through and changed
                    // nothing. Re-read so the panel shows the device as it is
                    // rather than as it was asked to be.
                    root.applied(deviceId, key, false, reply.error);
                    root.refresh();
                }
            }
        }
    }

    Timer {
        interval: root.pollIntervalSec * 1000
        running: true
        repeat: true
        triggeredOnStart: true
        onTriggered: root.refresh()
    }
}
