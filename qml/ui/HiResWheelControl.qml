import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: how the wheel reports movement.
//
// Two independent switches, so they get a row each rather than being folded
// into one control that would have to explain itself.
//
// Inverting happens in the mouse, before the desktop sees anything — so it
// stacks with whatever natural-scrolling setting the compositor has, rather
// than overriding it.
Column {
    id: root

    property var hiResWheel: null
    property QtObject bar: null
    property bool busy: false

    signal hiResRequested(bool on)
    signal invertRequested(bool on)

    readonly property color foreground: bar ? bar.foreground : Color.foreground

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(10)
    visible: hiResWheel !== null
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    component SwitchRow: Column {
        id: row

        property string label: ""
        // Named labelOn / tipOn, not onLabel / onTip: QML reserves any
        // property beginning with "on" followed by a capital for signal
        // handlers, and rejects the declaration outright.
        property string labelOff: ""
        property string labelOn: ""
        property string tipOff: ""
        property string tipOn: ""
        property bool on: false

        signal picked(bool on)

        width: parent ? parent.width : implicitWidth
        spacing: Style.space(6)

        SettingHeader {
            bar: root.bar
            group: true
            label: row.label
            value: row.on ? row.labelOn : row.labelOff
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
                    value: "off",
                    label: row.labelOff,
                    tooltip: row.tipOff
                },
                {
                    value: "on",
                    label: row.labelOn,
                    tooltip: row.tipOn
                }
            ]
            value: row.on ? "on" : "off"

            onChanged: function (value) {
                row.picked(value === "on");
            }
        }
    }

    SwitchRow {
        label: "Scroll Resolution"
        labelOff: "Standard"
        labelOn: "High"
        tipOff: "One step per notch of the wheel."
        tipOn: "Many small steps per notch, for smoother scrolling."
        on: root.hiResWheel ? root.hiResWheel.hiRes : false
        onPicked: function (on) {
            root.hiResRequested(on);
        }
    }

    SwitchRow {
        label: "Scroll Direction"
        labelOff: "Standard"
        labelOn: "Inverted"
        tipOff: "Scrolling down moves the page down."
        tipOn: "Scrolling down moves the page up, inverted in the mouse itself."
        on: root.hiResWheel ? root.hiResWheel.inverted : false
        onPicked: function (on) {
            root.invertRequested(on);
        }
    }
}
