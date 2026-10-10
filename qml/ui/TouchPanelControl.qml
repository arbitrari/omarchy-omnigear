import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: whether the headset's touch-sensitive earcup takes
// gestures — swipes for volume and tracks, taps to play and pause.
Column {
    id: root

    // null until read, so a failed read draws nothing rather than "Off".
    property var on: null
    property QtObject bar: null
    property bool busy: false

    signal requested(bool on)

    readonly property color foreground: bar ? bar.foreground : Color.foreground

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(6)
    visible: on !== null
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    SettingHeader {
        bar: root.bar
        group: true
        label: Model.capabilityLabel("touch-panel")
        value: root.on === true ? "On" : "Off"
    }

    ButtonGroup {
        width: parent.width
        spacing: Style.space(4)

        foreground: root.foreground
        background: root.bar ? root.bar.background : Color.background
        accent: Color.accent
        fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
        fontSize: Style.font.bodySmall
        focusable: false

        options: [
            { value: "off", label: "Off", tooltip: "Swipes and taps on the earcup do nothing." },
            { value: "on", label: "On", tooltip: "Swipe and tap the earcup to control playback." }
        ]
        value: root.on === true ? "on" : "off"

        onChanged: function (value) {
            root.requested(value === "on");
        }
    }
}
