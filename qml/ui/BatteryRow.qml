import QtQuick
import qs.Commons
import "../Model.js" as Model

// Capability control: battery. Read-only — nothing here writes to a device.
//
// One of the per-capability components under ui/. A device gets this row
// because its catalog entry declares `battery`, not because of what it is, so
// a headset and a mouse draw the same one.
Row {
    id: root

    property var battery: null
    property QtObject bar: null
    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

    readonly property int percent: battery ? battery.percent : Model.UNKNOWN
    readonly property bool low: percent >= 0 && percent <= 20

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(6)
    visible: battery !== null

    Text {
        anchors.verticalCenter: parent.verticalCenter
        text: Model.batteryIcon(root.percent, root.battery ? root.battery.status : "unknown")
        color: root.low ? (root.bar ? root.bar.urgent : Color.urgent) : root.foreground
        font.family: "monospace"
        font.pixelSize: Style.font.title
    }

    Text {
        anchors.verticalCenter: parent.verticalCenter
        text: Model.batteryText(root.percent)
        textFormat: Text.PlainText
        color: root.low ? (root.bar ? root.bar.urgent : Color.urgent) : root.foreground
        font.family: root.fontFamily
        font.pixelSize: Style.font.body
        font.bold: true
    }

    Text {
        anchors.verticalCenter: parent.verticalCenter
        text: Model.batteryStatusLabel(root.battery)
        textFormat: Text.PlainText
        color: Qt.darker(root.foreground, 1.4)
        font.family: root.fontFamily
        font.pixelSize: Style.font.bodySmall
    }
}
