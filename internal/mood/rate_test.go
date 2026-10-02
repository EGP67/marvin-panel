package mood

import (
	"testing"
	"time"

	"hog.local/marvin-panel/internal/model"
)

// TestFormatRate: automatic units (D-062); never "1024.0 KIB/S".
func TestFormatRate(t *testing.T) {
	cases := []struct {
		bps  int64
		want string
	}{
		{0, "0 B/S"},
		{512, "512 B/S"},
		{1023, "1023 B/S"},
		{1024, "1.0 KIB/S"},
		{6451, "6.3 KIB/S"},
		{20304, "19.8 KIB/S"},
		{1048474, "1023.9 KIB/S"},
		{1048575, "1.0 MIB/S"},
		{1048576, "1.0 MIB/S"},
		{41943040, "40.0 MIB/S"},
		{209715200, "200.0 MIB/S"},
		{1048471142, "999.9 MIB/S"},
	}
	for _, c := range cases {
		if got := formatRate(c.bps); got != c.want {
			t.Errorf("formatRate(%d) = %q, want %q", c.bps, got, c.want)
		}
	}
	for bps := int64(1048000); bps < 1048576; bps++ {
		if got := formatRate(bps); got == "1024.0 KIB/S" {
			t.Fatalf("formatRate(%d) = %q", bps, got)
		}
	}
}

// TestRPMByRole: {rpm} is the EXHAUST FANS rpm wherever it sits in fans[]; S1 names the
// first fan without telemetry (D-063). Values as in the collector's binding test.
func TestRPMByRole(t *testing.T) {
	intake := model.Fan{Bank: 1, Label: model.FanIntakeLabel, RPM: ip(1700), MaxRPM: ip(2000), Verdict: "ok"}
	exh := model.Fan{Bank: 2, Label: model.FanExhaustLabel, RPM: ip(1450), MaxRPM: ip(2000), Verdict: "ok"}
	for _, fans := range [][]model.Fan{{intake, exh}, {exh, intake}} {
		s := idle()
		s.Fans = fans
		if got := mustRender(t, "C7", view{s: s, now: t0}); got != "THE FANS ARE AT 1450 RPM. CALM. THE CALM BEFORE THE OTHER THING." {
			t.Errorf("C7 = %q", got)
		}
	}
	s := idle()
	s.Fans = []model.Fan{intake, exh}
	s.Fans[0].RPM = nil
	v := view{s: s, now: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	if got := mustRender(t, "S1", v); got != "INTAKE FANS: NO TELEMETRY. I'M COOLING BY FORCE OF WILL." {
		t.Errorf("S1 = %q", got)
	}
	if got := mustRender(t, "M9", v); got != "THE FANS HAVE NOTICED. 1450 RPM. AT LEAST SOMEONE IS LISTENING." {
		t.Errorf("M9 with the intake silent = %q", got)
	}
	s.Fans[0].RPM, s.Fans[1].RPM = ip(1700), nil
	if got := mustRender(t, "S1", v); got != "EXHAUST FANS: NO TELEMETRY. I'M COOLING BY FORCE OF WILL." {
		t.Errorf("S1 = %q", got)
	}
	if _, ok := table[indexOf(t, "M9")].text(v); ok {
		t.Error("M9 spoke with the exhaust fans silent")
	}
}
