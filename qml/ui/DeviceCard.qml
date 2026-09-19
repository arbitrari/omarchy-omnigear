import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// One device, drawn from what it says it can do.
//
// There is no per-model UI anywhere in this plugin and there should never be:
// the card asks the device for its capability list and renders the matching
// control from ui/. A new mouse is a catalog entry in Go and nothing here.
Rectangle {
    id: root

    property var device: null
    property QtObject bar: null
    property bool busy: false

    // Emitted when the user asks for a change. The card never talks to the
    // hardware itself — the panel owns the service.
    signal settingRequested(string key, int value)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family
    readonly property var unsupported: Model.unsupportedCapabilities(device)

    width: parent ? parent.width : implicitWidth
    implicitHeight: body.implicitHeight + Style.space(20)
    radius: Style.space(8)
    color: Style.normalFill

    Column {
        id: body
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: Style.space(10)
        spacing: Style.space(10)

        // --- who it is -----------------------------------------------------
        Item {
            width: parent.width
            implicitHeight: Math.max(identity.implicitHeight, battery.implicitHeight)

            Row {
                id: identity
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                spacing: Style.space(8)

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: Model.categoryIcon(root.device ? root.device.category : "")
                    color: root.foreground
                    font.family: "monospace"
                    font.pixelSize: Style.font.heading
                }

                Column {
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Style.space(2)

                    Text {
                        text: root.device ? root.device.name : ""
                        textFormat: Text.PlainText
                        color: root.foreground
                        font.family: root.fontFamily
                        font.pixelSize: Style.font.subtitle
                        font.bold: true
                    }

                    Text {
                        readonly property string support: root.device
                            ? Model.supportLabel(root.device.support) : ""

                        text: {
                            var brand = root.device ? root.device.brand : "";
                            return support === "" ? brand : brand + " · " + support;
                        }
                        textFormat: Text.PlainText
                        color: Qt.darker(root.foreground, 1.5)
                        font.family: root.fontFamily
                        font.pixelSize: Style.font.caption
                    }
                }
            }

            BatteryRow {
                id: battery
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                width: implicitWidth
                bar: root.bar
                battery: root.device ? root.device.battery : null
            }
        }

        PanelSeparator {
            foreground: root.foreground
            visible: dpi.visible || rate.visible || unsupportedList.visible
        }

        // --- what it can do ------------------------------------------------
        DpiControl {
            id: dpi
            bar: root.bar
            busy: root.busy
            dpi: root.device ? root.device.dpi : null
            onRequested: function (value) {
                root.settingRequested("dpi", value);
            }
        }

        PollingRateControl {
            id: rate
            bar: root.bar
            busy: root.busy
            pollingRate: root.device ? root.device.pollingRate : null
            onRequested: function (hz) {
                root.settingRequested("polling-rate", hz);
            }
        }

        Column {
            id: unsupportedList
            width: parent.width
            spacing: Style.space(4)
            visible: root.unsupported.length > 0

            Repeater {
                model: root.unsupported
                delegate: UnavailableRow {
                    required property string modelData
                    capability: modelData
                    bar: root.bar
                }
            }
        }

        // --- what went wrong -----------------------------------------------
        Column {
            width: parent.width
            spacing: Style.space(4)
            visible: root.device && root.device.errors.length > 0

            Repeater {
                model: root.device ? root.device.errors : []

                delegate: Text {
                    required property string modelData
                    width: parent.width
                    text: modelData
                    textFormat: Text.PlainText
                    wrapMode: Text.WordWrap
                    color: root.bar ? root.bar.urgent : Color.urgent
                    font.family: root.fontFamily
                    font.pixelSize: Style.font.caption
                }
            }
        }
    }
}
