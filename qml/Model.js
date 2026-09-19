// Model.js — turning `omnigear` JSON into things the UI can draw.
//
// Pure functions only: no QML types, no side effects. Everything the CLI says
// is treated as untrusted text until it has been through here.

.pragma library

var UNKNOWN = -1

/// Cap on a single CLI reply, so a runaway process cannot grow the shell.
var MAX_REPLY = 65536

function emptyState() {
  return { ok: false, devices: [], error: "", note: "" }
}

/// Strip control characters and anything that could be read as markup, then
/// bound the length. Device names come off the wire and land in tooltips.
function safeText(value, fallback, maxLength) {
  if (value === undefined || value === null) return fallback
  return String(value)
    .replace(/[<>&\u0000-\u001f\u007f]/g, "")
    .slice(0, maxLength) || fallback
}

function parse(raw) {
  var state = emptyState()
  if (typeof raw !== "string" || raw === "" || raw.length > MAX_REPLY) {
    state.error = "No reply from omnigear"
    return state
  }

  var data
  try {
    data = JSON.parse(raw)
  } catch (e) {
    state.error = "Unreadable reply from omnigear"
    return state
  }

  if (!data || data.ok !== true) {
    state.error = safeText(data && data.error, "omnigear reported a failure", 256)
    return state
  }

  // `list` replies with an array; `set` replies with the one device it just
  // wrote and read back. Both are devices, so both parse the same way.
  var devices = Array.isArray(data.devices) ? data.devices
              : (data.device ? [data.device] : [])
  state.devices = devices.map(parseDevice)
  state.note = safeText(data.note, "", 128)
  state.ok = true
  return state
}

function parseDevice(raw) {
  var d = raw || {}
  var s = d.state || {}
  return {
    id: safeText(d.id, "", 128),
    name: safeText(d.name, "Unknown device", 64),
    brand: safeText(d.brandLabel, "", 32),
    category: safeText(d.category, "", 16),
    support: safeText(d.support, "planned", 16),
    connection: parseConnection(d.connection),
    onboardProfile: safeText(s.onboardProfile, "", 16),
    capabilities: Array.isArray(d.capabilities) ? d.capabilities : [],
    battery: parseBattery(s.battery),
    dpi: s.dpi ? {
      current: Number(s.dpi.current) || 0,
      min: Number(s.dpi.min) || 0,
      max: Number(s.dpi.max) || 0,
      step: Number(s.dpi.step) || 0,
      presets: Array.isArray(s.dpi.presets) ? s.dpi.presets : []
    } : null,
    pollingRate: s.pollingRate ? {
      current: Number(s.pollingRate.current) || 0,
      supported: Array.isArray(s.pollingRate.supported) ? s.pollingRate.supported : []
    } : null,
    errors: Array.isArray(s.errors) ? s.errors.map(function (e) {
      return safeText(e, "", 256)
    }) : []
  }
}

function parseConnection(raw) {
  if (!raw) return { kind: "", label: "" }
  return {
    kind: safeText(raw.kind, "", 24),
    label: safeText(raw.label, "", 48)
  }
}

function parseBattery(raw) {
  if (!raw) return null
  var percent = typeof raw.percent === "number" ? raw.percent : UNKNOWN
  return {
    percent: percent,
    level: safeText(raw.level, "unknown", 16),
    status: safeText(raw.status, "unknown", 16)
  }
}

function has(device, capability) {
  return !!device && device.capabilities.indexOf(capability) !== -1
}

// --- presentation ----------------------------------------------------------

/// Nerd Font glyphs, matching the icons Omarchy's own plugins use.
function categoryIcon(category) {
  switch (category) {
  case "mouse": return "󰍽"
  case "keyboard": return "󰌌"
  case "headset": return "󰋋"
  default: return "󰄜"
  }
}

function batteryIcon(percent, status) {
  var charging = ["󰢜", "󰂆", "󰂇", "󰂈", "󰢝", "󰂉", "󰢞", "󰂊", "󰂋", "󰂅"]
  var idle = ["󰁺", "󰁻", "󰁼", "󰁽", "󰁾", "󰁿", "󰂀", "󰂁", "󰂂", "󰁹"]

  if (percent === UNKNOWN || percent < 0) return "󰁽"
  var slot = Math.max(0, Math.min(9, Math.floor(percent / 10)))
  if (status === "charging") return charging[slot]
  if (status === "full") return "󰂅"
  return idle[slot]
}

function batteryText(percent) {
  return (percent === UNKNOWN || percent < 0) ? "--" : String(percent) + "%"
}

/// The device the bar should speak for: the one with the lowest battery, so the
/// thing about to die is the thing you see.
function primary(devices) {
  if (!devices || devices.length === 0) return null
  var withBattery = devices.filter(function (d) {
    return d.battery && d.battery.percent !== UNKNOWN
  })
  if (withBattery.length === 0) return devices[0]
  return withBattery.reduce(function (lowest, d) {
    return d.battery.percent < lowest.battery.percent ? d : lowest
  })
}

function tooltip(state) {
  if (!state.ok) return "OmniGear: " + (state.error || "unavailable")
  if (state.devices.length === 0) return "OmniGear: no supported devices"

  return state.devices.map(function (d) {
    var parts = [d.name]
    if (d.battery && d.battery.percent !== UNKNOWN) {
      parts.push(batteryText(d.battery.percent))
    }
    if (d.dpi && d.dpi.current > 0) parts.push(d.dpi.current + " DPI")
    if (d.pollingRate && d.pollingRate.current > 0) parts.push(d.pollingRate.current + " Hz")
    return parts.join(" · ")
  }).join("\n")
}

// --- grouping --------------------------------------------------------------

/// Categories in the order the README lists them, so the panel reads the same
/// way the support table does.
var CATEGORY_ORDER = ["mouse", "keyboard", "headset"]

function categoryLabel(category) {
  switch (category) {
  case "mouse": return "Mice"
  case "keyboard": return "Keyboards"
  case "headset": return "Headsets"
  default: return "Other"
  }
}

/// Group devices into [{ category, label, devices }], skipping empty
/// categories. One section per "###" heading in the README.
function groups(devices) {
  var out = []
  for (var i = 0; i < CATEGORY_ORDER.length; i++) {
    var category = CATEGORY_ORDER[i]
    var members = (devices || []).filter(function (d) {
      return d.category === category
    })
    if (members.length > 0) {
      out.push({ category: category, label: categoryLabel(category), devices: members })
    }
  }

  // Anything with an unrecognised category still gets shown rather than
  // silently dropped — a device the UI cannot name is still a device.
  var known = CATEGORY_ORDER.join(",")
  var rest = (devices || []).filter(function (d) {
    return known.indexOf(d.category) === -1
  })
  if (rest.length > 0) {
    out.push({ category: "other", label: categoryLabel("other"), devices: rest })
  }
  return out
}

function capabilityLabel(capability) {
  switch (capability) {
  case "battery": return "Battery"
  case "dpi": return "DPI"
  case "polling-rate": return "Polling rate"
  case "hits": return "Haptic triggers"
  case "lod": return "Lift-off distance"
  case "onboard-profile": return "Profile storage"
  default: return capability
  }
}

/// Capabilities the device claims but this build cannot show a control for.
/// Listing them is deliberate: "declared, not yet driven" is information, and
/// hiding it would make the mouse look less capable than it is.
function unsupportedCapabilities(device) {
  if (!device) return []
  var handled = ["battery", "dpi", "polling-rate", "onboard-profile"]
  return device.capabilities.filter(function (c) {
    return handled.indexOf(c) === -1
  })
}

/// Which side owns the device's settings. "Onboard" means the device runs its
/// own stored profile and refuses software writes.
function profileModeLabel(mode) {
  switch (mode) {
  case "onboard": return "Onboard"
  case "host": return "Software"
  case "": return ""
  default: return titleCase(mode)
  }
}

function batteryStatusLabel(battery) {
  if (!battery) return ""
  switch (battery.status) {
  case "charging": return "Charging"
  case "full": return "Full"
  case "discharging": return battery.level ? titleCase(battery.level) : "On battery"
  default: return ""
  }
}

function titleCase(text) {
  if (!text) return ""
  return text.charAt(0).toUpperCase() + text.slice(1)
}

/// The line under a device's name: who made it, how it is attached, and how
/// complete support for it is. Empty parts are dropped rather than leaving
/// stray separators.
function deviceSubtitle(device) {
  if (!device) return ""
  var parts = [device.brand]
  if (device.connection && device.connection.label) parts.push(device.connection.label)
  var support = supportLabel(device.support)
  if (support) parts.push(support)
  return parts.filter(function (p) { return !!p }).join(" · ")
}

function supportLabel(support) {
  switch (support) {
  case "full": return ""
  case "partial": return "Partial support"
  case "planned": return "Not supported yet"
  default: return ""
  }
}

/// A slider needs a step it can actually land on. Devices report the finest
/// step of a range that gets coarser as it climbs, which would make a
/// 100–44000 slider crawl, so widen it enough to cross the range in a sane
/// number of moves while still hitting round numbers.
function sliderStep(dpi) {
  if (!dpi) return 1
  var span = Math.max(1, dpi.max - dpi.min)
  var target = span / 200
  var steps = [1, 5, 10, 25, 50, 100, 200, 500, 1000]
  for (var i = 0; i < steps.length; i++) {
    if (steps[i] >= target) return Math.max(steps[i], dpi.step || 1)
  }
  return steps[steps.length - 1]
}
