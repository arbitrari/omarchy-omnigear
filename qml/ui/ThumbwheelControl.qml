import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: the horizontal wheel under the thumb.
//
// The only thing worth deciding here is whether the wheel scrolls or has been
// handed to other software. Diverted, the device stops sending scroll events
// and sends HID++ notifications instead; nothing in OmniGear reads those, so
// unless something like Solaar is listening the wheel simply does nothing.
// That state is easy to end up in and impossible to diagnose from the desk,
// which is the reason this control exists at all.
//
// There is no direction switch. The device has an invert flag and it has no
// effect on the scroll stream — only on the diverted one — so offering it
// would be offering a switch that does nothing.
Column {
    id: root

    property var thumbwheel: null
    property QtObject bar: null
    property bool busy: false

    signal requested(string mode)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string mode: thumbwheel ? thumbwheel.mode : ""

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(6)
    visible: thumbwheel !== null
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    SettingHeader {
        bar: root.bar
        group: true
        label: Model.capabilityLabel("thumbwheel")
        value: Model.thumbwheelModeLabel(root.mode)
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
                value: "scroll",
                label: Model.thumbwheelModeLabel("scroll"),
                tooltip: "The wheel sends ordinary horizontal scroll events."
            },
            {
                value: "diverted",
                label: Model.thumbwheelModeLabel("diverted"),
                tooltip: "The wheel sends HID++ notifications for other software to handle."
            }
        ]
        value: root.mode

        onChanged: function (value) {
            root.requested(value);
        }
    }

    Text {
        width: parent.width
        visible: root.mode === "diverted"
        text: "Diverted. The wheel does nothing unless another program is "
            + "reading it. Set it back to scrolling to use it here."
        textFormat: Text.PlainText
        wrapMode: Text.WordWrap
        color: Qt.darker(root.foreground, 1.7)
        font.family: root.bar ? root.bar.fontFamily : Style.font.family
        font.pixelSize: Style.font.caption
    }
}
