import QtQuick
import qs.Commons
import "../Model.js" as Model

// The warning that a device is present but locked away.
//
// hidraw nodes are root-only by default, so without the project's udev rule a
// mouse is simply invisible: it enumerates, the kernel drives it as a mouse,
// and this plugin cannot open it to ask it anything. That failure has no
// symptom of its own — the device just never appears — which makes it the
// most misleading thing that can happen here, and the reason this card is
// worth the space.
//
// Shaped like a DeviceCard, tinted like the conflict card, for the same
// reason: it belongs in the stack of boxes rather than floating above it.
Rectangle {
    id: root

    property var paths: []
    property QtObject bar: null

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property color urgent: bar ? bar.urgent : Color.urgent
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

    width: parent ? parent.width : implicitWidth
    implicitHeight: body.implicitHeight + Style.space(20)
    radius: Style.space(0)
    color: Util.alpha(urgent, Style.selectedFillAlpha)

    Column {
        id: body
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: Style.space(10)
        spacing: Style.space(6)

        Text {
            width: parent.width
            text: "A DEVICE IS CONNECTED BUT UNREADABLE"
            textFormat: Text.PlainText
            wrapMode: Text.WordWrap
            color: root.urgent
            font.family: root.fontFamily
            font.pixelSize: Style.font.bodySmall
            font.bold: true
        }

        Text {
            width: parent.width
            text: Model.unreadableSummary(root.paths) + " "
                + root.paths.join(", ")
            textFormat: Text.PlainText
            wrapMode: Text.WordWrap
            color: root.foreground
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
        }

        Text {
            width: parent.width
            text: "HID devices are root-only until a udev rule says otherwise. "
                + "Install the one shipped with this plugin, then unplug and "
                + "replug the device:"
            textFormat: Text.PlainText
            wrapMode: Text.WordWrap
            color: Qt.darker(root.foreground, 1.4)
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
        }

        Text {
            width: parent.width
            text: "sudo cp ~/.config/omarchy/plugins/io.github.arbitrari.omnigear/"
                + "udev/60-omnigear.rules /etc/udev/rules.d/\nsudo udevadm control --reload-rules"
            textFormat: Text.PlainText
            wrapMode: Text.WrapAnywhere
            color: Qt.darker(root.foreground, 1.2)
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
        }
    }
}
