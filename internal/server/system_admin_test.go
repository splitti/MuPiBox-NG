package server

import "testing"

func TestParseBootTime(t *testing.T) {
	value := parseBootTime("Startup finished in 1.250s (kernel) + 3.400s (userspace) = 4.650s multi-user.target reached")
	if value.KernelMS != 1250 || value.UserspaceMS != 3400 || value.TotalMS != 4650 {
		t.Fatalf("unexpected boot time: %#v", value)
	}
}

func TestParseBootBlame(t *testing.T) {
	items := parseBootBlame("2.100s network.service\n 450ms audio.service\ninvalid\n", 10)
	if len(items) != 2 || items[0].Unit != "network.service" || items[0].DurationMS != 2100 || items[1].DurationMS != 450 {
		t.Fatalf("unexpected blame: %#v", items)
	}
}

func TestDurationMillis(t *testing.T) {
	for input, expected := range map[string]int64{"12ms": 12, "1.5s": 1500, "2min": 120000} {
		if actual := durationMillis(input); actual != expected {
			t.Fatalf("%s: got %d, want %d", input, actual, expected)
		}
	}
}
