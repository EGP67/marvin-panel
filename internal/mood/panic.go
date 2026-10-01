package mood

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// PanicFile is the PANIC COUNT store inside the state directory (D-017, D-058).
const PanicFile = "panic_count"

// panicStore holds the lifetime count. A count that cannot be read is null and the file
// is never overwritten (broken).
type panicStore struct {
	path   string
	count  *int
	broken bool
}

// loadPanic reads dir/panic_count; a missing file is created with 0.
func loadPanic(dir string) (*panicStore, error) {
	p := &panicStore{path: filepath.Join(dir, PanicFile)}
	b, err := os.ReadFile(p.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := writeAtomic(p.path, 0); err != nil {
			p.broken = true
			return p, fmt.Errorf("create %s: %w", p.path, err)
		}
		zero := 0
		p.count = &zero
		return p, nil
	case err != nil:
		p.broken = true
		return p, fmt.Errorf("read %s: %w", p.path, err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n < 0 {
		p.broken = true
		return p, fmt.Errorf("corrupt %s: %q", p.path, strings.TrimSpace(string(b)))
	}
	p.count = &n
	return p, nil
}

// increment adds one confirmed doomed entry and persists it; a broken store stays null.
func (p *panicStore) increment() error {
	if p.broken || p.count == nil {
		return nil
	}
	n := *p.count + 1
	if err := writeAtomic(p.path, n); err != nil {
		return fmt.Errorf("write %s: %w", p.path, err)
	}
	p.count = &n
	return nil
}

// writeAtomic writes "n\n" via temp file, fsync, rename and directory fsync; mode 0640.
// On any failure the temp file is removed and the original file is untouched.
func writeAtomic(path string, n int) (err error) {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".panic_count-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	renamed := false
	defer func() {
		if !renamed {
			err = errors.Join(err, os.Remove(tmp))
		}
	}()
	if _, err := fmt.Fprintf(f, "%d\n", n); err != nil {
		return errors.Join(err, f.Close())
	}
	if err := f.Chmod(0o640); err != nil {
		return errors.Join(err, f.Close())
	}
	if err := f.Sync(); err != nil {
		return errors.Join(err, f.Close())
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	renamed = true
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	return errors.Join(d.Sync(), d.Close())
}
