package collect

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"hog.local/marvin-panel/internal/model"
)

// Fan roles in display order (D-057, D-063): nct6687 on the MSI MS-7D75, each role
// bound to its input by name. fan1 is the CPU cooler (not shown).
var fanBanks = [2]struct {
	label  string
	input  string
	maxRPM int
}{
	{model.FanIntakeLabel, "fan6_input", 2000},  // header SYS_FAN4, intake chain, left cell
	{model.FanExhaustLabel, "fan3_input", 2000}, // header SYS_FAN1, exhaust chain, right cell
}

// hwmonPaths are the winning sensor files; "" when the chip or file is missing.
type hwmonPaths struct {
	cpuTemp, nvmeTemp, nvmeMax string
	fans                       [2]string
}

func (p hwmonPaths) String() string {
	return fmt.Sprintf("cpu=%s nvme=%s nvme_max=%s fan_intake=%s fan_exhaust=%s",
		p.cpuTemp, p.nvmeTemp, p.nvmeMax, p.fans[0], p.fans[1])
}

// scanHwmon matches chips by name, then sensors by label (DATA.md): k10temp "Tctl" is
// the CPU, nvme "Composite" the NVMe; nct6687 carries the intake and exhaust fans. amdgpu (the
// display adapter) and mt7921_phy0 (Wi-Fi) are never chosen.
func scanHwmon(root string) hwmonPaths {
	var p hwmonPaths
	dirs, err := filepath.Glob(filepath.Join(root, "sys/class/hwmon/hwmon*"))
	if err != nil {
		return p
	}
	for _, d := range dirs {
		name, err := readTrim(filepath.Join(d, "name"))
		if err != nil {
			continue
		}
		switch name {
		case "k10temp":
			if in := labelInput(d, "Tctl"); in != "" {
				p.cpuTemp = in
			}
		case "nvme":
			if in := labelInput(d, "Composite"); in != "" && p.nvmeTemp == "" {
				p.nvmeTemp = in
				if max := strings.TrimSuffix(in, "_input") + "_max"; exists(max) {
					p.nvmeMax = max
				}
			}
		case "nct6687":
			for i, b := range fanBanks {
				if f := filepath.Join(d, b.input); exists(f) {
					p.fans[i] = f
				}
			}
		}
	}
	return p
}

// labelInput returns the temp*_input whose temp*_label equals label.
func labelInput(dir, label string) string {
	labels, err := filepath.Glob(filepath.Join(dir, "temp*_label"))
	if err != nil {
		return ""
	}
	for _, l := range labels {
		if v, err := readTrim(l); err == nil && v == label {
			if in := strings.TrimSuffix(l, "_label") + "_input"; exists(in) {
				return in
			}
		}
	}
	return ""
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// readMilli reads a millidegree file as degrees rounded to places decimals.
func readMilli(path string, places int) (*float64, error) {
	if path == "" {
		return nil, nil
	}
	s, err := readTrim(path)
	if err != nil {
		return nil, err
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	k := math.Pow(10, float64(places))
	v = math.Round(v/1000*k) / k
	return &v, nil
}

func readRPM(path string) (*int, error) {
	if path == "" {
		return nil, nil
	}
	s, err := readTrim(path)
	if err != nil {
		return nil, err
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &v, nil
}

// hwmonValues is one read of every hwmon sensor the panel shows.
type hwmonValues struct {
	cpuTemp, nvmeTemp, nvmeMax *float64
	rpm                        [2]*int
}

// readHwmon reads p; failed reports whether any chosen file failed (rescan next time).
func readHwmon(p hwmonPaths) (hwmonValues, bool) {
	var v hwmonValues
	failed := false
	var err error
	if v.cpuTemp, err = readMilli(p.cpuTemp, 1); err != nil {
		failed = true
	}
	if v.nvmeTemp, err = readMilli(p.nvmeTemp, 1); err != nil {
		failed = true
	}
	if v.nvmeMax, err = readMilli(p.nvmeMax, 2); err != nil {
		failed = true
	}
	for i := range v.rpm {
		if v.rpm[i], err = readRPM(p.fans[i]); err != nil {
			failed = true
		}
	}
	return v, failed
}

// fans renders both roles in display order; verdict via model.FanVerdict on
// cpu.thermal_band.
func fans(rpm [2]*int, cpuThermal *model.Band) []model.Fan {
	out := make([]model.Fan, 2)
	for i := range out {
		f := model.Fan{Bank: i + 1, Label: fanBanks[i].label, RPM: rpm[i]}
		if rpm[i] != nil {
			m := fanBanks[i].maxRPM
			f.MaxRPM = &m
		}
		f.Verdict = model.FanVerdict(f.RPM, f.MaxRPM, cpuThermal)
		out[i] = f
	}
	return out
}
