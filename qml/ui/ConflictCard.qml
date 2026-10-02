import QtQuick
import qs.Commons
import "../Model.js" as Model

// The warning that something else is driving these devices.
//
// Shaped like a DeviceCard on purpose — same width, same square corners, same
// inner margins — so it sits in the stack as one of the panel's boxes rather
// than as loose text above them. The fill is the only thing that differs: the
// theme's urgent colour at the same kind of low alpha the cards use, so it
// reads as a red card and not as a red rectangle stapled to the panel.
//
// It is shown only when there is something to report. Nothing is drawn for
// the clear case, because only same-user processes are visible through /proc
// and "nothing found" is not the same as "nothing there".
Rectangle {
    id: root

    property var conflicts: []
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
            text: "ANOTHER PROGRAM IS USING THESE DEVICES"
            textFormat: Text.PlainText
            wrapMode: Text.WordWrap
            color: root.urgent
            font.family: root.fontFamily
            font.pixelSize: Style.font.bodySmall
            font.bold: true
        }

        // One line per program, naming what it has hold of.
        Repeater {
            model: root.conflicts

            delegate: Text {
                required property var modelData
                width: parent.width
                text: Model.conflictLine(modelData)
                textFormat: Text.PlainText
                wrapMode: Text.WordWrap
                color: root.foreground
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
            }
        }

        Text {
            width: parent.width
            text: "Two programs on one device share every reply, so changes "
                + "made here can be silently reverted. Quit it if settings "
                + "will not stick."
            textFormat: Text.PlainText
            wrapMode: Text.WordWrap
            color: Qt.darker(root.foreground, 1.4)
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
        }
    }
}
