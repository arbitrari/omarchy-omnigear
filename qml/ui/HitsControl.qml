import QtQuick
import qs.Commons
import "../Model.js" as Model

// Capability control: HITS — the analog left and right click.
//
// Each click has its own actuation point, rapid trigger and haptic strength,
// and the device takes them per button, so the two sides are laid out
// separately rather than pretending they move together.
Column {
    id: root

    property var hits: null
    property QtObject bar: null
    property bool busy: false

    // field is "<side>-<name>", e.g. "left-actuation" — the tail of the
    // setting key the CLI takes.
    signal requested(string field, int value)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property int step: hits && hits.step > 0 ? hits.step : 1

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(10)
    visible: hits !== null
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    Repeater {
        model: [
            { side: "left", label: "Left Click" },
            { side: "right", label: "Right Click" }
        ]

        delegate: Column {
            id: side
            required property var modelData

            readonly property var button: root.hits
                ? (modelData.side === "left" ? root.hits.left : root.hits.right)
                : null

            width: root.width
            spacing: Style.space(6)

            SettingHeader {
                bar: root.bar
                group: true
                label: side.modelData.label
            }

            TriggerSlider {
                bar: root.bar
                label: "Actuation"
                value: side.button ? side.button.actuation : 0
                maximum: root.hits ? root.hits.maxActuation : 0
                stepSize: root.step
                minimum: root.step
                onReleased: function (v) {
                    root.requested(side.modelData.side + "-actuation", v);
                }
            }

            TriggerSlider {
                bar: root.bar
                label: "Rapid Trigger"
                value: side.button ? side.button.rapidTrigger : 0
                maximum: root.hits ? root.hits.maxRapidTrigger : 0
                stepSize: root.step
                minimum: root.step
                onReleased: function (v) {
                    root.requested(side.modelData.side + "-rapid-trigger", v);
                }
            }

            TriggerSlider {
                bar: root.bar
                label: "Haptics"
                // Haptics is the one field with a meaningful off.
                minimum: 0
                stepSize: root.step
                value: side.button ? side.button.haptics : 0
                maximum: root.hits ? root.hits.maxHaptics : 0
                onReleased: function (v) {
                    root.requested(side.modelData.side + "-haptics", v);
                }
            }
        }
    }
}
