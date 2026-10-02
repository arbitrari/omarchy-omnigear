import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: a setting that is a whole number in a range —
// microphone volume, the mute light's brightness.
Column {
    id: root

    // The capability, which names the header.
    property string capability: ""
    // A Level from the CLI: { current, min, max }.
    property var level: null
    property QtObject bar: null
    property bool busy: false

    signal requested(int value)

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(6)
    visible: level !== null
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    SettingHeader {
        bar: root.bar
        group: true
        label: Model.capabilityLabel(root.capability)
        value: root.level ? root.level.current + " / " + root.level.max : ""
    }

    SquaredSlider {
        width: parent.width
        height: Style.spacing.controlHeight
        bar: root.bar
        minimum: root.level ? root.level.min : 0
        maximum: root.level ? root.level.max : 1
        value: root.level ? root.level.current : 0
        step: 1
        integer: true
        onReleased: function (v) {
            root.requested(Math.round(v));
        }
    }
}
