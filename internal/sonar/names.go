package sonar

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// An application names its own sound stream, and the name it gives is often
// its audio library's rather than its own: Discord's voice plays as "WEBRTC
// VoiceEngine", and Valheim's as "FMOD Ex App". Nobody would recognise
// either in a list. The stream also says which process is playing it, and
// that leads somewhere better:
//
//   - A game Steam launched carries its app id in its environment
//     (SteamAppId=892970), and Steam's own manifest for it has the store
//     name, "Valheim".
//   - Otherwise, the process's binary usually matches an installed
//     application's desktop entry, whose Name is what the launcher shows:
//     the binary "Discord" is the entry named "Discord".
//
// Failing both, the stream's own name is used. Only the label changes; an
// app is still routed by the name it gave, because that is what WirePlumber
// remembers its output by.

// labeller finds friendly names, reading the desktop entries once however
// many apps are asked about.
type labeller struct {
	desktop map[string]string
	loaded  bool
}

func (l *labeller) label(app string, pid int, binary string) string {
	if name := steamName(pid); name != "" {
		return name
	}
	if name := l.desktopName(binary); name != "" {
		return name
	}
	return app
}

// steamName is the store name of the Steam game a process belongs to, or
// empty for a process Steam did not launch.
func steamName(pid int) string {
	if pid <= 0 {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "environ"))
	if err != nil {
		return ""
	}
	var appID string
	for _, entry := range bytes.Split(raw, []byte{0}) {
		key, value, found := strings.Cut(string(entry), "=")
		if found && (key == "SteamAppId" || (key == "SteamGameId" && appID == "")) && isDigits(value) && value != "0" {
			appID = value
		}
	}
	if appID == "" {
		return ""
	}
	for _, library := range steamLibraries() {
		if name := acfValue(filepath.Join(library, "steamapps", "appmanifest_"+appID+".acf"), "name"); name != "" {
			return name
		}
	}
	return ""
}

// steamLibraries are the folders Steam installs games into: its own, and any
// others libraryfolders.vdf lists.
func steamLibraries() []string {
	home, _ := os.UserHomeDir()
	roots := []string{
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".steam", "steam"),
	}
	seen := map[string]bool{}
	var libraries []string
	add := func(path string) {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
		if path != "" && !seen[path] {
			seen[path] = true
			libraries = append(libraries, path)
		}
	}
	for _, root := range roots {
		add(root)
		for _, path := range acfValues(filepath.Join(root, "steamapps", "libraryfolders.vdf"), "path") {
			add(path)
		}
	}
	return libraries
}

// acfValues reads every value of a key from Steam's text format, one
// `"key"		"value"` pair to a line. Nothing here needs the nesting.
func acfValues(path, key string) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	var values []string
	want := `"` + key + `"`
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		rest, found := strings.CutPrefix(line, want)
		if !found {
			continue
		}
		rest = strings.TrimSpace(rest)
		if len(rest) >= 2 && rest[0] == '"' && rest[len(rest)-1] == '"' {
			values = append(values, rest[1:len(rest)-1])
		}
	}
	return values
}

func acfValue(path, key string) string {
	if values := acfValues(path, key); len(values) > 0 {
		return values[0]
	}
	return ""
}

// desktopName is the Name of the application whose desktop entry runs
// binary, matched by window class first and then by the program its Exec
// line runs. Case is ignored: the binary "Discord" is started as "discord".
func (l *labeller) desktopName(binary string) string {
	if binary == "" {
		return ""
	}
	if !l.loaded {
		l.desktop = desktopEntries()
		l.loaded = true
	}
	return l.desktop[strings.ToLower(binary)]
}

// desktopEntries maps a lowercased window class or program name to the
// application's Name, for every visible desktop entry. The user's own
// entries override the system's, as they do in a launcher.
func desktopEntries() map[string]string {
	home, _ := os.UserHomeDir()
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	dirs := []string{filepath.Join(dataHome, "applications")}
	dataDirs := os.Getenv("XDG_DATA_DIRS")
	if dataDirs == "" {
		dataDirs = "/usr/local/share:/usr/share"
	}
	for _, dir := range strings.Split(dataDirs, ":") {
		dirs = append(dirs, filepath.Join(dir, "applications"))
	}

	byClass := map[string]string{}
	byExec := map[string]string{}
	for _, dir := range dirs {
		files, _ := filepath.Glob(filepath.Join(dir, "*.desktop"))
		for _, file := range files {
			name, class, exec, ok := readDesktop(file)
			if !ok {
				continue
			}
			if class != "" {
				if _, taken := byClass[class]; !taken {
					byClass[class] = name
				}
			}
			if exec != "" {
				if _, taken := byExec[exec]; !taken {
					byExec[exec] = name
				}
			}
		}
	}
	for key, name := range byClass {
		byExec[key] = name
	}
	return byExec
}

// readDesktop reads an entry's Name, window class and the program its Exec
// line runs, lowercased, skipping entries a launcher would not show.
func readDesktop(path string) (name, class, exec string, ok bool) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", "", false
	}
	defer file.Close()
	inEntry := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inEntry = line == "[Desktop Entry]"
			continue
		}
		if !inEntry {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch key {
		case "Name":
			name = value
		case "StartupWMClass":
			class = strings.ToLower(value)
		case "Exec":
			exec = execProgram(value)
		case "Hidden", "NoDisplay":
			if value == "true" {
				return "", "", "", false
			}
		}
	}
	return name, class, exec, name != ""
}

// execProgram is the program an Exec line runs, lowercased: past an `env`
// and its assignments, and with its directory taken off.
func execProgram(line string) string {
	for _, field := range strings.Fields(line) {
		field = strings.Trim(field, `"`)
		if field == "env" || field == "/usr/bin/env" || strings.Contains(field, "=") {
			continue
		}
		return strings.ToLower(filepath.Base(field))
	}
	return ""
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
