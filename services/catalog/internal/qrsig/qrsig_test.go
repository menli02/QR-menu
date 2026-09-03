package qrsig

import "testing"

func TestSignVerify_roundTrip(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	sig := Sign(secret, "venue-1", "abcdef1234")
	if !Verify(secret, "venue-1", "abcdef1234", sig) {
		t.Fatal("a freshly generated signature failed to verify")
	}
}

func TestVerify_wrongSecretRejected(t *testing.T) {
	secretA, _ := GenerateSecret()
	secretB, _ := GenerateSecret()
	sig := Sign(secretA, "venue-1", "abcdef1234")
	if Verify(secretB, "venue-1", "abcdef1234", sig) {
		t.Fatal("signature verified against a different secret")
	}
}

func TestVerify_wrongVenueRejected(t *testing.T) {
	secret, _ := GenerateSecret()
	sig := Sign(secret, "venue-1", "abcdef1234")
	if Verify(secret, "venue-2", "abcdef1234", sig) {
		t.Fatal("signature for venue-1 verified for venue-2 — cross-venue replay should be impossible")
	}
}

func TestVerify_wrongTableCodeRejected(t *testing.T) {
	secret, _ := GenerateSecret()
	sig := Sign(secret, "venue-1", "abcdef1234")
	if Verify(secret, "venue-1", "zzzzzzzz99", sig) {
		t.Fatal("signature for one table_code verified for another")
	}
}

func TestVerify_malformedSigRejected(t *testing.T) {
	secret, _ := GenerateSecret()
	if Verify(secret, "venue-1", "abcdef1234", "not-base64url-!!!") {
		t.Fatal("malformed signature should never verify")
	}
	if Verify(secret, "venue-1", "abcdef1234", "") {
		t.Fatal("empty signature should never verify")
	}
}

func TestGenerateTableCode_uniqueAndLongEnough(t *testing.T) {
	a, err := GenerateTableCode()
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateTableCode()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two generated table codes collided")
	}
	if len(a) < 10 {
		t.Fatalf("table code shorter than FR-T2's 10-char minimum: %q", a)
	}
}

func TestGenerateSecret_unique(t *testing.T) {
	a, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	if string(a) == string(b) {
		t.Fatal("two generated secrets collided")
	}
}
