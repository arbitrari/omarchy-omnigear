import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: whether the headset pauses and lets the room in when
// its wearer starts talking.
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
        label: Model.capabilityLabel("speak-to-chat")
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
            { value: "off", label: "Off", tooltip: "Talking changes nothing." },
            { value: "on", label: "On", tooltip: "Talking pauses the music and lets in the room." }
        ]
        value: root.on === true ? "on" : "off"

        onChanged: function (value) {
            root.requested(value === "on");
        }
    }
}
