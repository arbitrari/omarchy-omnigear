# Architecture

OmniGear is two pieces with one seam between them:

```
  qml/          the Omarchy bar widget — draws things, talks to no hardware
    │
    │  JSON over stdout, one object per invocation
    ▼
  omnigear      a Go CLI — owns every byte that reaches a device
```

The seam is deliberate. QML runs inside the long-lived `omarchy-shell` process,
where a blocking read on a wedged USB device would freeze the desktop. The CLI
is a short-lived process with bounded timeouts; the worst a broken device can do
is make one poll come back empty.

## Finding a device in the source

The tree mirrors the README's support table — category, then brand, then model:

```
internal/devices/
  mice/
    logitech/
      logitech.go                 every Logitech mouse, as catalog entries
      prox2superstrike.go         what is specific to this one model
    razer/
    corsair/  glorious/  finalmouse/
  keyboards/
    logitech/  razer/  keychron/
  headsets/
    steelseries/  logitech/  razer/  hyperx/
    sony/  apple/  google/  nothing/  oneplus/
```

A device's runtime id is that same path, so a bug report names its own source
directory:

```
mouse/logitech/pro-x2-superstrike#5f-ba-c9-65
└─┬─┘ └───┬──┘ └────────┬───────┘ └────┬────┘
category  brand       model        serial
```

## The three layers under `internal/`

| Layer         | Knows about                              | Reused by                       |
|---------------|------------------------------------------|---------------------------------|
| `transport/`  | wire formats — hidraw nodes, HID++ 2.0   | every brand that speaks it      |
| `drivers/`    | how to read/write a family of devices    | every model in that family      |
| `devices/`    | which models exist and what they can do  | —                               |

The split is what keeps the catalog cheap. `transport/hidpp` is the HID++
protocol and nothing else, so a Logitech *keyboard* will use it unchanged.
`drivers/logitech` turns that protocol into battery/DPI/polling-rate
reads, so most new Logitech mice are a catalog entry and no new code.

## Capabilities are the contract

A catalog entry declares what a model can do:

```go
Capabilities: []model.Capability{model.CapBattery, model.CapDPI, model.CapPollingRate},
```

Everything downstream keys off that list. The driver only probes what is
claimed. The CLI only emits state for what was probed. The UI renders one
control per capability. **Adding a device does not mean writing UI** — that is
the whole point of the arrangement.

## Adding a model

For a model whose family already has a driver:

1. Add a `model.Entry` to `internal/devices/<category>/<brand>/<brand>.go`.
2. Give it the USB ids you have *observed* — run `omnigear probe` with the
   device plugged in. Never guess an id; a wrong one binds a driver to
   somebody else's hardware.
3. Set `Support` honestly: `SupportPartial` until every capability it claims
   works.

For a model that needs new behaviour, add a file beside it
(`prox2superstrike.go`) and, if a whole new protocol is involved, a
`transport/` package and a `drivers/` package.

## Adding a brand

A brand that does not speak an existing protocol needs a transport. The shape to
follow is `transport/hidraw` + `transport/hidpp`: enumeration and framing in
the transport, meaning in the driver.

## Working out a wire format

`omnigear probe` lists every hidraw node, whether the catalog claims it, and
which HID++ features the device actually implements. `omnigear call` then sends
one raw request:

```bash
omnigear probe
omnigear call pro-x2-superstrike 0x2202 5 00 00
# → { "featureIndex": 9, "hex": "00 0F A0 0F A0 0F A0 0F A0 02 …" }
```

Every decoder in `drivers/logitech` carries the bytes it was read off
in a doc comment. Keep that up: these formats are not documented anywhere
public, and the bytes are the only proof.

Two things learned the hard way on the PRO X2 SUPERSTRIKE, both noted in the
source:

- `setSensorDpi` rejects a lift-off distance of `0`. The write has to carry the
  device's current LOD, not a placeholder.
- The polling rates on offer depend on the *link*: 125–1000 Hz on Lightspeed,
  125–8000 Hz on a cable. `hidraw.Node.Link` reads that off the sysfs parent.

## You are not the only one talking to the device

A hidraw node delivers every reply to **every** open reader. A second HID++
client on the same mouse — Solaar, libratbag/Piper, a vendor plugin — is not
partitioned from OmniGear: its replies arrive interleaved with ours, and its
writes can revert ours moments after they land.

The signature is a device that reads perfectly but will not keep a setting.
It is unguessable from the outside, so both places that can see it say so:

```bash
omnigear probe    # every node lists `openedBy`
# → { "path": "/dev/hidraw8", "openedBy": [{"pid": 1234, "name": "solaar"}] }
```

and a write that does not stick names the other client in its error rather
than just reporting the stale value:

```
device accepted the change but kept 4000 — solaar (pid 1234) also has
/dev/hidraw8 open, which can revert writes
```

Only same-user processes are visible through `/proc`, so an empty list means
"nothing found", not "nothing there".

## Writes are verified, never assumed

A device can accept a write and quietly ignore it. `omnigear set` therefore
reads the device back and compares:

| Outcome                        | Result                                      |
|--------------------------------|---------------------------------------------|
| value matches what was asked   | `ok`                                        |
| value changed, but rounded     | `ok`, with `applied` and a `note`           |
| value did not change           | **error** — "accepted the change but kept …"|

The UI shows what the hardware says, not what it was told.

## JSON contract

Every command prints exactly one JSON object. Failures are objects too, so the
QML side never has to parse stderr:

```jsonc
{ "ok": true, "schema": 1, "devices": [ … ] }
{ "ok": false, "error": "no connected device matches 'foo'" }
```

`schema` is bumped when the shape changes in a way that would break a reader.

## Why Go

The first cut of this was Rust, ported early. Nothing here needs what Rust
buys: a 20-second poll, a few hundred bytes of I/O, no concurrency. What the
project does need is to be readable by whoever maintains it and cheap to
contribute a device to — and the catalog is data, which reads better as plain
Go structs than as `&'static [Capability]`. Cross-compiling the shipped binary
for another architecture is also one environment variable rather than a
toolchain.

The one thing lost is exhaustive matching: adding a `Capability` no longer
makes the compiler point at every place that must handle it. If the capability
list grows much past a handful, that gap is worth covering with a test that
walks the catalog.
