package mavencentral

import (
	"testing"

	"github.com/aparcero/mcp-maven/internal/config"
)

// TestParseLicensesFromPOM covers the license extractor, including the edge
// cases the previous string-scanning implementation handled incorrectly
// (namespaced POMs, CDATA, comments) and the contract that licenses without a
// name are dropped while the result is never nil.
func TestParseLicensesFromPOM(t *testing.T) {
	t.Parallel()

	client := NewClient(config.MavenCentralConfig{}, nil)

	t.Run("namespaced POM extracts name and url", func(t *testing.T) {
		t.Parallel()
		// The default Maven namespace on <project> used to defeat the old
		// string scanner, which searched for the literal "<license>".
		pom := []byte(`<project xmlns="http://maven.apache.org/POM/4.0.0">
			<licenses>
				<license>
					<name>Apache License, Version 2.0</name>
					<url>https://www.apache.org/licenses/LICENSE-2.0.txt</url>
				</license>
			</licenses>
		</project>`)

		licenses := client.parseLicensesFromPOM(pom)
		if len(licenses) != 1 {
			t.Fatalf("expected 1 license, got %d", len(licenses))
		}
		if licenses[0].Name != "Apache License, Version 2.0" {
			t.Fatalf("unexpected name: %q", licenses[0].Name)
		}
		if licenses[0].URL != "https://www.apache.org/licenses/LICENSE-2.0.txt" {
			t.Fatalf("unexpected url: %q", licenses[0].URL)
		}
	})

	t.Run("multiple licenses preserved in order", func(t *testing.T) {
		t.Parallel()
		pom := []byte(`<project xmlns="http://maven.apache.org/POM/4.0.0">
			<licenses>
				<license><name>MIT</name><url>https://opensource.org/license/mit</url></license>
				<license><name>EPL-2.0</name><url>https://www.eclipse.org/legal/epl-2.0/</url></license>
			</licenses>
		</project>`)

		licenses := client.parseLicensesFromPOM(pom)
		if len(licenses) != 2 {
			t.Fatalf("expected 2 licenses, got %d", len(licenses))
		}
		if licenses[0].Name != "MIT" || licenses[1].Name != "EPL-2.0" {
			t.Fatalf("unexpected order: %+v", licenses)
		}
	})

	t.Run("license without name is dropped", func(t *testing.T) {
		t.Parallel()
		pom := []byte(`<project xmlns="http://maven.apache.org/POM/4.0.0">
			<licenses>
				<license><url>https://example.org/license</url></license>
				<license><name>Real License</name></license>
			</licenses>
		</project>`)

		licenses := client.parseLicensesFromPOM(pom)
		if len(licenses) != 1 {
			t.Fatalf("expected nameless license to be dropped, got %d", len(licenses))
		}
		if licenses[0].Name != "Real License" {
			t.Fatalf("unexpected name: %q", licenses[0].Name)
		}
	})

	t.Run("CDATA and comments do not break extraction", func(t *testing.T) {
		t.Parallel()
		pom := []byte(`<project xmlns="http://maven.apache.org/POM/4.0.0">
			<licenses>
				<!-- primary license -->
				<license>
					<name><![CDATA[BSD 3-Clause]]></name>
					<url>https://opensource.org/licenses/BSD-3-Clause</url>
				</license>
			</licenses>
		</project>`)

		licenses := client.parseLicensesFromPOM(pom)
		if len(licenses) != 1 {
			t.Fatalf("expected 1 license, got %d", len(licenses))
		}
		if licenses[0].Name != "BSD 3-Clause" {
			t.Fatalf("unexpected name: %q", licenses[0].Name)
		}
	})

	t.Run("no licenses returns non-nil empty slice", func(t *testing.T) {
		t.Parallel()
		pom := []byte(`<project xmlns="http://maven.apache.org/POM/4.0.0">
			<groupId>org.example</groupId>
		</project>`)

		licenses := client.parseLicensesFromPOM(pom)
		if licenses == nil {
			t.Fatal("expected non-nil slice when POM has no licenses")
		}
		if len(licenses) != 0 {
			t.Fatalf("expected 0 licenses, got %d", len(licenses))
		}
	})

	t.Run("malformed XML returns empty slice", func(t *testing.T) {
		t.Parallel()
		licenses := client.parseLicensesFromPOM([]byte(`<not><closed>`))
		if len(licenses) != 0 {
			t.Fatalf("expected 0 licenses for malformed XML, got %d", len(licenses))
		}
	})

	t.Run("whitespace around name and url is trimmed", func(t *testing.T) {
		t.Parallel()
		pom := []byte(`<project xmlns="http://maven.apache.org/POM/4.0.0">
			<licenses>
				<license>
					<name>  Apache-2.0
					</name>
					<url>  https://www.apache.org/licenses/LICENSE-2.0  </url>
				</license>
			</licenses>
		</project>`)

		licenses := client.parseLicensesFromPOM(pom)
		if len(licenses) != 1 {
			t.Fatalf("expected 1 license, got %d", len(licenses))
		}
		if licenses[0].Name != "Apache-2.0" {
			t.Fatalf("unexpected trimmed name: %q", licenses[0].Name)
		}
		if licenses[0].URL != "https://www.apache.org/licenses/LICENSE-2.0" {
			t.Fatalf("unexpected trimmed url: %q", licenses[0].URL)
		}
	})
}
