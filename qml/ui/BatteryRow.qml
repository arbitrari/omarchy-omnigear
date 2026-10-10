import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: battery. Read-only — nothing here writes to a device.
//
// One of the per-capability components under ui/. A device gets this row
// because its catalog entry declares `battery`, not because of what it is, so
// a headset and a mouse draw the same one.
//
// A spare battery, charging in a base station to swap in, sits on a line of
// its own beneath, so it never crowds the reading beside it.
Column {
    id: root

    property var battery: null
    property QtObject bar: null
    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

    // Scored rather than read straight off the percentage: a device that
    // only counts steps has no percentage, and "Low" still needs to look low.
    readonly property int score: Model.batteryScore(battery)
    readonly property bool low: score >= 0 && score <= 20
    readonly property var spare: Model.spareBattery(battery)
    readonly property color muted: Qt.darker(foreground, 1.4)

    width: parent ? parent.width : implicitWidth
    spacing: 0
    visible: battery !== null

    Row {
        anchors.right: parent.right
        spacing: Style.space(6)

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

    // Drawn the same way as the battery in use but smaller and greyed out:
    // it is not the one running down. Hovering says what it is, since a
    // second battery on a headset is not something anyone expects.
    Row {
        anchors.right: parent.right
        spacing: Style.space(4)
        visible: root.spare !== null

        // A handler rather than a MouseArea: the whole header is the
        // collapse control, and a MouseArea here would swallow its clicks.
        HoverHandler {
            id: spareHover
        }

        PanelToolTip {
            visible: spareHover.hovered
            text: "Spare battery, charging in the base station"
            fontFamily: root.fontFamily
        }

        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: Model.batteryIcon(root.spare)
            color: root.muted
            font.family: "monospace"
            font.pixelSize: Style.font.body
        }

        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: Model.batteryReading(root.spare)
            textFormat: Text.PlainText
            color: root.muted
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
            font.bold: true
        }
    }
}
