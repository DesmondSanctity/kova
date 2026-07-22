package secretbox

import "testing"

func TestRoundTrip(t *testing.T) {
	b, err := New("master-key")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, pt := range []string{"", "sk_live_secret", "MK_TEST_ABC123", "a longer secret with spaces & symbols !@#"} {
		enc, err := b.Encrypt(pt)
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", pt, err)
		}
		if pt != "" && enc == pt {
			t.Fatalf("ciphertext equals plaintext for %q", pt)
		}
		got, err := b.Decrypt(enc)
		if err != nil {
			t.Fatalf("Decrypt: %v", err)
		}
		if got != pt {
			t.Fatalf("round-trip = %q; want %q", got, pt)
		}
	}
}

func TestEmptyKeyRejected(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestNonceIsRandom(t *testing.T) {
	b, _ := New("k")
	a, _ := b.Encrypt("same")
	c, _ := b.Encrypt("same")
	if a == c {
		t.Fatal("expected different ciphertexts for the same plaintext (random nonce)")
	}
}

func TestWrongKeyFails(t *testing.T) {
	b1, _ := New("key-one")
	b2, _ := New("key-two")
	enc, _ := b1.Encrypt("secret")
	if _, err := b2.Decrypt(enc); err == nil {
		t.Fatal("decrypt with wrong key should fail")
	}
}

func TestTamperDetected(t *testing.T) {
	b, _ := New("k")
	enc, _ := b.Encrypt("secret")
	// Flip a character in the base64 ciphertext.
	tampered := []byte(enc)
	tampered[len(tampered)-2] ^= 0x01
	if _, err := b.Decrypt(string(tampered)); err == nil {
		t.Fatal("tampered ciphertext should fail authentication")
	}
}
