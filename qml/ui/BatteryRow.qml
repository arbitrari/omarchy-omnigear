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

    // Scored rather than read straight off the percentage: a device that
    // only counts steps has no percentage, and "Low" still needs to look low.
    readonly property int score: Model.batteryScore(battery)
    readonly property bool low: score >= 0 && score <= 20

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(6)
    visible: battery !== null

    Text {
        anchors.verticalCenter: parent.verticalCenter
        text: Model.batteryIcon(root.battery)
        color: root.low ? (root.bar ? root.bar.urgent : Color.urgent) : root.foreground
        font.family: "monospace"
        font.pixelSize: Style.font.title
    }

    Text {
        anchors.verticalCenter: parent.verticalCenter
        text: Model.batteryReading(root.battery)
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
