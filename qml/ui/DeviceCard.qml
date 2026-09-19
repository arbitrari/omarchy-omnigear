import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// One device, drawn from what it says it can do.
//
// There is no per-model UI anywhere in this plugin and there should never be:
// the card asks the device for its capability list and renders the matching
// control from ui/. A new mouse is a catalog entry in Go and nothing here.
//
// Controls are grouped into tabs, and the tabs come from that same capability
// list — a device with nothing to put in a group never grows that tab, and a
// device with only one group never shows a strip at all.
Rectangle {
    id: root

    property var device: null
    property QtObject bar: null
    property bool busy: false

    // Emitted when the user asks for a change. The card never talks to the
    // hardware itself — the panel owns the service.
    signal settingRequested(string key, string value)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family
    readonly property var unsupported: Model.unsupportedCapabilities(device)
    readonly property var tabs: Model.deviceTabs(device)
    readonly property bool showTabs: tabs.length > 1

    // Which tab is open is owned by the panel, not by this card. The card is a
    // Repeater delegate and every poll rebuilds the model, destroying and
    // recreating it — state kept here would be reset on the next read, which
    // looked like the tab switching itself back mid-edit.
    property string activeTab: ""

    signal tabSelected(string id)

    // Falls back to the first tab when the panel has no preference yet, or
    // when a remembered tab no longer exists because the device lost that
    // capability.
    readonly property string currentTab: {
        for (var i = 0; i < tabs.length; i++) {
            if (tabs[i].id === activeTab)
                return activeTab;
        }
        return tabs.length > 0 ? tabs[0].id : "";
    }

    width: parent ? parent.width : implicitWidth
    implicitHeight: body.implicitHeight + Style.space(20)
    radius: Style.space(0)
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
                        text: Model.deviceSubtitle(root.device)
                        textFormat: Text.PlainText
                        color: Qt.darker(root.foreground, 1.5)
                        font.family: root.fontFamily
                        font.pixelSize: Style.font.caption
                    }
                }
            }

            // Battery sits outside the tabs: it is the one reading you want
            // regardless of which group of controls is open.
            BatteryRow {
                id: battery
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                width: implicitWidth
                bar: root.bar
                battery: root.device ? root.device.battery : null
            }
        }

        // With a tab strip present its rail is the divider; a separator
        // directly above it would just be a second line.
        PanelSeparator {
            foreground: root.foreground
            visible: !root.showTabs && (root.tabs.length > 0 || unsupportedList.visible)
        }

        // --- which group of controls ---------------------------------------
        TabBar {
            bar: root.bar
            visible: root.showTabs
            tabs: root.tabs
            current: root.currentTab
            onSelected: function (id) {
                root.tabSelected(id);
            }
        }

        // --- sensor --------------------------------------------------------
        Column {
            width: parent.width
            spacing: Style.space(10)
            visible: root.currentTab === "sensor"

            DpiControl {
                bar: root.bar
                busy: root.busy
                dpi: root.device ? root.device.dpi : null
                onRequested: function (value) {
                    root.settingRequested("dpi", String(value));
                }
            }

            PollingRateControl {
                bar: root.bar
                busy: root.busy
                pollingRate: root.device ? root.device.pollingRate : null
                onRequested: function (hz) {
                    root.settingRequested("polling-rate", String(hz));
                }
            }

            ProfileModeControl {
                bar: root.bar
                busy: root.busy
                mode: root.device ? root.device.onboardProfile : ""
                onRequested: function (mode) {
                    root.settingRequested("profile-mode", mode);
                }
            }
        }

        // --- triggers ------------------------------------------------------
        HitsControl {
            bar: root.bar
            busy: root.busy
            // A failed HITS read leaves the tab present but empty; guard so it
            // does not draw bare "Left click" headings over nothing.
            visible: root.currentTab === "triggers" && root.device && root.device.hits !== null
            hits: root.device ? root.device.hits : null
            onRequested: function (field, value) {
                root.settingRequested("hits-" + field, String(value));
            }
        }

        // --- declared, not driven ------------------------------------------
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
