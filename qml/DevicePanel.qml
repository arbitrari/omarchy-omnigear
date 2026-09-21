import QtQuick
import QtQuick.Controls
import QtQuick.Effects
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
    readonly property var conflicts: Model.conflictSummary(state.devices)
    readonly property bool busy: gear ? gear.busy : false
    readonly property var build: gear ? gear.build : ({ label: "" })

    // The last write's outcome, shown until the next one starts. A failed
    // write is the interesting case: the device can accept a change and keep
    // its old value, and the panel has to say so.
    property string status: ""
    property bool statusIsError: false

    // Which tab each device has open, by device id. Kept here because the
    // panel outlives the cards: their Repeater rebuilds on every poll.
    property var tabSelection: ({})

    // Which mouse the user picked to speak for the bar. Owned by the bar
    // widget, because that is where the widget's settings live.
    property string primaryMouseId: ""

    // Space held back down the right for the scrollbar.
    //
    // Reserved always, not only while the list happens to overflow. Taking it
    // from the content on demand made the cards change width the moment a tab
    // grew tall enough to scroll — switching to Wheel visibly resized the
    // whole section.
    readonly property int gutter: Style.space(8)

    readonly property int mouseCount: Model.devicesOfCategory(state.devices, "mouse").length

    function choosePrimaryMouse(deviceId) {
        if (hostWidget && typeof hostWidget.setPrimaryMouse === "function")
            hostWidget.setPrimaryMouse(deviceId);
    }

    // Hands a device to the CLI to be written up, and opens the prefilled
    // issue in the browser. The panel stays open behind it.
    function requestReport(deviceId) {
        if (gear)
            gear.report(deviceId);
    }

    function tabFor(deviceId) {
        return tabSelection[deviceId] || "";
    }

    // Which devices are folded down to their header, by device id. Kept here
    // for the same reason as the tab selection: the cards are rebuilt on every
    // poll, so state held in them does not survive.
    property var collapsed: ({})

    function isCollapsed(deviceId) {
        return collapsed[deviceId] === true;
    }

    function toggleCollapsed(deviceId) {
        var next = {};
        for (var id in collapsed)
            next[id] = collapsed[id];
        next[deviceId] = !next[deviceId];
        collapsed = next;
    }

    function selectTab(deviceId, tabId) {
        var next = {};
        for (var id in tabSelection)
            next[id] = tabSelection[id];
        next[deviceId] = tabId;
        tabSelection = next;
    }

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

        // A report that cannot be written is worth saying out loud: the user
        // clicked a button and a browser did not open, which otherwise looks
        // like the button being broken.
        function onReportFailed(message) {
            root.statusIsError = true;
            root.status = message;
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
        contentHeight: popup.fittedContentHeight(
            header.height - header.inkTop + content.anchors.topMargin
            + content.implicitHeight
            + (footer.visible ? footer.height + Style.space(12) : 0), Style.space(720))

        PanelKeyCatcher {
            id: keys
            anchors.fill: parent
            onCloseRequested: root.close()
            onTabRequested: function (direction) {
                root.switchPanel(direction);
            }

            // --- title ------------------------------------------------
            //
            // Outside the Flickable on purpose. The wordmark is pulled left so
            // its letters line up with the cards, which puts its shade ramp at
            // a negative x — inside the scroller that gets clipped away, out
            // here it bleeds harmlessly into the popup's padding.
            Item {
                id: header
                anchors.top: parent.top
                // Hang the letters, not the artwork's blank strip, off the top
                // of the panel.
                anchors.topMargin: -header.inkTop
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.rightMargin: root.gutter
                height: Math.max(header.inkBottom, refresh.y + refresh.height)

                // Where the ink starts and stops, as opposed to where the
                // items do. The artwork is a fifth empty beyond its ink at
                // each end, and the build line's descent is leading rather
                // than anything drawn, so neither belongs in the block the
                // rest of the header is measured and aligned against.
                readonly property real inkTop: wordmark.inkPadding * wordmark.unitPx
                readonly property real inkBottom: buildLine.visible
                    ? buildLine.y + buildMetrics.ascent
                    : wordmark.height - header.inkTop

                // What the build line needs to come up by to sit against the
                // letters, leaving a hair.
                readonly property real buildOffset: Style.space(1) - header.inkTop

                // The wordmark is block art on a 62.5 x 10 grid of square
                // pixels, so a height that is a multiple of 10 lands every
                // block on whole device pixels. 20 gives 2px blocks and
                // letters about as tall as the text this replaced.
                Item {
                    id: wordmark

                    // The artwork carries its own padding: in the 625x100
                    // viewBox the first letter pixel sits 35 units in, with
                    // the shade ramp starting at 20. Pull the mark left by the
                    // letter inset so the O lines up with the cards below.
                    readonly property real letterInset: 35
                    readonly property real unitPx: height / 100

                    // The same padding vertically: the ink runs from y 20
                    // to y 80 of the 100-unit viewBox, so a fifth of the
                    // item's height is blank above the letters and a fifth
                    // below them.
                    readonly property real inkPadding: 20

                    anchors.left: parent.left
                    anchors.leftMargin: -Math.round(letterInset * unitPx)
                    anchors.top: parent.top
                    height: Style.space(20)
                    width: Math.round(height * (625 / 100))

                    // Drawn from the white artwork and tinted, so the wordmark
                    // follows the theme instead of being fixed black or white.
                    // Hidden behind the effect, which samples it as a texture.
                    Image {
                        id: wordmarkArt
                        anchors.fill: parent
                        source: Qt.resolvedUrl("../assets/omnigear-logo-white.svg")
                        sourceSize.width: Math.round(width * Screen.devicePixelRatio)
                        sourceSize.height: Math.round(height * Screen.devicePixelRatio)
                        fillMode: Image.PreserveAspectFit
                        visible: false
                        layer.enabled: true
                    }

                    MultiEffect {
                        anchors.fill: wordmarkArt
                        source: wordmarkArt
                        colorization: 1.0
                        colorizationColor: root.foreground
                    }
                }

                // Reads devices again on demand, rather than waiting out
                // the poll interval — useful the moment a mouse is switched on
                // or moved to another connection.
                //
                // It also stands in for the old "reading…" label: the shared
                // Button spins its icon while a read is out, which says the
                // same thing in the space the button already occupies.
                Button {
                    id: refresh
                    anchors.right: parent.right

                    // Centred on the letters and the line beneath them taken
                    // together, which is what reads as the title now — not on
                    // the wordmark alone, which left it sitting high.
                    y: Math.round((header.inkTop + header.inkBottom - height) / 2)

                    // Square on the button's own height, which for a lone
                    // icon is the larger of the two implicit sizes anyway.
                    width: height
                    height: implicitHeight

                    bordered: true
                    iconText: "󰑐"
                    // The icon font, as every other glyph here uses: the
                    // theme's text font need not carry Nerd Font glyphs.
                    fontFamily: "monospace"
                    // The icon token rather than a text size: it is the one
                    // themes tune for glyphs, and it sizes the square with it.
                    iconSize: Style.font.icon
                    foreground: root.foreground
                    background: root.bar ? root.bar.background : Color.background
                    accent: Color.accent
                    tooltipText: "Read devices again"
                    iconSpinning: root.busy

                    onClicked: if (root.gear) root.gear.refresh()
                }

                // What this copy is: its version, or — off the mainline — the
                // branch and commit it was built from, so a panel running a
                // work in progress says so rather than passing for a release.
                //
                // Aligned to the header's own left edge, which is where the
                // cards start: the wordmark above is pulled further left by its
                // artwork padding, and matching that would put this out past
                // everything else.
                Text {
                    id: buildLine
                    anchors.top: wordmark.bottom
                    anchors.topMargin: header.buildOffset
                    anchors.left: parent.left
                    anchors.right: parent.right

                    text: root.build ? root.build.label : ""
                    visible: text !== ""
                    textFormat: Text.PlainText
                    elide: Text.ElideRight
                    color: Qt.darker(root.foreground, 1.7)
                    font.family: root.fontFamily
                    font.pixelSize: Style.font.bodySmall
                }

                FontMetrics {
                    id: buildMetrics
                    font: buildLine.font
                }
            }

            // --- which device speaks for the bar ------------------------
            //
            // A panel-level choice rather than a per-device one: it is a
            // preference about the bar, not a setting on any mouse, so it
            // lives once at the foot of the panel instead of repeating as an
            // ornament on every card.
            //
            // One chooser per kind of device. Only mice report a battery to
            // the bar today; a keyboard or headset that does will add its own
            // chooser here rather than change this one.
            Item {
                id: footer
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.rightMargin: root.gutter
                anchors.bottom: parent.bottom
                // Computed from the model, not from the chooser's own
                // visibility: a parent whose visibility depends on its child's
                // does not re-evaluate when that child's data arrives.
                visible: root.mouseCount > 1
                height: visible ? footerBody.implicitHeight : 0

                Column {
                    id: footerBody
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    spacing: Style.space(8)

                    PanelSeparator {
                        foreground: root.foreground
                    }

                    PrimaryChooser {
                        id: mouseChooser
                        bar: root.bar
                        label: "Primary Mouse"
                        devices: Model.devicesOfCategory(root.state.devices, "mouse")
                        current: root.primaryMouseId
                        onChosen: function (deviceId) {
                            root.choosePrimaryMouse(deviceId);
                        }
                    }
                }
            }

            Flickable {
                id: flick
                anchors.top: header.bottom
                anchors.topMargin: Style.space(buildLine.visible ? 5 : 12)
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.bottom: footer.visible ? footer.top : parent.bottom
                anchors.bottomMargin: footer.visible ? Style.space(12) : 0
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
                    width: flick.width - root.gutter
                    spacing: Style.space(12)

                    // --- present, but we cannot open it --------------------
                    AccessCard {
                        bar: root.bar
                        paths: root.state.unreadable || []
                        visible: (root.state.unreadable || []).length > 0
                    }

                    // --- something else is driving these devices -----------
                    //
                    // Top of the panel, above the cards, because it explains
                    // the cards: a setting that will not stick has no other
                    // visible cause, and the device reports no error for it.
                    ConflictCard {
                        bar: root.bar
                        conflicts: root.conflicts
                        visible: root.conflicts.length > 0
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
                                    activeTab: root.tabFor(modelData.id)
                                    collapsed: root.isCollapsed(modelData.id)
                                    onCollapseToggled: root.toggleCollapsed(modelData.id)
                                    onSettingRequested: function (key, value) {
                                        root.applySetting(modelData.id, key, value);
                                    }
                                    onTabSelected: function (id) {
                                        root.selectTab(modelData.id, id);
                                    }
                                    onReportRequested: {
                                        root.requestReport(modelData.id);
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
