package collect

import (
	"bufio"
	"io"
	"strings"
	"syscall"

	"hog.local/marvin-panel/internal/model"
)

// Mounts are the SPACE rows in wire order (D-019).
var Mounts = []string{"/", "/srv/hogdata", "/boot"}

// StatfsFunc is syscall.Statfs; injectable for tests.
type StatfsFunc func(path string, st *syscall.Statfs_t) error

type mountInfo struct{ device, fs string }

// parseMountinfo maps mount point -> source device and fs type from
// /proc/self/mountinfo; a later line for the same mount point wins (overmounts).
func parseMountinfo(r io.Reader) map[string]mountInfo {
	out := map[string]mountInfo{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		pre, post, ok := strings.Cut(sc.Text(), " - ")
		if !ok {
			continue
		}
		a, b := strings.Fields(pre), strings.Fields(post)
		if len(a) < 5 || len(b) < 2 {
			continue
		}
		out[a[4]] = mountInfo{device: b[1], fs: b[0]}
	}
	return out
}

// mount builds one SPACE row with df semantics (D-055): used = (blocks - bfree) x
// frsize, free = bavail x frsize, used_pct = used / (used + free). On a statfs error
// every measured field is nil.
func mount(mp string, mi mountInfo, statfs StatfsFunc) (model.Mount, error) {
	m := model.Mount{Mount: mp, Device: mi.device, FS: mi.fs}
	if m.Device == "" {
		m.Device = "unknown"
	}
	if m.FS == "" {
		m.FS = "unknown"
	}
	var st syscall.Statfs_t
	if err := statfs(mp, &st); err != nil {
		return m, err
	}
	bs := st.Frsize
	if bs <= 0 {
		bs = st.Bsize
	}
	total := int64(st.Blocks) * bs
	used := int64(st.Blocks-st.Bfree) * bs
	free := int64(st.Bavail) * bs
	m.TotalBytes, m.UsedBytes, m.FreeBytes = &total, &used, &free
	if used+free > 0 {
		p := r1(100 * float64(used) / float64(used+free))
		s := model.MountState(p)
		m.UsedPct, m.State = &p, &s
	}
	return m, nil
}
