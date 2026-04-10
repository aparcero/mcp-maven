package domain

import "testing"

func TestClassifyStabilityRecognizesQualifierVariants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		version string
		want    Stability
	}{
		{version: "1.0.0-RC1", want: StabilityRC},
		{version: "1.0.0-M2", want: StabilityMilestone},
		{version: "1.0.0-BETA2", want: StabilityBeta},
		{version: "1.0.0-ALPHA1", want: StabilityAlpha},
		{version: "1.0.0-SNAPSHOT", want: StabilitySnapshot},
		{version: "1.0.0-SP1", want: StabilityStable},
		{version: "1.0.0-custom", want: StabilityAlpha},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.version, func(t *testing.T) {
			t.Parallel()
			if got := ClassifyStability(tc.version); got != tc.want {
				t.Fatalf("ClassifyStability(%q) = %q, want %q", tc.version, got, tc.want)
			}
		})
	}
}

func TestVersionComparatorRespectsMavenQualifierOrdering(t *testing.T) {
	t.Parallel()

	vc := NewVersionComparator()
	assertPositive(t, vc.Compare("1.0.0", "1.0.0-RC1"), "release should sort after rc")
	assertPositive(t, vc.Compare("1.0.0-RC1", "1.0.0-M1"), "rc should sort after milestone")
	assertPositive(t, vc.Compare("1.0.0-M1", "1.0.0-BETA1"), "milestone should sort after beta")
	assertPositive(t, vc.Compare("1.0.0-BETA1", "1.0.0-ALPHA1"), "beta should sort after alpha")
}

func TestVersionComparatorTreatsQualifierPromotionAsPatchUpgrade(t *testing.T) {
	t.Parallel()

	vc := NewVersionComparator()
	if got := vc.DetermineUpdateType("2.0.0-M1", "2.0.0"); got != UpdatePatch {
		t.Fatalf("DetermineUpdateType qualifier promotion = %q, want %q", got, UpdatePatch)
	}
}

func assertPositive(t *testing.T, got int, msg string) {
	t.Helper()
	if got <= 0 {
		t.Fatalf("%s: got %d, want > 0", msg, got)
	}
}
