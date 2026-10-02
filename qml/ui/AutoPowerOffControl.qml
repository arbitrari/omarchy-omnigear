import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: when a headset switches itself off.
//
// The choices are the headset's own, listed by the driver, since models
// differ: an XM4 offers only never and when taken off, an XM3 four timers and
// never. A handful fit side by side as buttons; more go in a dropdown.
Column {
    id: root

    property var autoPowerOff: null
    property QtObject bar: null
    property bool busy: false

    signal requested(string choice)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property var choices: autoPowerOff ? autoPowerOff.options.map(function (option) {
        return { value: option.slug, label: option.label };
    }) : []
    readonly property string current: autoPowerOff ? autoPowerOff.current : ""

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(6)
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    function currentLabel() {
        if (!autoPowerOff) return "";
        for (var i = 0; i < autoPowerOff.options.length; i++) {
            if (autoPowerOff.options[i].slug === autoPowerOff.current)
                return autoPowerOff.options[i].label;
        }
        return "";
    }

    SettingHeader {
        bar: root.bar
        group: true
        label: Model.capabilityLabel("auto-power-off")
        value: root.currentLabel()
    }

    ButtonGroup {
        width: parent.width
        visible: root.choices.length <= 3
        spacing: Style.space(4)

        foreground: root.foreground
        background: root.bar ? root.bar.background : Color.background
        accent: Color.accent
        fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
        fontSize: Style.font.bodySmall
        focusable: false

        options: root.choices
        value: root.current

        onChanged: function (value) {
            root.requested(value);
        }
    }

    Dropdown {
        width: parent.width
        visible: root.choices.length > 3
        showLabel: false
        fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
        options: root.choices
        value: root.current

        onChanged: function (value) {
            root.requested(value);
        }
    }
}
