import QtQuick
import qs.Commons
import "../Model.js" as Model

// A capability the device has and this build cannot drive yet.
//
// Showing it is deliberate. The mouse really does have haptic triggers; the
// software is what is missing, and saying so is more honest than a panel that
// silently looks like the hardware is less than it is.
Row {
    id: root

    property string capability: ""
    property QtObject bar: null
    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(6)

    Text {
        anchors.verticalCenter: parent.verticalCenter
        text: Model.capabilityLabel(root.capability).toUpperCase()
        textFormat: Text.PlainText
        color: root.foreground
        font.family: root.fontFamily
        font.pixelSize: Style.font.bodySmall
        font.bold: true
    }

    Text {
        anchors.verticalCenter: parent.verticalCenter
        text: "not driven yet"
        textFormat: Text.PlainText
        color: Qt.darker(root.foreground, 1.8)
        font.family: root.fontFamily
        font.pixelSize: Style.font.caption
        font.italic: true
    }
}
