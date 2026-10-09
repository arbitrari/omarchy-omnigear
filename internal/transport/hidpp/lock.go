package hidpp

import (
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"
)

// Every omnigear process sends the same software id, and a reply carries
// nothing else that says whose request it answers: two processes asking one
// node for different features at once can each take the other's reply, and
// write to the feature index that came back. The bar polls with `list` while
// a `set` the user just asked for runs, so that overlap is ordinary.
//
// So a process holds an exclusive flock on a per-node lock file for as long
// as it has any conversation open on that node. Within one process the lock
// is shared by reference count: the concurrent index sweeps address different
// device indexes, which replies do carry, and serializing them would cost a
// timeout per empty slot.
var nodeLocks = struct {
	sync.Mutex
	held map[string]*nodeLock
}{held: map[string]*nodeLock{}}

type nodeLock struct {
	once sync.Once
	file *os.File
	refs int
}

// lockNode returns once this process holds the node, and the function that
// lets it go. A lock file that cannot be made leaves the node unlocked rather
// than unusable.
func lockNode(path string) func() {
	nodeLocks.Lock()
	lock, ok := nodeLocks.held[path]
	if !ok {
		lock = &nodeLock{}
		nodeLocks.held[path] = lock
	}
	lock.refs++
	nodeLocks.Unlock()

	// Waited for outside the table's mutex, so another process holding one
	// node does not hold up this one's conversations with the rest.
	lock.once.Do(func() {
		name := "omnigear-" + filepath.Base(path) + ".lock"
		file, err := os.OpenFile(filepath.Join(lockDir(), name), os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return
		}
		if unix.Flock(int(file.Fd()), unix.LOCK_EX) != nil {
			file.Close()
			return
		}
		lock.file = file
	})

	return func() {
		nodeLocks.Lock()
		defer nodeLocks.Unlock()
		lock.refs--
		if lock.refs > 0 {
			return
		}
		delete(nodeLocks.held, path)
		if lock.file != nil {
			lock.file.Close()
		}
	}
}

func lockDir() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir
	}
	return os.TempDir()
}
