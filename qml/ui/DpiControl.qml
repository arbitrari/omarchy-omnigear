import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: DPI.
//
// Two ways to set it, because devices report two different things. The onboard
// stages (800/1200/4000 on a SUPERSTRIKE) are the values the mouse itself
// cycles between, so they get chips. The slider covers the full range for
// anything in between.
//
// Nothing is written while dragging: the device is only told once the knob is
// released, so a drag from 800 to 4000 is one write rather than hundreds.
Column {
    id: root

    property var dpi: null
    property QtObject bar: null
    property bool busy: false

    signal requested(int value)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property bool adjustable: dpi !== null && dpi.max > dpi.min

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(6)
    visible: dpi !== null
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    SettingHeader {
        bar: root.bar
        group: true
        label: Model.capabilityLabel("dpi")
        value: root.dpi ? root.dpi.current + " DPI" : "--"
    }

    ButtonGroup {
        id: presets

        // A device with a stepped range reports no stages worth offering, and
        // an empty chip row would just be a gap.
        visible: root.dpi !== null && root.dpi.presets.length > 0
        width: parent.width
        spacing: Style.space(4)

        foreground: root.foreground
        background: root.bar ? root.bar.background : Color.background
        accent: Color.accent
        fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
        fontSize: Style.font.bodySmall
        focusable: false

        options: {
            if (!root.dpi)
                return [];
            return root.dpi.presets.map(function (preset) {
                return { value: String(preset), label: String(preset) };
            });
        }
        value: root.dpi ? String(root.dpi.current) : ""

        onChanged: function (value) {
            root.requested(parseInt(value, 10));
        }
    }

    // SquaredSlider, not Omarchy's PanelSlider: same behaviour, squared corners.
    SquaredSlider {
        id: slider

        visible: root.adjustable
        width: parent.width
        height: Style.spacing.controlHeight
        bar: root.bar

        minimum: root.dpi ? root.dpi.min : 0
        maximum: root.dpi ? root.dpi.max : 1
        value: root.dpi ? root.dpi.current : 0
        step: Model.sliderStep(root.dpi)
        integer: true

        onReleased: function (value) {
            root.requested(Math.round(value));
        }
    }

    Item {
        width: parent.width
        height: bounds.implicitHeight
        visible: root.adjustable

        Text {
            id: bounds
            anchors.left: parent.left
            text: root.dpi ? String(root.dpi.min) : ""
            textFormat: Text.PlainText
            color: Qt.darker(root.foreground, 1.6)
            font.family: root.bar ? root.bar.fontFamily : Style.font.family
            font.pixelSize: Style.font.caption
        }

        Text {
            anchors.right: parent.right
            text: root.dpi ? String(root.dpi.max) : ""
            textFormat: Text.PlainText
            color: Qt.darker(root.foreground, 1.6)
            font.family: root.bar ? root.bar.fontFamily : Style.font.family
            font.pixelSize: Style.font.caption
        }
    }
}
