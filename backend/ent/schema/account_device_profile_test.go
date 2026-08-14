package schema

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"entgo.io/ent/dialect/entsql"
)

func TestAccountDeviceProfileEntCheckNamesAreSQLSubset(t *testing.T) {
	entChecks := accountDeviceProfileEntChecks(t)
	sqlNames := accountDeviceProfileSQLConstraintNames(t)

	if len(entChecks) == 0 {
		t.Fatal("expected Ent Checks, got none")
	}
	if len(sqlNames) == 0 {
		t.Fatal("expected SQL constraint names, got none")
	}

	for name := range entChecks {
		if _, ok := sqlNames[name]; !ok {
			t.Errorf("Ent check %q is not a constraint name in 234_account_device_profiles.sql", name)
		}
	}
}

func accountDeviceProfileEntChecks(t *testing.T) map[string]string {
	t.Helper()
	for _, ann := range (AccountDeviceProfile{}).Annotations() {
		switch a := ann.(type) {
		case entsql.Annotation:
			if a.Checks != nil {
				return a.Checks
			}
		case *entsql.Annotation:
			if a != nil && a.Checks != nil {
				return a.Checks
			}
		}
	}
	t.Fatal("AccountDeviceProfile annotations missing entsql Checks")
	return nil
}

func accountDeviceProfileSQLConstraintNames(t *testing.T) map[string]struct{} {
	t.Helper()
	path := filepath.Join("..", "..", "migrations", "234_account_device_profiles.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	re := regexp.MustCompile(`(?m)^\s*CONSTRAINT\s+(\S+)`)
	names := make(map[string]struct{})
	for _, match := range re.FindAllStringSubmatch(string(body), -1) {
		names[match[1]] = struct{}{}
	}
	return names
}
