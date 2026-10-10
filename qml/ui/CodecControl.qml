import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: which Bluetooth codec the audio plays over.
//
// This is this computer's choice, not the headset's — the sound server picks
// a codec both ends support — so it is set here and forgotten by the headset.
// Switching drops the audio for a moment while the link renegotiates.
Column {
    id: root

    property var codec: null
    property QtObject bar: null
    property bool busy: false

    signal requested(string codec)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(6)
    visible: codec !== null && codec.options.length > 0
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    SettingHeader {
        bar: root.bar
        group: true
        label: Model.capabilityLabel("codec")
        value: Model.codecLabel(root.codec)
    }

    Dropdown {
        width: parent.width
        showLabel: false
        fontFamily: root.fontFamily
        options: root.codec ? root.codec.options.map(function (option) {
            return { value: option.slug, label: option.label };
        }) : []
        value: root.codec ? root.codec.current : ""

        onChanged: function (value) {
            root.requested(value);
        }
    }

    Text {
        width: parent.width
        text: "Set by this computer, not stored in the headset. "
            + "The audio drops out briefly while it switches."
        textFormat: Text.PlainText
        wrapMode: Text.WordWrap
        color: Qt.darker(root.foreground, 1.7)
        font.family: root.fontFamily
        font.pixelSize: Style.font.caption
    }
}
