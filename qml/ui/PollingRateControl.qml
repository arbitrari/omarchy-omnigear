import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: polling rate.
//
// Only the rates the device says it supports *on its current link* are
// offered — a Lightspeed mouse that does 8000 Hz on a cable does 1000 Hz on
// the dongle, and showing a chip that cannot work is worse than not showing
// it. The driver reads that list per link; this just draws it.
Column {
    id: root

    property var pollingRate: null
    property QtObject bar: null
    property bool busy: false

    signal requested(int hz)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property bool choosable: pollingRate !== null && pollingRate.supported.length > 1

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(6)
    visible: pollingRate !== null
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    SettingHeader {
        bar: root.bar
        group: true
        label: Model.capabilityLabel("polling-rate")
        value: root.pollingRate && root.pollingRate.current > 0
            ? root.pollingRate.current + " Hz"
            : "--"
    }

    ButtonGroup {
        visible: root.choosable
        width: parent.width
        spacing: Style.space(4)

        foreground: root.foreground
        background: root.bar ? root.bar.background : Color.background
        accent: Color.accent
        fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
        fontSize: Style.font.bodySmall
        focusable: false

        options: {
            if (!root.pollingRate)
                return [];
            return root.pollingRate.supported.map(function (hz) {
                return { value: String(hz), label: String(hz) };
            });
        }
        value: root.pollingRate ? String(root.pollingRate.current) : ""

        onChanged: function (value) {
            root.requested(parseInt(value, 10));
        }
    }
}
