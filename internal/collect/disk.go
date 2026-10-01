package collect

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// diskSectors returns sectors read (field 6) and written (field 10) for dev from
// /proc/diskstats (fields 1-based: major, minor, name, ...).
func diskSectors(r io.Reader, dev string) (read, written int64, err error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 10 || f[2] != dev {
			continue
		}
		if read, err = strconv.ParseInt(f[5], 10, 64); err != nil {
			return 0, 0, fmt.Errorf("diskstats %s: %w", dev, err)
		}
		if written, err = strconv.ParseInt(f[9], 10, 64); err != nil {
			return 0, 0, fmt.Errorf("diskstats %s: %w", dev, err)
		}
		return read, written, nil
	}
	if err := sc.Err(); err != nil {
		return 0, 0, fmt.Errorf("diskstats: %w", err)
	}
	return 0, 0, fmt.Errorf("diskstats: no %s line", dev)
}
