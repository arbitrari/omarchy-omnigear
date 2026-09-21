
<picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/omnigear-logo-white.png">
    <source media="(prefers-color-scheme: light)" srcset="assets/omnigear-logo-black.png">
    <img src="assets/omnigear-logo-black.png" alt="OmniGear" width="350" style="margin-left: -16px">
</picture>

___

An [Omarchy](https://omarchy/org) Plugin for monitoring and configuring your peripherals in a brand-agnostic way. No longer do you need a seperate plugin for your mouse, keyboard, and headsets; OmniGear handles them all.

## Installation

`omarchy plugin add omni.arbirtari.dev --enable`

## Updates

Updates can be done directly in the plugin.

**OR**

`omarchy plugin update io.github.arbitrari.omnigear`

## Supported Devices

> [!NOTE]   
> 🟩 = Fully Supported   
> 🟨 = Partially Supported   
> 🟥 = Not Supported, but Planned   
> ⬛ = Not Applicable _(the device does not have this feature)_

### Mice

<details>
<summary><b>Logitech</b></summary>

|  | Name | Battery | DPI | Polling | HITS* | SmartShift** | Notes 
|------|-----|-----|-----|-----|-----|-----|-----|
| 🟩 | PRO X2 SUPERSTRIKE  |🟩|🟩|🟩|🟩| N/A |Not configurable when using Onboard memory|
| 🟥 | PRO X2 SUPERLIGHT 2 | | | |⬛| | |
| 🟥 | PRO X2 SUPERLIGHT   | | | |⬛| | |
| 🟥 | PRO 2 LIGHTSPEED    | | | |⬛| | |
| 🟥 | G502 X / PLUS       | | | |⬛| | |
| 🟥 | G309 LIGHTSPEED     | | | |⬛| | |
| 🟥 | G305                | | | |⬛| | |
| 🟥 | MX Master 4         | | | |⬛| | |
| 🟨 | MX Master 3S        |🟩|🟩|⬛|⬛|🟩| |
| 🟥 | MX Master 3         | | | |⬛| | |
| 🟥 | MX Master 2         | | | |⬛| | |
| 🟥 | MX Master           | | | |⬛| | |
 
***Haptic Inductive Trigger System:** configurable actuation points and haptic feedback for left and right click. In the plugin, this appears as a Triggers tab in the SUPERSTRIKE's Panel

****Smartshift:** ability for the scroll wheel to automatically switch between ratcheting and smooth scroll. In the plugin, this appears as a Wheel tab in the XM Master family's Panel

</details>

<details>
<summary><b>Razer</b></summary>

|  | Name | Battery | DPI | Polling | Notes 
|------|-----|-----|-----|-----|-----|
| 🟥 | DeathAdder V4 Pro  | | | | |
| 🟥 | DeathAdder V3 Pro  | | | | |
| 🟥 | DeathAdder V3 HS  | | | | |
| 🟥 | DeathAdder V3  | | | | |
| 🟥 | DeathAdder V2 X HS  | | | | |
| 🟥 | Viper V4 Pro | | | | N/A |
| 🟥 | Viper V3 Pro SE | | | | N/A |
| 🟥 | Viper V3 Pro | | | | N/A |
| 🟥 | Naga V3 Pro   | | | | N/A |
| 🟥 | Naga V2 Pro   | | | | N/A |
| 🟥 | Naga V2 HS   | | | | N/A |
| 🟥 | Basilisk V3 Pro   | | | | N/A |

</details>

<details>
<summary><b>Corsair</b></summary>
</details>

<details>
<summary><b>Glorious</b></summary>
</details>

<details>
<summary><b>FinalMouse</b></summary>
</details>

### Keyboards

<details>
<summary><b>Logitech</b></summary>
</details>

<details>
<summary><b>Razer</b></summary>
</details>

<details>
<summary><b>Wooting</b></summary>
</details>

<details>
<summary><b>Keychron</b></summary>
</details>

<details>
<summary><b>Steelseries</b></summary>
</details>

<details>
<summary><b>ASUS ROG</b></summary>
</details>

<details>
<summary><b>Pulsar</b></summary>
</details>

<details>
<summary><b>Glorious</b></summary>
</details>

<details>
<summary><b>Lofree</b></summary>
</details>

<details>
<summary><b>HyperX</b></summary>
</details>

<details>
<summary><b>8BitDo</b></summary>
</details>

### Headsets / Headphones

<details>
<summary><b>Steelseries</b></summary>
</details>

<details>
<summary><b>Logitech</b></summary>
</details>

<details>
<summary><b>Razer</b></summary>
</details>

<details>
<summary><b>HyperX</b></summary>
</details>

<details>
<summary><b>Turtle Beach</b></summary>
</details>

<details>
<summary><b>Sony</b></summary>
</details>

<details>
<summary><b>Sennheizer</b></summary>
</details>

<details>
<summary><b>Audio-Technica</b></summary>
</details>

<details>
<summary><b>Apple</b></summary>
</details>

<details>
<summary><b>Google</b></summary>
</details>

<details>
<summary><b>Nothing</b></summary>
</details>

<details>
<summary><b>OnePlus</b></summary>
</details>

## Contribute

Is your device not currently supported? Feel free to submit a Pull Request to add support for it!

Use of Agents such as Claude Code, Codex, Cursor, Grok, Opencode, etc is encouraged. That said, _please_ make sure to keep PRs concise. Also, _please_ test all changes made as the maintainers most likely do not have the same device to test it themselves.

### Getting Started

You need [mise](https://mise.jdx.dev) (it pulls in Go) and the device you are adding. Read [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) first. It explains where a model lives in the tree, why a capability list is all the UI needs, and the hardware quirks that otherwise cost you an evening.

### Testing Changes

As you make frequent changes, you will want to build the plugin. To make this easy, run the following in terminal:

```
./scripts/dev-install
```
That will build the plugin and load it into the current shell

### Adding a Device

Most mice are one catalog entry and no new code, since the driver for the family already exists.

1. Plug the device in and run `./bin/omnigear probe`. It lists every hidraw node and which HID++ features actually answered. Never copy a USB id off the internet. A wrong one binds a driver to somebody else's hardware.
2. Add a `model.Entry` to `internal/devices/<category>/<brand>/<brand>.go`, with the ids you observed and only the capabilities you have seen work.
3. Set `Support` honestly. `SupportPartial` until every capability it claims works.
4. Update the table in this README to match.

A model that behaves unlike the rest of its family gets its own file next to the catalog. A brand that speaks a protocol nothing else here speaks needs a `transport/` package and a `drivers/` package as well, and that is a much bigger PR. Say so in the description and it can be reviewed in pieces.

### Reporting a Bug

Open an Issue with the output of `probe` and `list`, taken with the device connected. The binary ships inside the installed plugin:

```
cd ~/.config/omarchy/plugins/io.github.arbitrari.omnigear
./bin/omnigear probe
./bin/omnigear list
```

Serials are in that output, so scrub them if you would rather not publish them. The rest is what makes the report fixable.

## Disclaimer
OmniGear is not officially affiliated with the Omarchy Foundation nor any of the brands mentioned in this README or source code. This product is developed in open-source and is provided for free by volunteer contributors. If a brand has an issue with their product(s) being supported, please reach out in an GitHub Issue and it can be taken care of.

## License
OmniGear is licensed under the [*GNU General Public License v3.0*](https://choosealicense.com/licenses/lgpl-3.0/). Permissions of this strong copyleft license are conditioned on making available complete source code of licensed works and modifications, which include larger works using a licensed work, under the same license. Copyright and license notices must be preserved. Contributors provide an express grant of patent rights.

For the betterment of the community, we would prefer you contribute directly to this project, but you are allowed to use your own fork as long as it is public like this project is.

<p align="center">___</p>
<p align="center">
    <img src="https://avatars.githubusercontent.com/u/148131180?s=200&v=4" height="100" style="border-radius: 16px"><br>
    <picture>
        <source media="(prefers-color-scheme: dark)" srcset="https://arbitrari.dev/assets/arbitrari-white.png">
        <source media="(prefers-color-scheme: light)" srcset="https://arbitrari.dev/assets/arbitrari-black.png">
        <img src="https://arbitrari.dev/assets/arbitrari-black.png" height="16px" style="margin-top: 16px">
    </picture>
</p>