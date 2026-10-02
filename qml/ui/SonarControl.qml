import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: Sonar — separate outputs for games, chat, media and
// the rest, with the headset's ChatMix dial balancing game against chat.
//
// The switch restarts the sound server, which is said on it rather than left
// to surprise anyone: every stream drops for a moment. The dial's position
// comes from a listener the service keeps running, not from the device's
// read, because the base station only reports the dial as it turns.
Column {
    id: root

    property var sonar: null
    // { game, chat, error } from the ChatMix listener, or null until the dial
    // has moved since the listener started.
    property var chatMix: null
    property QtObject bar: null
    property bool busy: false

    signal requested(bool on)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property color muted: Qt.darker(foreground, 1.4)
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family
    readonly property bool enabledNow: sonar !== null && sonar.enabled

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(10)
    visible: sonar !== null
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    Column {
        width: parent.width
        spacing: Style.space(6)

        SettingHeader {
            bar: root.bar
            group: true
            label: Model.capabilityLabel("sonar")
            value: root.enabledNow ? "On" : "Off"
        }

        ButtonGroup {
            width: parent.width
            spacing: Style.space(4)

            foreground: root.foreground
            background: root.bar ? root.bar.background : Color.background
            accent: Color.accent
            fontFamily: root.fontFamily
            fontSize: Style.font.bodySmall
            focusable: false

            options: [
                { value: "off", label: "Off", tooltip: "One output. Restarts audio for a moment." },
                { value: "on", label: "On", tooltip: "Game, Chat, Media and Aux outputs. Restarts audio for a moment." }
            ]
            value: root.enabledNow ? "on" : "off"

            onChanged: function (value) {
                root.requested(value === "on");
            }
        }

        Text {
            width: parent.width
            visible: root.enabledNow && !root.sonar.live
            text: "On, but the sound server has not made the outputs. They appear when the headset is plugged in."
            wrapMode: Text.WordWrap
            textFormat: Text.PlainText
            color: root.muted
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
        }
    }

    Column {
        width: parent.width
        spacing: Style.space(6)
        visible: root.enabledNow

        SettingHeader {
            bar: root.bar
            label: "ChatMix"
            value: root.chatMix
                ? "Game " + root.chatMix.game + "% · Chat " + root.chatMix.chat + "%"
                : ""
        }

        // Game on the left, chat on the right, as the dial turns.
        Row {
            width: parent.width
            height: Style.space(4)
            spacing: Style.space(2)
            visible: root.chatMix !== null

            Rectangle {
                width: (parent.width - parent.spacing) / 2
                height: parent.height
                color: Qt.darker(root.foreground, 3)

                Rectangle {
                    anchors.right: parent.right
                    width: parent.width * (root.chatMix ? root.chatMix.game : 0) / 100
                    height: parent.height
                    color: root.foreground
                }
            }

            Rectangle {
                width: (parent.width - parent.spacing) / 2
                height: parent.height
                color: Qt.darker(root.foreground, 3)

                Rectangle {
                    anchors.left: parent.left
                    width: parent.width * (root.chatMix ? root.chatMix.chat : 0) / 100
                    height: parent.height
                    color: root.foreground
                }
            }
        }

        Text {
            width: parent.width
            visible: root.chatMix === null || (root.chatMix.error || "") !== ""
            text: root.chatMix === null
                ? "Turn the ChatMix dial on the base station to balance Game against Chat."
                : root.chatMix.error
            wrapMode: Text.WordWrap
            textFormat: Text.PlainText
            color: root.chatMix !== null ? (root.bar ? root.bar.urgent : Color.urgent) : root.muted
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
        }
    }

    Column {
        width: parent.width
        spacing: Style.space(4)
        visible: root.enabledNow

        SettingHeader {
            bar: root.bar
            label: "Outputs"
        }

        Repeater {
            model: root.sonar ? root.sonar.channels : []

            delegate: Item {
                required property var modelData

                width: parent.width
                implicitHeight: name.implicitHeight

                Text {
                    id: name
                    anchors.left: parent.left
                    text: "OmniGear " + modelData.label
                    textFormat: Text.PlainText
                    color: root.foreground
                    font.family: root.fontFamily
                    font.pixelSize: Style.font.bodySmall
                }

                Text {
                    anchors.right: parent.right
                    anchors.verticalCenter: name.verticalCenter
                    text: modelData.mixed ? "On The Dial" : ""
                    textFormat: Text.PlainText
                    color: root.muted
                    font.family: root.fontFamily
                    font.pixelSize: Style.font.caption
                }
            }
        }

        Text {
            width: parent.width
            text: "Pick an output for each app in your sound settings. New apps play to Game."
            wrapMode: Text.WordWrap
            textFormat: Text.PlainText
            color: root.muted
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
        }
    }
}
