<picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/omnigear-logo-white.png">
    <source media="(prefers-color-scheme: light)" srcset="assets/omnigear-logo-black.png">
    <img src="assets/omnigear-logo-black.png" alt="OmniGear" width="350" style="margin-left: -16px">
</picture>

___

An [Omarchy](https://omarchy.org) plugin that monitors and configures your mice, keyboards, and headsets, whatever the brand. One plugin replaces a plugin per vendor.

## Install

```sh
omarchy plugin add https://omni.arbitrari.dev --enable
```

Then let OmniGear open your devices. Linux makes HID devices root-only by default, and without this rule OmniGear cannot see them:

```sh
sudo cp ~/.config/omarchy/plugins/io.github.arbitrari.omnigear/udev/60-omnigear.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules && sudo udevadm trigger
```

> [!IMPORTANT]
> Disconnect each device and connect them back after you add the rule. A device keeps the permissions it had when it was connected.

## Update

Update from the plugin, or run:

```sh
omarchy plugin update io.github.arbitrari.omnigear
```

## Supported devices

> [!NOTE]
> 🟩 Supported · 🟨 Partly supported · 🟥 Planned · ⬛ The device does not have this feature

### Mice

<details>
<summary><b>Logitech</b></summary>

|  | Name | Battery | DPI | Polling | HITS | SmartShift | Hi-Res Wheel | Thumbwheel | Buttons | Easy-Switch | Notes |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 🟩 | PRO X2 SUPERSTRIKE  |🟩|🟩|🟩|🟩|⬛|⬛|⬛|⬛|⬛| Settings are read-only while the mouse uses onboard memory |
| 🟥 | PRO X2 SUPERLIGHT 2 | | | |⬛| | | | | | |
| 🟥 | PRO X2 SUPERLIGHT   | | | |⬛| | | | | | |
| 🟥 | PRO 2 LIGHTSPEED    | | | |⬛| | | | | | |
| 🟥 | G502 X / PLUS       | | | |⬛| | | | | | |
| 🟥 | G309 LIGHTSPEED     | | | |⬛| | | | | | |
| 🟥 | G305                | | | |⬛| | | | | | |
| 🟥 | MX Master 4         | | | |⬛| | | | | | |
| 🟩 | MX Master 3S        |🟩|🟩|⬛|⬛|🟩|🟩|🟩|🟩|🟩| |
| 🟩 | MX Master 3         |🟨|🟩|⬛|⬛|🟩|🟩|🟩|🟩|🟨| Battery shows four levels, not a percentage. Hosts show no names or pairing status |
| 🟥 | MX Master 2         | | | |⬛| | | | | | |
| 🟥 | MX Master           | | | |⬛| | | | | | |

- **HITS** (Haptic Inductive Trigger System). Set the actuation point and haptic feedback for left and right click. Shown in the **Triggers** tab.
- **SmartShift.** The scroll wheel switches between ratcheted and free spin by itself. Shown in the **Wheel** tab.
- **Buttons.** Reassign a button in the mouse itself. Left and right click cannot be reassigned, so the mouse can always click. Shown in the **Buttons** tab.
- **Easy-Switch.** See the paired hosts and switch between them. Shown in the **Hosts** tab.

</details>

<details>
<summary><b>Razer</b></summary>

|  | Name | Battery | DPI | Polling | Notes |
|---|---|---|---|---|---|
| 🟥 | DeathAdder V4 Pro  | | | | |
| 🟥 | DeathAdder V3 Pro  | | | | |
| 🟥 | DeathAdder V3 HS   | | | | |
| 🟥 | DeathAdder V3      | | | | |
| 🟥 | DeathAdder V2 X HS | | | | |
| 🟥 | Viper V4 Pro       | | | | |
| 🟥 | Viper V3 Pro SE    | | | | |
| 🟥 | Viper V3 Pro       | | | | |
| 🟥 | Naga V3 Pro        | | | | |
| 🟥 | Naga V2 Pro        | | | | |
| 🟥 | Naga V2 HS         | | | | |
| 🟥 | Basilisk V3 Pro    | | | | |

</details>

Planned brands with no models listed yet: Corsair, Glorious, and FinalMouse.

### Keyboards

No keyboards are supported yet. Planned brands: Logitech, Razer, Wooting, Keychron, SteelSeries, ASUS ROG, Pulsar, Glorious, Lofree, HyperX, and 8BitDo.

### Headsets and headphones

<details>
<summary><b>SteelSeries</b></summary>

|  | Name | Battery | Noise Control | Equalizer | Gain | Sidetone | Mic Volume | Mute Light | Wireless Mode | Auto Power Off | Sonar | Mic Mute | Notes |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 🟩 | Arctis Nova Pro Wireless |🟩|🟩|🟩|🟩|🟩|🟩|🟩|🟩|🟩|🟩|🟩| Needs the base station |

- **Battery.** The charge of the headset and of the spare battery in the base station, in eighths.
- **Noise Control.** Noise cancelling, transparency, or off, with the transparency level. Shown in the **Sound** tab.
- **Equalizer.** A 10-band equalizer for each Sonar channel, with Flat, Bass Boost, Treble Boost, Vocal, and Custom presets. Shown in the **Equalizer** tab.
- **Wireless Mode.** Favor low latency (speed) or distance (range). The headset drops for a few seconds while it reconnects. Shown in the **Power** tab.
- **Sonar.** Separate Game, Chat, Media, and Aux outputs, like SteelSeries Sonar. The base station's ChatMix dial balances Game against Chat. Set each channel's volume, and move each playing app to a channel. An app keeps its channel the next time it plays. Turning Sonar on or off restarts PipeWire, so audio drops for a moment. Shown in the **Sonar** tab.
- **Screen.** Turn the base station's screen off, or have it turn off after 1 minute to 1 hour untouched. Off is dark, or a screensaver: a DVD-style bouncing logo, the Omarchy logo built by random effects like Omarchy's own screensaver, or the OmniGear wordmark. Turning the knob or the dial brings it back. Shown in the **Screen** tab.

</details>

<details>
<summary><b>Sony</b></summary>

|  | Name | Battery | Noise Control | Equalizer | Codec | DSEE | Speak-To-Chat | Auto Power Off | Touch Panel | Notes |
|---|---|---|---|---|---|---|---|---|---|---|
| 🟥 | WH-1000XM6 | | | | | | | | | |
| 🟥 | WH-1000XM5 | | | | | | | | | |
| 🟩 | WH-1000XM4 |🟩|🟩|🟩|🟩|🟩|🟩|🟩|🟩| Bluetooth only |
| 🟩 | WH-1000XM3 |🟩|🟩|🟩|🟩|🟩|⬛|🟩|🟩| Bluetooth only. Equalizer works over SBC and AAC only |

- **Noise Control.** Noise cancelling, ambient sound, or off, with the ambient level and focus on voice. Shown in the **Sound** tab.
- **Codec.** The Bluetooth codec the audio uses, such as SBC, AAC, aptX, or LDAC. Your computer's sound server picks it, not the headset. Shown in the **Sound** tab.
- **DSEE.** Sony's upscaling of compressed audio. Shown in the **Sound** tab.

</details>

Planned brands with no models listed yet: Logitech, Razer, HyperX, Turtle Beach, Sennheiser, Audio-Technica, Apple, Google, Nothing, and OnePlus.

## Add your device

If you own a device OmniGear does not support, you can add it. You need the device connected to an Omarchy machine. The maintainers probably do not own it, so your testing is what makes the PR mergeable.

1. Clone the repository and build it:

   ```sh
   git clone https://github.com/arbitrari/omarchy-omnigear
   cd omarchy-omnigear
   ./scripts/dev-install
   ```

   The script installs Go through mise on first run, builds the plugin, installs it into your shell, and restarts the shell.

2. Follow [Add support for a device you own](docs/ADDING-A-DEVICE.md). It covers finding the ids, writing the code, testing, and opening the PR.

### Use a coding agent

Claude Code, Codex, Cursor, opencode, and other agents read [`AGENTS.md`](AGENTS.md) on their own. Start one in the repository with the device connected, and give it this prompt:

```text
Add OmniGear support for my <brand and model>, which is connected to this
machine now. Read AGENTS.md, then follow docs/ADDING-A-DEVICE.md one step at a
time. Use only ids and behavior you observe on this device. Ask me before you
run anything with sudo or restart my shell, and ask me to confirm anything you
cannot observe yourself, such as lights or sounds. When every step passes, open
the pull request with the template from step 8.
```

Read the diff before the agent opens the PR. Keep the PR to one device.

## Report a bug

Open an issue with the output of these commands, run with the device connected:

```sh
cd ~/.config/omarchy/plugins/io.github.arbitrari.omnigear
./bin/omnigear report
./bin/omnigear list
```

The output contains serial numbers. Remove them if you do not want them public.

## Disclaimer

OmniGear is not affiliated with the Omarchy Foundation or with any brand named in this README or the source code. Volunteers build it in the open and give it away. If you represent a brand and object to a product's support, open an issue.

## License

OmniGear is licensed under the [GNU General Public License v3.0](LICENSE.md). If you distribute a modified version, you must publish its full source under the same license and keep the copyright and license notices. We would rather you contribute here, but a public fork is allowed.

<p align="center">___</p>
<p align="center">
    <img src="https://avatars.githubusercontent.com/u/148131180?s=200&v=4" height="100" style="border-radius: 16px"><br>
    <picture>
        <source media="(prefers-color-scheme: dark)" srcset="https://arbitrari.dev/assets/arbitrari-white.png">
        <source media="(prefers-color-scheme: light)" srcset="https://arbitrari.dev/assets/arbitrari-black.png">
        <img src="https://arbitrari.dev/assets/arbitrari-black.png" height="16px" style="margin-top: 16px">
    </picture>
</p>
