import QtQuick
import qs.Commons
import "../Model.js" as Model

// One labelled, bounded slider: the shape every HITS field takes.
//
// Values are the device's own units. It reports a ceiling for each field and
// nothing about what a unit means, so the reading is shown as "16 / 40" rather
// than invented millimetres.
Column {
    id: root

    property string label: ""
    property int value: 0
    property int minimum: 1
    property int maximum: 0
    // The device stores in fixed increments and rounds anything else without
    // saying so, so the slider is only allowed to land on the grid.
    property int stepSize: 1
    property QtObject bar: null

    signal released(int value)

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(4)
    visible: maximum > 0

    SettingHeader {
        bar: root.bar
        label: root.label
        value: root.value + " / " + root.maximum
    }

    SquaredSlider {
        width: parent.width
        height: Style.spacing.controlHeight
        bar: root.bar
        minimum: root.minimum
        maximum: root.maximum
        value: root.value
        step: root.stepSize
        integer: true
        onReleased: function (v) {
            var snapped = Math.round(v / root.stepSize) * root.stepSize;
            root.released(Math.max(root.minimum, Math.min(root.maximum, snapped)));
        }
    }
}
