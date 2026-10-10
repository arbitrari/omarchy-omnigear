import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: a base station's screen, on or off.
//
// Off is OmniGear's dark picture, not the device's own off, so the base
// station takes its screen back as soon as it is touched.
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
        label: Model.capabilityLabel("screen")
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
            { value: "off", label: "Off", tooltip: "Blank the screen until the base station is touched." },
            { value: "on", label: "On", tooltip: "Show the base station's own screen." }
        ]
        value: root.on === true ? "on" : "off"

        onChanged: function (value) {
            root.requested(value === "on");
        }
    }

    Text {
        width: parent.width
        text: "Turning the volume knob or the ChatMix dial brings the screen back."
        textFormat: Text.PlainText
        wrapMode: Text.WordWrap
        color: Qt.darker(root.foreground, 1.7)
        font.family: root.bar ? root.bar.fontFamily : Style.font.family
        font.pixelSize: Style.font.caption
    }
}
