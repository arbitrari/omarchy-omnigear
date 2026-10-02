import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: Sony's upscaling of compressed audio.
//
// Headed with the model's own name for it — DSEE Extreme on an XM4 — since
// that is what the Sony app and the box call it.
Column {
    id: root

    property var dsee: null
    property QtObject bar: null
    property bool busy: false

    signal requested(bool on)

    readonly property color foreground: bar ? bar.foreground : Color.foreground

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(6)
    visible: dsee !== null
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    SettingHeader {
        bar: root.bar
        group: true
        label: root.dsee ? root.dsee.label : Model.capabilityLabel("dsee")
        value: root.dsee && root.dsee.on ? "On" : "Off"
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
            { value: "off", label: "Off", tooltip: "Play compressed audio as it is." },
            { value: "on", label: "On", tooltip: "Restore the high frequencies compressed audio drops." }
        ]
        value: root.dsee && root.dsee.on ? "on" : "off"

        onChanged: function (value) {
            root.requested(value === "on");
        }
    }
}
