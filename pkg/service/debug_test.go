// SPDX-License-Identifier: MIT

package service

import "testing"

func TestSetDebug(t *testing.T) {
	prev := debugEnabled
	t.Cleanup(func() { debugEnabled = prev })

	SetDebug(true)
	if !debugEnabled {
		t.Fatal("expected debugEnabled=true")
	}
	debugf("hello %s", "world") // exercise enabled path

	SetDebug(false)
	if debugEnabled {
		t.Fatal("expected debugEnabled=false")
	}
	debugf("silent") // exercise disabled path
}
