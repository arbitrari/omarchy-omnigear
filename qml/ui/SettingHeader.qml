import QtQuick
import qs.Commons

// The label-and-value line above a control: what it is on the left, what the
// device currently reports on the right.
//
// Two levels, because a card has two. A `group` header names a block of
// controls — DPI, POLLING RATE, LEFT CLICK — and is set in caps to match the
// panel's section titles. Without it the header names one field inside such a
// block — Actuation, Rapid Trigger — and is set quieter, in Title Case.
//
// The casing is applied here rather than in the strings, so the labels stay
// readable at the point they are written and the house style lives in one
// place.
Item {
    id: root

    property string label: ""
    property string value: ""
    property bool group: false
    property QtObject bar: null

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

    width: parent ? parent.width : implicitWidth
    implicitHeight: Math.max(name.implicitHeight, reading.implicitHeight)

    Text {
        id: name
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        text: root.group ? root.label.toUpperCase() : root.label
        textFormat: Text.PlainText
        color: root.group ? root.foreground : Qt.darker(root.foreground, 1.4)
        font.family: root.fontFamily
        font.pixelSize: root.group ? Style.font.bodySmall : Style.font.caption
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
