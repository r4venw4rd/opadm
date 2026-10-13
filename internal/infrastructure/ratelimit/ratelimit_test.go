package ratelimit

import (
	"testing"
	"time"
)

func TestCleanup(t *testing.T) {
	l := New(10, 50*time.Millisecond)
	l.Allow("1.1.1.1")
	l.Allow("2.2.2.2")
	if l.Size() != 2 {
		t.Fatalf("Size = %d, istendi 2", l.Size())
	}
	time.Sleep(80 * time.Millisecond)
	l.Cleanup()
	if l.Size() != 0 {
		t.Fatalf("Cleanup sonrası Size = %d, istendi 0", l.Size())
	}
}

func TestStaleEntryDeletedOnAllow(t *testing.T) {
	l := New(10, 30*time.Millisecond)
	l.Allow("9.9.9.9")
	time.Sleep(50 * time.Millisecond)
	if !l.Allow("9.9.9.9") {
		t.Fatal("pencere dolunca Allow true dönmeli")
	}
	// Tek IP'lik taze girdi kalmalı, şişkinlik yok.
	if l.Size() != 1 {
		t.Fatalf("Size = %d, istendi 1", l.Size())
	}
}

func TestRelease(t *testing.T) {
	l := New(10, time.Minute)
	l.Allow("1.1.1.1")
	l.Allow("2.2.2.2")
	l.Release("1.1.1.1")
	if l.Size() != 1 {
		t.Fatalf("Size = %d, istendi 1", l.Size())
	}
	l.Release("yok")
	if l.Size() != 1 {
		t.Fatalf("olmayan anahtar Size oynatmamalı: %d", l.Size())
	}
}
