package domain

import "testing"

// BenchmarkCompare measures the cost of a single version comparison,
// including qualifier parsing and rank resolution.
func BenchmarkCompare(b *testing.B) {
	vc := NewVersionComparator()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vc.Compare("2.14.6", "2.14.7-RC1")
	}
}

// BenchmarkParseVersion measures the cost of tokenizing a version string
// into numeric parts plus a normalized qualifier.
func BenchmarkParseVersion(b *testing.B) {
	vc := NewVersionComparator()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vc.ParseVersion("1.2.3-M2")
	}
}

// BenchmarkDetermineUpdateType covers the full update-classification path.
func BenchmarkDetermineUpdateType(b *testing.B) {
	vc := NewVersionComparator()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vc.DetermineUpdateType("1.2.3", "2.0.0")
	}
}

// BenchmarkTrailingNumber isolates the trailing-digit extraction used by
// qualifier rank resolution.
func BenchmarkTrailingNumber(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = trailingNumber("rc12")
	}
}
