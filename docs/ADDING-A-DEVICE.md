# Add support for a device you own

This guide takes a device that is plugged into your machine and turns it into a
pull request. It is written for a coding agent working beside you, and it works
the same way when you do it by hand. The device has to be connected the whole
time. Every claim the PR makes is checked against that device.

## Rules that are never broken

- **Use only ids you observed.** Take USB and Bluetooth ids from `probe`,
  `report`, `lsusb`, or `bluetoothctl info` on this machine. Never copy them
  from the internet. A wrong id binds a driver to somebody else's hardware.
- **Claim only what you saw work.** A capability goes in the catalog entry only
  after a read and a write of it succeeded on the device. Use
  `model.SupportPartial` until every claimed capability works.
- **Never write to an unknown function.** On HID++, a function index does not
  say whether it reads or writes. Feature `0x1815` fn 4 overwrites a host name
  when called with no arguments. Read a feature's whole function list from a
  public source before you call into it. Never touch firmware update features
  (`0x00C2`, `0x00C3`, or any DFU interface).
- **Put every setting back.** Record each value before you change it, and
  restore it when you are done.
- **Ask the person for what you cannot see.** A light, a sound, a click feel,
  and the panel's appearance need a human. Ask a specific question, such as
  "Did the mute light turn off?".
- **Stop for `sudo`.** Installing the udev rule needs root. Give the person the
  command and wait.

## 1. Build the CLI

```sh
mise exec -- go build -o bin/omnigear ./cmd/omnigear
```

On Omarchy this needs nothing else. The first run installs Go through mise.
Rebuild after every Go change. `./bin/omnigear` prints JSON for every command.

## 2. Identify the device

```sh
./bin/omnigear list      # catalogued and discovered devices, with state
./bin/omnigear probe     # every hidraw node, its ids, and which HID++ features answered
./bin/omnigear report    # an issue-ready write-up of every node on the machine
lsusb                    # USB devices, as vendor:product
bluetoothctl devices     # paired Bluetooth devices; then `bluetoothctl info <MAC>`
```

Write down the vendor and product id, the connection type, and the kernel
driver. A device can show different ids on each connection. A mouse can have
one id on its dongle, one over a cable, and one over Bluetooth. Plug it in every
way it supports and record each id.

If `probe` shows a node with `EACCES`, the udev rule does not cover the vendor
yet. See step 5.

## 3. Pick the path

Find the row that matches what step 2 showed.

| What you see | Path | Typical size |
|---|---|---|
| `list` shows the device with `"support": "unsupported"`, or `probe` shows HID++ features answering | A. Catalog entry | One entry |
| A Sony headphone that publishes the control service `96cc203e…` | A. Catalog entry | One entry |
| A SteelSeries Arctis base station | A or C | Read `drivers/steelseries` first |
| The family is driven, but the device has a setting no capability covers | B. New capability | Model, driver, CLI, and UI |
| No driver here speaks its protocol | C. New protocol | Transport, driver, udev, and catalog |

Path C is a large PR. Say so in the PR description, so the maintainers can
review it in pieces.

## 4. Implement

### Path A: add a catalog entry

1. Open `internal/devices/<category>/<brand>/<brand>.go`. Copy the closest
   existing entry. `internal/devices/mice/logitech/logitech.go` has examples.
2. Put each observed id in a named constant, with a comment that says where it
   was observed. See `internal/devices/headsets/sony/sony.go`.
3. If a `planned(...)` entry for the model exists, replace it. Keep the `Slug`.
4. For a device behind a receiver the kernel does not expand, add `Names`. Use
   the name the device reports over HID++.
5. List only the capabilities you verify in step 6.
6. If the model differs from the rest of its family, write the differences in a
   file beside the entry. `mxmaster3.go` is the pattern.

A brand package with no entries yet is already registered in
`internal/devices/<category>/<category>.go`. A new brand needs a `model.Brand`
constant and a `brandLabels` entry in `internal/model/model.go`, a package,
and a line in that registry.

### Path B: add a capability

A capability reaches every layer. Search for an existing one of the same shape,
such as `CapSidetone`, and follow it through each file:

1. `internal/model/model.go`. Add the `Capability` constant, the state field,
   the `SettingKey`, and the parsing in `ParseSetting`.
2. The driver under `internal/drivers/<brand>/`. Add the read to `Read` and the
   write to `Write`. Put each new error in `DeviceState.Errors`. Do not fail
   the whole read.
3. `cmd/omnigear/main.go`. Add the key to the `usage` text.
4. `qml/Model.js`. Add the capability to `TAB_GROUPS`, `capabilityLabel`, and
   the `handled` list in `unsupportedCapabilities`.
5. `qml/ui/DeviceCard.qml`. Add a control. A setting with a few named values
   uses `ChoiceControl`. A number on a scale uses `LevelControl`. Write a new
   control only when neither fits.

### Path C: add a protocol

1. Find what is already known. Read the source of an open project that drives
   the device. Name it in a doc comment on the transport. Do not copy code
   from a project whose license does not allow GPL-3.0 use.
2. Write the wire format in `internal/transport/<protocol>/`: framing,
   enumeration, and timeouts only. Model it on `transport/arctis` for a vendor
   HID protocol, or `transport/mdr` for Bluetooth RFCOMM.
3. Write the meaning in `internal/drivers/<brand>/`: a `model.Driver`. If the
   device owns several hidraw nodes, implement `model.Speaker` so discovery
   picks the right one from its report descriptor. If its battery can be read
   without waking it, implement `model.BatteryReader`.
4. Give every decoder a doc comment that carries the bytes it was read from.
   These formats are not documented in public, and the bytes are the only
   proof.
5. Do throwaway protocol experiments in a scratch program outside the repo.
   Commit only what the driver uses. `./bin/omnigear call` speaks only HID++.

`docs/ARCHITECTURE.md` explains the timing, sleep, and node-selection traps
that apply to every protocol. Read the sections that match the device before
you write a driver.

## 5. Grant access with udev

Skip this step if the vendor already has a line in `udev/60-omnigear.rules`.

Add one rule for the vendor, with a comment that names the brand. Scope it by
vendor id. Never match all of `SUBSYSTEM=="hidraw"`. A Bluetooth HID device has
no USB parent, so copy the `KERNELS=="0005:*046D:*"` rule and change the
vendor. Headphones driven over RFCOMM need no rule. Then ask the person to run:

```sh
sudo cp udev/60-omnigear.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules && sudo udevadm trigger
```

Ask them to unplug the device and plug it back in. A node keeps the permissions
it had when it was added.

## 6. Verify on the device

Do this for each capability you claim:

```sh
./bin/omnigear get <slug>                 # note the current value
./bin/omnigear set <slug> <key> <value>   # change it; expect "ok": true
./bin/omnigear get <slug>                 # confirm the new value
./bin/omnigear set <slug> <key> <old>     # put it back
```

`set` reads the device back after every write. An error that says "accepted the
change but kept …" means the device ignored the write. Treat it as a failure.

Then check the panel:

```sh
./scripts/dev-install
```

The script installs the plugin into the running shell and restarts the shell.
Tell the person before you run it. Ask them to open the OmniGear panel, check
that each control appears and works, and check that the bar shows the battery.

Finally, check the code:

```sh
gofmt -l .                       # must print nothing
mise exec -- go vet ./...
```

## 7. Update the docs

- `README.md`. Add or update the device's row in its brand's table. Fill each
  column with 🟩, 🟨, 🟥, or ⬛. Put any limits in **Notes**. A new capability
  gets a column and a one-line footnote.
- `docs/ARCHITECTURE.md`. A new protocol gets a section. A hardware quirk you
  found gets a line in the matching section, with the bytes that showed it.

## 8. Open the pull request

Keep the PR to one device. Branch from `master` and commit:

```sh
git fetch origin
git switch -c add-<slug> origin/master
git add -A && git commit -m "<Brand>: support the <Model>"
```

Then push to your fork and open the PR against `arbitrari/omarchy-omnigear`:

```sh
gh repo fork --remote --remote-name fork
git push -u fork add-<slug>
gh pr create --repo arbitrari/omarchy-omnigear --base master \
  --head "$(gh api user -q .login):add-<slug>" --title "<Brand>: support the <Model>" --body-file pr.md
```

`gh repo fork` makes the fork once and adds it as the `fork` remote. Write the
body to `pr.md` from the template below, and delete the file after the PR is
open.

Edit the body to follow this template. The maintainers probably do not own the
device, so the evidence is what they review.

```markdown
## Device
<Model>, <vendor:product> over <connection>. Path <A|B|C>.

## What works
| Capability | Read | Write | Restored |
|---|---|---|---|
| dpi | 🟩 | 🟩 | 🟩 |

## Evidence
<details><summary>probe</summary>

(paste `./bin/omnigear probe`, with serials removed)
</details>
<details><summary>get, set, get</summary>

(paste the commands and output from step 6)
</details>

## Checked by a person
- [ ] The panel shows every control, and each one works
- [ ] <anything physical: lights, sounds, feel>

## Not done
<capabilities the device has that this PR leaves out, and why>
```

Remove serial numbers from everything you paste. They appear in `probe`,
`list`, and device ids after the `#`.
