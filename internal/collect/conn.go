package collect

import (
	"bufio"
	"io"
	"strings"
)

// countEstablished counts /proc/net/tcp{,6} rows in state 01 (ESTABLISHED, D-036).
func countEstablished(r io.Reader) (int, error) {
	n := 0
	sc := bufio.NewScanner(r)
	first := true
	for sc.Scan() {
		if first {
			first = false
			continue
		}
		f := strings.Fields(sc.Text())
		if len(f) > 3 && f[3] == "01" {
			n++
		}
	}
	return n, sc.Err()
}
