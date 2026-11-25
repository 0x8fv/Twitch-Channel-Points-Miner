package dashboard

import (
	"bytes"
	"io/fs"
	"sort"
	"testing"
)

func TestPageInlinesEveryWebFile(t *testing.T) {
	for dir, listed := range map[string][]string{"web/styles": pageStyles, "web/scripts": pageScripts} {
		entries, err := fs.ReadDir(web, dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		var onDisk []string
		for _, e := range entries {
			onDisk = append(onDisk, e.Name())
		}
		want := append([]string(nil), listed...)
		sort.Strings(want)
		if len(onDisk) != len(want) {
			t.Fatalf("%s holds %v, page lists %v", dir, onDisk, want)
		}
		for i := range want {
			if onDisk[i] != want[i] {
				t.Fatalf("%s holds %v, page lists %v", dir, onDisk, want)
			}
		}
	}
	for _, marker := range []string{"/*styles*/", "/*scripts*/"} {
		if bytes.Contains(indexHTML, []byte(marker)) {
			t.Fatalf("marker %s was not replaced", marker)
		}
	}
	for _, want := range []string{"'use strict';", "function draw()", "--line:", "@media (max-width: 820px)"} {
		if !bytes.Contains(indexHTML, []byte(want)) {
			t.Fatalf("assembled page is missing %q", want)
		}
	}
}
