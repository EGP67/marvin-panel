package model

import "testing"

func fp(v float64) *float64 { return &v }

func agreementSnap(total *float64, lastHist *float64, perThread ...float64) *Snapshot {
	s := &Snapshot{CPU: CPU{Threads: len(perThread), TotalPct: total}}
	for _, v := range perThread {
		s.CPU.PerThreadPct = append(s.CPU.PerThreadPct, fp(v))
		sev := Severity(v)
		s.CPU.PerThreadSev = append(s.CPU.PerThreadSev, &sev)
	}
	s.CPU.HistPct = []*float64{nil, fp(1), lastHist}
	return s
}

func TestCheckAgreement(t *testing.T) {
	short := agreementSnap(fp(50), fp(50), 50, 50)
	short.CPU.PerThreadSev = short.CPU.PerThreadSev[:1]
	nilTotal := agreementSnap(nil, nil, 0, 100)
	cases := []struct {
		name    string
		s       *Snapshot
		wantErr bool
	}{
		{"pass", agreementSnap(fp(50), fp(50), 40, 60, 50), false},
		{"pass at tolerance edge", agreementSnap(fp(50), fp(50), 62, 62), false},
		{"nil total", nilTotal, false},
		{"last hist mismatch", agreementSnap(fp(50), fp(49.9), 50, 50), true},
		{"last hist nil", agreementSnap(fp(50), nil, 50, 50), true},
		{"mean out of tolerance", agreementSnap(fp(50), fp(50), 62.1, 62.1), true},
		{"per_thread_pct length mismatch", func() *Snapshot { s := agreementSnap(fp(50), fp(50), 50, 50); s.CPU.Threads = 3; return s }(), true},
		{"per_thread_sev length mismatch", short, true},
	}
	for _, tc := range cases {
		if err := CheckAgreement(tc.s); (err != nil) != tc.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", tc.name, err, tc.wantErr)
		}
	}
}
