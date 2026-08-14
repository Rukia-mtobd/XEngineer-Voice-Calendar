package main

import "testing"

func TestValidHHMM(t *testing.T) {
	for _, value := range []string{"00:00", "09:30", "23:59"} {
		if !validHHMM(value) {
			t.Errorf("validHHMM(%q) = false", value)
		}
	}
	for _, value := range []string{"", "9:30", "24:00", "12:60", "99:99"} {
		if validHHMM(value) {
			t.Errorf("validHHMM(%q) = true", value)
		}
	}
}
