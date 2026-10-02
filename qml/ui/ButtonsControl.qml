import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: what each button does.
//
// A row per button the device will let software reassign, each with a picker
// of what it may become. Both lists come from the device — which buttons are
// reprogrammable, and which targets each one admits — so a mouse with a
// different set of buttons grows the right rows without any change here.
//
// Left and right click are absent because the hardware does not offer them,
// which is also why no amount of fiddling here can leave a mouse that cannot
// click. "Default" is a real choice rather than the absence of one: it maps
// the button back to its own job.
Column {
    id: root

    property var buttons: []
    property QtObject bar: null
    property bool busy: false

    signal requested(string slug, string target)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(10)
    visible: buttons.length > 0
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    Repeater {
        model: root.buttons

        delegate: Column {
            id: row
            required property var modelData

            width: root.width
            spacing: Style.space(6)

            SettingHeader {
                bar: root.bar
                label: row.modelData.label
                value: Model.buttonAssignment(row.modelData)
            }

            Dropdown {
                width: parent.width
                showLabel: false
                fontFamily: root.fontFamily

                options: [
                    {
                        value: "default",
                        label: "Default"
                    }
                ].concat(row.modelData.targets.map(function (target) {
                    return {
                        value: target.slug,
                        label: target.label
                    };
                }))

                // A button doing its own job shows as Default rather than as
                // its own name, so the picker agrees with the line above it.
                value: row.modelData.isDefault ? "default" : row.modelData.mappedTo

                onChanged: function (value) {
                    root.requested(row.modelData.slug, value);
                }
            }
        }
    }
}
