//go:build darwin && cgo

package secrets

import "testing"

func TestKeychainOperationsRestoreInteractionState(t *testing.T) {
	original, err := keychainInteractionAllowed()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := setKeychainInteraction(original); err != nil {
			t.Error(err)
		}
	})

	for _, initial := range []bool{true, false} {
		if err := setKeychainInteraction(initial); err != nil {
			t.Fatal(err)
		}
		if err := withoutKeychainInteraction(func() {
			if allowed, err := keychainInteractionAllowed(); allowed || err != nil {
				t.Errorf("during Keychain operation, allowed=%v, err=%v", allowed, err)
			}
		}); err != nil {
			t.Fatal(err)
		}
		if restored, err := keychainInteractionAllowed(); restored != initial || err != nil {
			t.Fatalf("initial=%v, restored=%v, err=%v", initial, restored, err)
		}
	}
}
