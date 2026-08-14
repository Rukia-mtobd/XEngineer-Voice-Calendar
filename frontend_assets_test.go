package main

import (
	"os"
	"strings"
	"testing"
)

func TestFrontendEntryUsesExternalAssets(t *testing.T) {
	data, err := os.ReadFile("frontend/index.html")
	if err != nil {
		t.Fatalf("read frontend entry: %v", err)
	}
	html := string(data)

	for _, want := range []string{
		`href="./styles/main.css"`,
		`src="./js/app.js"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("frontend entry does not reference %s", want)
		}
	}
	if strings.Contains(html, "<style>") {
		t.Error("frontend entry contains an inline style block")
	}
	if strings.Contains(html, `<script type="module">`) {
		t.Error("frontend entry contains an inline module script")
	}

	for _, path := range []string{"frontend/styles/main.css", "frontend/js/app.js"} {
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("external asset %s: %v", path, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("external asset %s is empty", path)
		}
	}
}
