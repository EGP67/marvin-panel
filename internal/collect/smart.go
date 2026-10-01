package collect

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"hog.local/marvin-panel/internal/model"
)

// SmartMaxAge is the staleness limit of the root-owned handoff file (D-012).
const SmartMaxAge = 3600 * time.Second

// smartFile holds ONLY the fields the panel shows; serial_number, wwn and uuid are
// never decoded, stored or logged.
type smartFile struct {
	Status *struct {
		Passed *bool `json:"passed"`
	} `json:"smart_status"`
	Log *struct {
		PercentageUsed  *int `json:"percentage_used"`
		UnsafeShutdowns *int `json:"unsafe_shutdowns"`
	} `json:"nvme_smart_health_information_log"`
}

// smartReading is one parse of the handoff file plus its mtime.
type smartReading struct {
	file  smartFile
	mtime time.Time
}

func readSmart(path string) (smartReading, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return smartReading{}, fmt.Errorf("smart: %w", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return smartReading{}, fmt.Errorf("smart: %w", err)
	}
	var f smartFile
	if err := json.Unmarshal(b, &f); err != nil {
		return smartReading{}, fmt.Errorf("smart: unparsable %s: %w", path, err)
	}
	return smartReading{file: f, mtime: fi.ModTime()}, nil
}

// smartAt renders a reading at wall time now: unknown when missing, unparsable or
// stale (counters null); failing when passed is false.
func smartAt(r *smartReading, now time.Time) model.Smart {
	unknown := model.Smart{State: "unknown"}
	if r == nil || r.file.Status == nil || r.file.Status.Passed == nil {
		return unknown
	}
	age := now.Sub(r.mtime)
	if age > SmartMaxAge {
		return unknown
	}
	s := model.Smart{State: "ok"}
	if !*r.file.Status.Passed {
		s.State = "failing"
	}
	secs := int(max(0, age/time.Second))
	s.AgeSeconds = &secs
	if r.file.Log != nil {
		s.PercentageUsed, s.UnsafeShutdowns = r.file.Log.PercentageUsed, r.file.Log.UnsafeShutdowns
	}
	return s
}
