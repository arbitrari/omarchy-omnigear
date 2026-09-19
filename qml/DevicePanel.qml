import QtQuick
import QtQuick.Controls
import Quickshell
import qs.Commons
import qs.Ui
import "Model.js" as Model
import "ui"

// The drill-down: every connected device, grouped the way the README groups
// them, each rendering the controls its capabilities call for.
//
// The panel owns the one Service and passes changes down; cards emit requests
// and never touch hardware. That keeps writes in a single place, which is what
// makes "show what the device actually did" possible rather than optimistic.
Panel {
    id: root
    moduleName: "io.github.arbitrari.omnigear"

    // This panel is summoned by its bar widget, not over IPC. A handler
    // declared here does not register anyway: the shell only wires IPC for a
    // plugin's entry-point component, and this one is reached through a Loader.
    manageIpc: false

    property var anchorItem: null
    property var hostWidget: null
    property var gear: null

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

    readonly property var state: gear ? gear.state : Model.emptyState()
    readonly property var groups: Model.groups(state.devices)
    readonly property bool busy: gear ? gear.busy : false

    // The last write's outcome, shown until the next one starts. A failed
    // write is the interesting case: the device can accept a change and keep
    // its old value, and the panel has to say so.
    property string status: ""
    property bool statusIsError: false

    function open() {
        root.controller.show();
        if (gear)
            gear.refresh();
    }

    function close() {
        root.controller.hide();
    }

    function toggle() {
        root.opened ? root.close() : root.open();
    }

    function switchPanel(direction) {
        if (root.bar && typeof root.bar.switchPanelFrom === "function")
            return root.bar.switchPanelFrom(root.hostWidget || root, direction);
        return false;
    }

    function applySetting(deviceId, key, value) {
        root.status = "";
        if (gear)
            gear.apply(deviceId, key, value);
    }

    Connections {
        target: root.gear
        enabled: root.gear !== null

        function onApplied(deviceId, key, success, message) {
            root.statusIsError = !success;
            root.status = success ? "" : (message || "The device refused the change");
        }
    }

    onOpenedChanged: if (opened) {
        if (flick)
            flick.contentY = 0;
        root.status = "";
        if (gear)
            gear.refresh();
        Qt.callLater(function () {
            keys.forceActiveFocus();
        });
    }

    KeyboardPanel {
        id: popup
        anchorItem: root.anchorItem
        owner: root.hostWidget || root
        bar: root.bar
        open: root.opened
        focusTarget: keys
        contentWidth: popup.fittedContentWidth(Style.space(400))
        contentHeight: popup.fittedContentHeight(content.implicitHeight, Style.space(720))

        PanelKeyCatcher {
            id: keys
            anchors.fill: parent
            onCloseRequested: root.close()
            onTabRequested: function (direction) {
                root.switchPanel(direction);
            }

            Flickable {
                id: flick
                anchors.fill: parent
                contentWidth: width
                contentHeight: content.implicitHeight
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                flickableDirection: Flickable.VerticalFlick
                interactive: contentHeight > height
                ScrollBar.vertical: ScrollBar {
                    policy: ScrollBar.AsNeeded
                }

                Column {
                    id: content
                    width: flick.width - (flick.interactive ? Style.space(8) : 0)
                    spacing: Style.space(12)

                    // --- title ---------------------------------------------
                    Item {
                        width: parent.width
                        implicitHeight: title.implicitHeight

                        Text {
                            id: title
                            anchors.left: parent.left
                            text: "OMNIGEAR"
                            textFormat: Text.PlainText
                            color: root.foreground
                            font.family: root.fontFamily
                            font.pixelSize: Style.font.subtitle
                            font.bold: true
                        }

                        Text {
                            anchors.right: parent.right
                            anchors.verticalCenter: title.verticalCenter
                            text: root.busy ? "reading…" : ""
                            textFormat: Text.PlainText
                            color: Qt.darker(root.foreground, 1.6)
                            font.family: root.fontFamily
                            font.pixelSize: Style.font.caption
                        }
                    }

                    // --- the devices ---------------------------------------
                    Repeater {
                        model: root.groups

                        delegate: Column {
                            id: group
                            required property var modelData

                            width: content.width
                            spacing: Style.space(8)

                            PanelSectionHeader {
                                foreground: root.foreground
                                fontFamily: root.fontFamily
                                text: group.modelData.label
                            }

                            Repeater {
                                model: group.modelData.devices

                                delegate: DeviceCard {
                                    required property var modelData

                                    width: group.width
                                    bar: root.bar
                                    busy: root.busy
                                    device: modelData
                                    onSettingRequested: function (key, value) {
                                        root.applySetting(modelData.id, key, value);
                                    }
                                }
                            }
                        }
                    }

                    // --- nothing to show -----------------------------------
                    Text {
                        width: parent.width
                        visible: root.state.ok && root.state.devices.length === 0
                        text: "No supported devices connected.\nSee the README for what is catalogued."
                        textFormat: Text.PlainText
                        wrapMode: Text.WordWrap
                        color: Qt.darker(root.foreground, 1.4)
                        font.family: root.fontFamily
                        font.pixelSize: Style.font.bodySmall
                    }

                    // --- the CLI itself failed -----------------------------
                    Text {
                        width: parent.width
                        visible: !root.state.ok && root.state.error !== ""
                        text: root.state.error
                        textFormat: Text.PlainText
                        wrapMode: Text.WordWrap
                        color: root.bar ? root.bar.urgent : Color.urgent
                        font.family: root.fontFamily
                        font.pixelSize: Style.font.bodySmall
                    }

                    // --- outcome of the last write -------------------------
                    Text {
                        width: parent.width
                        visible: root.status !== ""
                        text: root.status
                        textFormat: Text.PlainText
                        wrapMode: Text.WordWrap
                        color: root.statusIsError
                            ? (root.bar ? root.bar.urgent : Color.urgent)
                            : Qt.darker(root.foreground, 1.4)
                        font.family: root.fontFamily
                        font.pixelSize: Style.font.caption
                    }
                }
            }
        }
    }
}
