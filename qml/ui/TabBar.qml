import QtQuick
import qs.Commons

// A tab strip for switching between groups of controls.
//
// Deliberately not a ButtonGroup. The chips inside a card — DPI presets,
// polling rates, onboard/software — are all bordered ButtonGroups, so a tab
// strip built the same way reads as one more setting rather than as
// navigation. Tabs here are unbordered labels on a rail, with the active one
// underlined: a different shape for a different job.
Item {
    id: root

    // [{ id, label }], in the order they should appear.
    property var tabs: []
    property string current: ""
    property QtObject bar: null

    signal selected(string id)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

    readonly property int underline: Math.max(2, Style.space(2))
    readonly property int gap: Style.space(6)

    // Height comes from the font, not from the laid-out labels. Deriving it
    // from the Row while the delegates size themselves against the root is a
    // polish loop, and Qt says so at runtime.
    FontMetrics {
        id: metrics
        font.family: root.fontFamily
        font.pixelSize: Style.font.bodySmall
    }

    readonly property int rowHeight: Math.ceil(metrics.height) + gap + underline

    width: parent ? parent.width : implicitWidth
    implicitHeight: rowHeight

    // The rail runs the full width so the strip reads as one surface, with the
    // active tab's segment sitting brighter on top of it.
    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: Math.max(1, Style.space(1))
        color: Qt.rgba(root.foreground.r, root.foreground.g, root.foreground.b, 0.12)
    }

    Row {
        id: labels
        anchors.left: parent.left
        anchors.top: parent.top
        spacing: Style.space(16)

        Repeater {
            model: root.tabs

            delegate: Item {
                id: tab
                required property var modelData

                readonly property bool active: root.current === tab.modelData.id

                width: text.implicitWidth
                height: root.rowHeight

                Text {
                    id: text
                    anchors.top: parent.top
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: tab.modelData.label
                    textFormat: Text.PlainText
                    color: tab.active
                        ? root.foreground
                        : (hover.containsMouse
                            ? Qt.darker(root.foreground, 1.2)
                            : Qt.darker(root.foreground, 1.7))
                    font.family: root.fontFamily
                    font.pixelSize: Style.font.bodySmall
                    font.bold: tab.active

                    Behavior on color {
                        ColorAnimation { duration: 110 }
                    }
                }

                Rectangle {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    height: root.underline
                    color: root.foreground
                    opacity: tab.active ? 1.0 : 0.0

                    Behavior on opacity {
                        NumberAnimation { duration: 110 }
                    }
                }

                MouseArea {
                    id: hover
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: root.selected(tab.modelData.id)
                }
            }
        }
    }
}
