package collect

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"hog.local/marvin-panel/internal/model"
)

// DefaultNvidiaSMI is the only NVIDIA access marvind has (D-008, D-024).
const DefaultNvidiaSMI = "/usr/bin/nvidia-smi"

// smiTimeout bounds one query; a slower answer marks the tick stale (D-008).
const smiTimeout = 750 * time.Millisecond

var smiArgs = []string{
	"--query-gpu=index,name,uuid,pci.bus_id,memory.total,memory.used,temperature.gpu,utilization.gpu,power.draw,power.limit",
	"--format=csv,noheader,nounits",
}

// gpuSlots binds PCI addresses to GPU0/GPU1 at the first successful query (D-057).
var gpuSlots = map[string]int{"00000000:01:00.0": 0, "00000000:06:00.0": 1}

// gpuRow is one parsed nvidia-smi line; nil means [N/A], [Not Supported] or unparsable.
type gpuRow struct {
	name, uuid, bus          string
	memTotal, memUsed        *int
	temp, util, power, limit *float64
}

func smiFloat(s string) *float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return nil
	}
	v = r1(v)
	return &v
}

func smiInt(s string) *int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return nil
	}
	return &v
}

// parseGPUCSV parses the query output; malformed lines are skipped.
func parseGPUCSV(b []byte) []gpuRow {
	var rows []gpuRow
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		f := strings.Split(sc.Text(), ",")
		if len(f) != 10 {
			continue
		}
		for i := range f {
			f[i] = strings.TrimSpace(f[i])
		}
		rows = append(rows, gpuRow{
			name: f[1], uuid: f[2], bus: strings.ToUpper(f[3]),
			memTotal: smiInt(f[4]), memUsed: smiInt(f[5]),
			temp: smiFloat(f[6]), util: smiFloat(f[7]), power: smiFloat(f[8]), limit: smiFloat(f[9]),
		})
	}
	return rows
}

// runSMI executes one bounded query. WaitDelay keeps a child that ignores the kill
// from holding the pipe open past the timeout.
func runSMI(parent context.Context, path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, smiTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, smiArgs...)
	cmd.WaitDelay = 100 * time.Millisecond
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("nvidia-smi: no answer within %s: %w", smiTimeout, ctx.Err())
	}
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi: %w", err)
	}
	return out, nil
}

// gpuCard is one bound slot: its UUID binding, last good identity and history.
type gpuCard struct {
	uuid     string // in memory and on the wire only; never logged
	name     string
	memTotal *int
	limit    *float64
	hist     *model.Ring[*float64]
}

// gpuSet tracks both slots across ticks.
type gpuSet struct {
	cards [2]gpuCard
}

func newGPUSet() *gpuSet {
	g := &gpuSet{}
	for i := range g.cards {
		g.cards[i] = gpuCard{name: gpuName, hist: model.NewRing[*float64](gpuHistLen)}
	}
	return g
}

// apply matches rows to slots (PCI until bound, UUID afterwards) and returns both
// wire entries plus the GPU-line state. bound reports each first-time binding's bus.
func (g *gpuSet) apply(rows []gpuRow, bound func(slot int, bus string)) ([]model.GPU, []model.GPUState) {
	cur := [2]*gpuRow{}
	for i := range rows {
		r := &rows[i]
		for slot := range g.cards {
			c := &g.cards[slot]
			if c.uuid == "" {
				if s, ok := gpuSlots[r.bus]; ok && s == slot && r.uuid != "" {
					c.uuid = r.uuid
					bound(slot, r.bus)
					cur[slot] = r
				}
			} else if r.uuid == c.uuid {
				cur[slot] = r
			}
		}
	}
	out := make([]model.GPU, 2)
	states := make([]model.GPUState, 2)
	for slot := range g.cards {
		c := &g.cards[slot]
		e := model.GPU{UUID: gpuUnbound}
		if r := cur[slot]; r != nil {
			if r.name != "" {
				c.name = r.name
			}
			if r.memTotal != nil {
				c.memTotal = r.memTotal
			}
			if r.limit != nil {
				c.limit = r.limit
			}
			e.MemUsedMiB, e.UtilPct, e.TempC, e.PowerW = r.memUsed, r.util, r.temp, r.power
			if r.temp != nil {
				b := model.ThermalBand(*r.temp)
				e.ThermalBand = &b
			}
			if r.util != nil {
				b := model.Severity(*r.util)
				e.UtilSev = &b
			}
		}
		if c.uuid != "" {
			e.UUID = c.uuid
		}
		e.Name, e.DisplayName = c.name, model.GPUDisplayName(c.name)
		e.MemTotalMiB, e.PowerLimitW = c.memTotal, c.limit
		c.hist.Push(e.UtilPct)
		vals := c.hist.Values()
		e.HistUtilPct = append(make([]*float64, gpuHistLen-len(vals)), vals...)
		out[slot] = e
		states[slot] = model.GPUState{TempC: e.TempC, UtilPct: e.UtilPct, MemUsedMiB: e.MemUsedMiB}
	}
	return out, states
}
