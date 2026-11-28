package sqlite

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"

	"jindi-drama/internal/fileprivacy"
)

func TestRollbackDoesNotInheritDestinationDACL(t *testing.T) {
	parent := t.TempDir()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + user.User.Sid.String() + ")(A;OICI;FR;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(parent, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	hasWorldRead := func(path string) bool {
		t.Helper()
		acl, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Contains(acl.String(), ";;;WD)")
	}
	probe := filepath.Join(parent, "probe")
	if err := os.WriteFile(probe, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !hasWorldRead(probe) {
		t.Fatal("fixture did not inherit Everyone access")
	}
	parentRoot, err := os.OpenRoot(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer parentRoot.Close()
	root, err := fileprivacy.CreatePrivateDirectory(parentRoot, "stage-probe")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if existing, err := fileprivacy.CreatePrivateDirectory(parentRoot, "stage-probe"); !errors.Is(err, os.ErrExist) {
		if existing != nil {
			existing.Close()
		}
		t.Fatalf("creating a private stage must not reopen an existing directory: %v", err)
	}
	stage := filepath.Join(parent, "stage-probe")
	protected, err := windows.GetNamedSecurityInfo(stage, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := protected.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 || hasWorldRead(stage) {
		t.Fatalf("stage DACL is not protected: %v %v", control, err)
	}
	if err := root.WriteFile("rollback.db-journal", []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if hasWorldRead(filepath.Join(stage, "rollback.db-journal")) {
		t.Fatal("staged journal inherited Everyone access")
	}
	source := filepath.Join(t.TempDir(), "current.db")
	store, err := Open(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(parent, "rollback.db")
	if err := PrepareRollbackCopy(source, target, 2); err != nil {
		t.Fatal(err)
	}
	if hasWorldRead(target) {
		t.Fatal("published rollback inherited Everyone access")
	}
	if err := VerifyDesktopRollback(target); err != nil {
		t.Fatal(err)
	}
}
