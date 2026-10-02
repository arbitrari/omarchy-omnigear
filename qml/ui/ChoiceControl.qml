import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: a setting with a handful of named values — auto power
// off, sidetone, gain, wireless mode.
//
// The choices are the device's own, listed by the driver, since models
// differ: an XM4 offers only never and when taken off for auto power off, a
// Nova Pro seven timers. A handful fit side by side as buttons; more go in a
// dropdown.
Column {
    id: root

    // The capability, which names the header.
    property string capability: ""
    // A Choice from the CLI: { current, options: [{ slug, label }] }.
    property var choice: null
    // Hover text for a choice, by slug. Optional.
    property var tooltips: ({})
    property QtObject bar: null
    property bool busy: false

    signal requested(string slug)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property var choices: choice ? choice.options.map(function (option) {
        return { value: option.slug, label: option.label, tooltip: root.tooltips[option.slug] || "" };
    }) : []
    readonly property string current: choice ? choice.current : ""

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(6)
    visible: choice !== null
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    SettingHeader {
        bar: root.bar
        group: true
        label: Model.capabilityLabel(root.capability)
        value: Model.choiceLabel(root.choice)
    }

    ButtonGroup {
        width: parent.width
        visible: root.choices.length <= 4
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
        visible: root.choices.length > 4
        showLabel: false
        fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
        options: root.choices
        value: root.current

        onChanged: function (value) {
            root.requested(value);
        }
    }
}
