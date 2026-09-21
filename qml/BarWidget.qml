import QtQuick
import Quickshell
import qs.Commons
import qs.Ui
import "Model.js" as Model
import "ui"

// The bar entry: one icon for the device that most needs attention, with every
// connected device in the tooltip. Clicking it opens the drill-down.
BarWidget {
    id: root
    moduleName: "io.github.arbitrari.omnigear"

    readonly property bool opened: panelLoader.item ? panelLoader.item.opened === true : false
    readonly property bool popoutSwitchClosing: panelLoader.item
        ? panelLoader.item.popoutSwitchClosing === true : false

    readonly property bool showPercentage: setting("showPercentage", true) === true
    readonly property string primaryMouseId: String(setting("primaryMouse", ""))

    implicitWidth: button.implicitWidth
    implicitHeight: button.implicitHeight

    onBarChanged: injectPanel()

    // An empty id means "decide for me", which the panel offers as an explicit
    // choice rather than as the absence of one.
    //
    // The key names the kind of device on purpose: a keyboard and a headset
    // will each want their own, and a single `primaryDevice` would have had to
    // be migrated away from later.
    function setPrimaryMouse(deviceId) {
        var next = Object.assign({}, root.settings, {
            primaryMouse: deviceId
        });
        // Shed the key this replaced, so it does not linger in shell.json.
        delete next.primaryDevice;
        root.settings = next;
        if (root.bar && root.bar.shell && typeof root.bar.shell.updateEntryInline === "function")
            root.bar.shell.updateEntryInline(root.moduleName, root.settings);
    }

    function togglePercentage() {
        root.settings = Object.assign({}, root.settings, {
            showPercentage: !root.showPercentage
        });
        if (root.bar && root.bar.shell && typeof root.bar.shell.updateEntryInline === "function")
            root.bar.shell.updateEntryInline(root.moduleName, root.settings);
    }

    // The panel is a sibling component, so the bar has to hand it everything it
    // needs: where to anchor, who owns it, and the one Service instance. Two
    // services would mean two poll timers and two answers.
    function injectPanel() {
        if (!panelLoader.item)
            return;
        panelLoader.item.bar = root.bar;
        panelLoader.item.anchorItem = button;
        panelLoader.item.hostWidget = root;
        panelLoader.item.gear = gear;
        panelLoader.item.primaryMouseId = Qt.binding(function () {
            return root.primaryMouseId;
        });
    }

    function open() {
        if (panelLoader.item)
            panelLoader.item.open();
    }

    function close() {
        if (panelLoader.item)
            panelLoader.item.close();
    }

    function toggle() {
        if (panelLoader.item)
            panelLoader.item.toggle();
    }

    function closeForPopoutSwitch() {
        if (panelLoader.item)
            panelLoader.item.closeForPopoutSwitch();
    }

    Service {
        id: gear
        settings: root.settings
    }

    Loader {
        id: panelLoader
        active: true
        source: Qt.resolvedUrl("DevicePanel.qml")
        visible: false
        onLoaded: {
            root.injectPanel();
            Qt.callLater(root.injectPanel);
        }
    }

    BarIconButton {
        id: button
        anchors.fill: parent
        bar: root.bar
        active: root.opened
        useActiveColor: false
        slotSize: Style.bar.iconSlot * Model.barSlots(gear.primary, root.showPercentage)
        tooltipText: Model.tooltip(gear.state)

        // Drawn rather than set as text: the charging bolt is smaller than the
        // device glyph beside it, and a plain string is one font size all the
        // way through.
        text: ""
        iconComponent: Component {
            Item {
                anchors.fill: parent

                Row {
                    anchors.centerIn: parent
                    spacing: Style.space(4)

                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: text !== ""
                        text: Model.barCharge(gear.primary, root.showPercentage)
                        textFormat: Text.PlainText
                        color: button.foreground
                        font.family: button.fontFamily
                        font.pixelSize: button.fontSize
                    }

                    Item {
                        anchors.verticalCenter: parent.verticalCenter
                        width: glyph.implicitWidth + (bolt.visible ? bolt.implicitWidth : 0)
                        height: glyph.implicitHeight

                        // The same component the panel cards use, so a device
                        // with a look of its own keeps it here. `size` is a
                        // font pixel size, so this lands exactly where the
                        // plain glyph did.
                        DeviceIcon {
                            id: glyph
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            bar: root.bar
                            device: gear.primary
                            size: button.fontSize
                        }

                        Text {
                            id: bolt
                            anchors.left: glyph.right
                            // Sits high against the glyph, the way a badge
                            // does, rather than centred beside it.
                            anchors.top: glyph.top
                            anchors.topMargin: Math.round(button.fontSize * 0.1)
                            visible: Model.isCharging(gear.primary)
                            text: Model.CHARGING_BOLT
                            textFormat: Text.PlainText
                            color: button.foreground
                            font.family: button.fontFamily
                            font.pixelSize: Math.round(button.fontSize * 0.55)
                        }
                    }
                }
            }
        }

        onPressed: function (code) {
            if (code === Qt.LeftButton)
                root.toggle();
            else if (code === Qt.RightButton)
                root.togglePercentage();
        }
    }
}
