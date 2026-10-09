# OmniGear for agents

OmniGear is an Omarchy bar widget. It has two halves:

- `qml/` draws the bar and the panel. It never talks to hardware.
- `cmd/omnigear/` is a Go CLI that the QML runs on every poll. It owns every
  byte sent to a device and prints one JSON object per command.

`internal/` holds the Go layers. `transport/` is wire formats, `drivers/` turns
them into settings, and `devices/` is the catalog of models.
`docs/ARCHITECTURE.md` explains why each part is built the way it is. Read the
section for the subsystem you change before you change it.

## Commands

```sh
mise exec -- go build -o bin/omnigear ./cmd/omnigear   # build the CLI
mise exec -- go vet ./...
gofmt -l .                                             # must print nothing
./bin/omnigear list | probe | report | get <device>
./scripts/dev-install                                  # install into the shell; restarts it
```

There is no test suite. Verify against the real device with `get` and `set`.

## Adding hardware

Follow `docs/ADDING-A-DEVICE.md` step by step. It covers ids, the three
implementation paths, udev, verification, and the PR template.

## Conventions

- A capability in a catalog entry is a promise that the driver reads and writes
  it on that model. The UI draws one control per capability.
- `set` reads the device back after every write. Keep that true for new
  settings.
- A failed read of one capability goes in `DeviceState.Errors`. It never fails
  the whole read.
- Decoders carry the bytes they were read from in a doc comment.
- Comments explain a reason the code cannot show. Match the comment style of the
  file you edit.
- Section titles and control-group headings in the panel are ALL CAPS. Field
  labels inside a group are Title Case. Keep label strings in Title Case and
  pass `group: true` to `qml/ui/SettingHeader.qml`, which applies the caps.
- Remove serial numbers from anything you paste into an issue or PR.
