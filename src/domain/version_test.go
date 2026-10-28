package domain

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"10.0", "9.9", 1},
		{"1.2", "1.2.0.0", 0},
		{"24.08", "24.8", 0},
		{"1.2.3", "1.2.4", -1},
		{"0", "0.0", 0},
		{"99999999999999999999999", "2", 1},
	}
	for _, tc := range cases {
		got, err := CompareVersions(tc.a, tc.b)
		if err != nil || got != tc.want {
			t.Errorf("%s vs %s: got %d, err %v; want %d", tc.a, tc.b, got, err, tc.want)
		}
	}
	for _, v := range []string{"", "v1.2", "1.beta", "1..2", "-1", "1.0 "} {
		if ValidVersion(v) {
			t.Fatalf("aceitou versão inválida %q", v)
		}
	}
}
func TestExitCodePrecedence(t *testing.T) {
	tests := []struct {
		results []Result
		want    int
	}{
		{nil, ExitOK},
		{[]Result{{Outcome: OutcomeSkipped}}, ExitOK},
		{[]Result{{Outcome: OutcomeInstalledReboot}}, ExitReboot},
		{[]Result{{Outcome: OutcomeFailed, Reason: ReasonTimeout}}, ExitPartial},
		{[]Result{{Outcome: OutcomeInstalledReboot}, {Outcome: OutcomeFailed, Reason: ReasonExitCode}}, ExitPartial},
		{[]Result{{Outcome: OutcomeFailed, Reason: ReasonIntegrity}, {Outcome: OutcomeInstalledReboot}}, ExitIntegrity},
	}
	for _, tc := range tests {
		if got := ExitCodeFor(tc.results); got != tc.want {
			t.Errorf("got %d want %d", got, tc.want)
		}
	}
}
