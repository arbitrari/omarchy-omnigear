import QtQuick
import qs.Commons
import qs.Ui
import "Model.js" as Model

// The bar entry: one icon for the device that most needs attention, with every
// connected device in the tooltip. The drill-down panel comes next.
BarWidget {
    id: root
    moduleName: "io.github.arbitrari.omnigear"

    readonly property bool showPercentage: setting("showPercentage", true) === true

    implicitWidth: button.implicitWidth
    implicitHeight: button.implicitHeight

    function togglePercentage() {
        root.settings = Object.assign({}, root.settings, {
            showPercentage: !root.showPercentage
        });
        if (root.bar && root.bar.shell && typeof root.bar.shell.updateEntryInline === "function")
            root.bar.shell.updateEntryInline(root.moduleName, root.settings);
    }

    Service {
        id: gear
        settings: root.settings
    }

    BarIconButton {
        id: button
        anchors.fill: parent
        bar: root.bar
        active: false
        useActiveColor: false
        slotSize: Style.bar.iconSlot * (root.showPercentage && gear.primary ? 2.2 : 1.0)
        tooltipText: Model.tooltip(gear.state)

        text: {
            var device = gear.primary;
            if (!device)
                return Model.categoryIcon("");

            var icon = Model.categoryIcon(device.category);
            if (!root.showPercentage || !device.battery)
                return icon;

            return Model.batteryText(device.battery.percent) + " " + icon;
        }

        onPressed: function (code) {
            if (code === Qt.LeftButton)
                gear.refresh();
            else if (code === Qt.RightButton)
                root.togglePercentage();
        }
    }
}
