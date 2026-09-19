import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: who owns the device's settings.
//
// Unlike every other control here, this one changes how the device behaves
// when OmniGear is not running — in software mode its stored profile stops
// applying. That is worth a sentence on screen rather than a silent toggle,
// and it is never flipped automatically to make some other write succeed.
Column {
    id: root

    property string mode: ""
    property QtObject bar: null
    property bool busy: false

    signal requested(string mode)

    readonly property color foreground: bar ? bar.foreground : Color.foreground

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(6)
    visible: mode !== ""
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    SettingHeader {
        bar: root.bar
        group: true
        label: Model.capabilityLabel("onboard-profile")
        value: Model.profileModeLabel(root.mode)
    }

    ButtonGroup {
        width: parent.width
        spacing: Style.space(4)

        foreground: root.foreground
        background: root.bar ? root.bar.background : Color.background
        accent: Color.accent
        fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
        fontSize: Style.font.bodySmall
        focusable: false

        options: [
            {
                value: "onboard",
                label: Model.profileModeLabel("onboard"),
                tooltip: "The device runs its own stored profile and refuses changes from here."
            },
            {
                value: "host",
                label: Model.profileModeLabel("host"),
                tooltip: "OmniGear can change settings. The device's stored profile stops applying."
            }
        ]
        value: root.mode

        onChanged: function (value) {
            root.requested(value);
        }
    }

    Text {
        width: parent.width
        visible: root.mode === "onboard"
        text: "Onboard settings are owned by the device. Switch to software to change them here."
        textFormat: Text.PlainText
        wrapMode: Text.WordWrap
        color: Qt.darker(root.foreground, 1.7)
        font.family: root.bar ? root.bar.fontFamily : Style.font.family
        font.pixelSize: Style.font.caption
    }
}
