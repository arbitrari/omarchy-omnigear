import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: Easy-Switch, the host slots an MX device is paired to.
//
// This is the one control whose whole effect is that the device stops talking
// to this machine. There is no undo from here: once the mouse is on another
// host, the button on its underside is what brings it back. So the choice is
// spelled out on screen rather than left to a row of unlabelled numbers.
//
// Only paired slots are offered. An empty slot is named but not clickable,
// because switching to one leaves the device hunting for a host that was
// never there, and ButtonGroup has no disabled state to say so with.
Column {
    id: root

    property var hosts: null
    property QtObject bar: null
    property bool busy: false

    signal requested(int slot)

    readonly property color foreground: bar ? bar.foreground : Color.foreground

    readonly property var paired: hosts
        ? hosts.slots.filter(function (host) { return host.paired; })
        : []
    readonly property var empty: hosts
        ? hosts.slots.filter(function (host) { return !host.paired; })
        : []

    readonly property var currentHost: {
        if (!hosts)
            return null;
        var match = hosts.slots.filter(function (host) { return host.active; });
        return match.length > 0 ? match[0] : null;
    }

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(6)
    visible: hosts !== null
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    SettingHeader {
        bar: root.bar
        group: true
        label: Model.capabilityLabel("host")
        value: Model.hostLabel(root.currentHost)
    }

    ButtonGroup {
        visible: root.paired.length > 1
        width: parent.width
        spacing: Style.space(4)

        foreground: root.foreground
        background: root.bar ? root.bar.background : Color.background
        accent: Color.accent
        fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
        fontSize: Style.font.bodySmall
        focusable: false

        options: root.paired.map(function (host) {
            return {
                value: String(host.slot),
                label: Model.hostLabel(host),
                tooltip: host.active
                    ? "The computer this mouse is on now."
                    : "Move the mouse to this computer. It leaves this one."
            };
        })
        value: root.currentHost ? String(root.currentHost.slot) : ""

        onChanged: function (value) {
            root.requested(parseInt(value, 10));
        }
    }

    Text {
        width: parent.width
        visible: root.paired.length > 1
        text: {
            var base = "Switching hands the mouse to that computer. It "
                + "disappears from this one until you switch back, which you "
                + "do with the button underneath the mouse.";
            return root.pairingKnown
                ? base
                : base + " This device does not report which slots are "
                    + "paired, so all of them are offered.";
        }
        textFormat: Text.PlainText
        wrapMode: Text.WordWrap
        color: Qt.darker(root.foreground, 1.7)
        font.family: root.bar ? root.bar.fontFamily : Style.font.family
        font.pixelSize: Style.font.caption
    }

    Text {
        width: parent.width
        visible: root.empty.length > 0
        text: {
            var numbers = root.empty.map(function (host) { return host.slot; });
            var slots = numbers.length === 1
                ? "Slot " + numbers[0] + " is"
                : "Slots " + numbers.join(" and ") + " are";
            return slots + " not paired to anything. Pair from the button "
                + "underneath the mouse.";
        }
        textFormat: Text.PlainText
        wrapMode: Text.WordWrap
        color: Qt.darker(root.foreground, 1.7)
        font.family: root.bar ? root.bar.fontFamily : Style.font.family
        font.pixelSize: Style.font.caption
    }
}
