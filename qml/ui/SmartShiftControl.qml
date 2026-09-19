import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: SmartShift — how the scroll wheel behaves.
//
// A ratcheting wheel clicks line by line until it is flicked hard enough, at
// which point it breaks into a free spin. The threshold is how hard that has
// to be, so it only means anything while the wheel is ratcheting.
Column {
    id: root

    property var smartShift: null
    property QtObject bar: null
    property bool busy: false

    signal modeRequested(string mode)
    signal thresholdRequested(int threshold)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property bool ratcheting: smartShift && smartShift.mode === "ratchet"

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(10)
    visible: smartShift !== null
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    Column {
        width: parent.width
        spacing: Style.space(6)

        SettingHeader {
            bar: root.bar
            group: true
            label: "Scroll Mode"
            value: root.smartShift ? Model.wheelModeLabel(root.smartShift.mode) : ""
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
                {
                    value: "ratchet",
                    label: Model.wheelModeLabel("ratchet"),
                    tooltip: "The wheel clicks line by line until flicked hard."
                },
                {
                    value: "freespin",
                    label: Model.wheelModeLabel("freespin"),
                    tooltip: "The wheel always spins freely."
                }
            ]
            value: root.smartShift ? root.smartShift.mode : ""

            onChanged: function (value) {
                root.modeRequested(value);
            }
        }
    }

    Column {
        width: parent.width
        spacing: Style.space(6)
        // Meaningless on a wheel that is already spinning free.
        visible: root.ratcheting

        SettingHeader {
            bar: root.bar
            group: true
            label: "Shift Threshold"
            value: root.smartShift ? Model.thresholdLabel(root.smartShift.threshold) : ""
        }

        SquaredSlider {
            width: parent.width
            height: Style.spacing.controlHeight
            bar: root.bar
            minimum: 1
            maximum: root.smartShift ? root.smartShift.max : 1
            // "Never" sits past the end of the scale, so the knob rests at the
            // far end rather than snapping back to the middle.
            value: {
                if (!root.smartShift)
                    return 1;
                return root.smartShift.threshold >= Model.THRESHOLD_NEVER
                    ? root.smartShift.max
                    : root.smartShift.threshold;
            }
            step: 1
            integer: true
            onReleased: function (v) {
                root.thresholdRequested(Math.round(v));
            }
        }

        Item {
            width: parent.width
            height: low.implicitHeight

            Text {
                id: low
                anchors.left: parent.left
                text: "Flick Lightly"
                textFormat: Text.PlainText
                color: Qt.darker(root.foreground, 1.6)
                font.family: root.bar ? root.bar.fontFamily : Style.font.family
                font.pixelSize: Style.font.caption
            }

            Text {
                anchors.right: parent.right
                text: "Flick Hard"
                textFormat: Text.PlainText
                color: Qt.darker(root.foreground, 1.6)
                font.family: root.bar ? root.bar.fontFamily : Style.font.family
                font.pixelSize: Style.font.caption
            }
        }
    }
}
