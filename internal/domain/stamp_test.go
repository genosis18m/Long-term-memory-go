package domain

import "testing"

// Which conversation a reopened domain resumes is decided by the scene record's last-used stamp, whose
// unit is milliseconds.
func TestNextUsedStampIsStrictlyIncreasing(t *testing.T) {
	c := &Context{}
	const now = int64(1_700_000_000_000)
	var seen []int64
	for i := 0; i < 4; i++ {
		seen = append(seen, c.NextUsedStamp(now))
	}
	for i := 1; i < len(seen); i++ {
		if seen[i] <= seen[i-1] {
			t.Fatalf("stamps are not strictly increasing: %v", seen)
		}
	}
	// A clock that moves forward is taken as-is; a stamp equal to the last one issued is
	// nudged, so the field keeps the unit its records promise.
	if got := c.NextUsedStamp(now + 1000); got != now+1000 {
		t.Fatalf("a later clock reading became %d, want the reading itself", got)
	}
	if got := c.NextUsedStamp(now + 1); got != now+1001 {
		t.Fatalf("a stale clock reading went backwards: %d", got)
	}
}
