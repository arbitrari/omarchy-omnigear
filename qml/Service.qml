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
    readonly property bool busy: listProcess.running

    readonly property int pollIntervalSec: {
        var configured = settings ? Number(settings.pollInterval) : NaN
        return Math.max(5, configured || 20)
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

    // `set` writes, verifies against hardware, and returns the device fresh —
    // so the reply is used as the new state rather than re-polling after it.
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
                // A `set` reply carries one device, not a list; a failed one
                // carries only the error. Either way, re-read to stay truthful.
                root.applied(internal.pendingId, internal.pendingKey, reply.ok, reply.error);
                internal.pendingId = "";
                internal.pendingKey = "";
                root.refresh();
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
