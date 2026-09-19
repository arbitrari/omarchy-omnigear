import QtQuick
import qs.Commons

// The label-and-value line that sits above a setting's control: the
// capability's name on the left, what the device currently reports on the
// right. Shared so DPI and polling rate cannot drift apart.
Item {
    id: root

    property string label: ""
    property string value: ""
    property QtObject bar: null
    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

    width: parent ? parent.width : implicitWidth
    implicitHeight: Math.max(name.implicitHeight, reading.implicitHeight)

    Text {
        id: name
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        text: root.label
        textFormat: Text.PlainText
        color: Qt.darker(root.foreground, 1.4)
        font.family: root.fontFamily
        font.pixelSize: Style.font.caption
        font.bold: true
    }

    Text {
        id: reading
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        text: root.value
        textFormat: Text.PlainText
        color: root.foreground
        font.family: root.fontFamily
        font.pixelSize: Style.font.bodySmall
    }
}
