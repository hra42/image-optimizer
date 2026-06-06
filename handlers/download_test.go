package handlers

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/hra42/image-optimizer/processor"
)

func TestHasMultipleSources(t *testing.T) {
	tests := []struct {
		name    string
		outputs []outFile
		want    bool
	}{
		{"empty", nil, false},
		{"single source", []outFile{{srcBase: "a"}, {srcBase: "a"}}, false},
		{"two sources", []outFile{{srcBase: "a"}, {srcBase: "b"}}, true},
		{
			// A bundle output has an empty srcBase; it must NOT be counted as a
			// distinct source, or it would force every per-file output into a
			// namespaced folder.
			name:    "bundle ignored among single source",
			outputs: []outFile{{srcBase: "a"}, {bundle: true}},
			want:    false,
		},
		{
			name:    "bundle ignored, real multi still detected",
			outputs: []outFile{{srcBase: "a"}, {bundle: true}, {srcBase: "b"}},
			want:    true,
		},
		{"only a bundle", []outFile{{bundle: true}}, false},
	}
	for _, tt := range tests {
		if got := hasMultipleSources(tt.outputs); got != tt.want {
			t.Errorf("%s: hasMultipleSources = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestSoleImageOutput locks the single-file fast path: only a lone plain image
// output is served raw; multi-output jobs, packs, and bundles still ZIP.
func TestSoleImageOutput(t *testing.T) {
	tests := []struct {
		name    string
		outputs []outFile
		want    bool
	}{
		{"empty", nil, false},
		{
			name:    "one plain image",
			outputs: []outFile{{preset: "instagram_square", format: processor.FormatJPEG, data: []byte("img")}},
			want:    true,
		},
		{
			name: "two images stay zipped",
			outputs: []outFile{
				{preset: "instagram_square", data: []byte("a")},
				{preset: "convert_webp", data: []byte("b")},
			},
			want: false,
		},
		{
			// A favicon pack is a single output but folder-shaped (many members).
			name:    "lone pack stays zipped",
			outputs: []outFile{{preset: "favicon", pack: []processor.OutputFile{{Name: "favicon.ico", Data: []byte("ico")}}}},
			want:    false,
		},
		{
			name:    "lone bundle stays zipped",
			outputs: []outFile{{preset: "linkedin_doc_square", bundle: true, pack: []processor.OutputFile{{Name: "x.pdf", Data: []byte("%PDF")}}}},
			want:    false,
		},
		{
			name:    "nil data is not served raw",
			outputs: []outFile{{preset: "instagram_square", data: nil}},
			want:    false,
		},
	}
	for _, tt := range tests {
		_, got := soleImageOutput(tt.outputs)
		if got != tt.want {
			t.Errorf("%s: soleImageOutput ok = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestWriteBundleTopLevel verifies a bundle output lands at the ZIP root under
// its member filename, never namespaced by source or preset folder.
func TestWriteBundleTopLevel(t *testing.T) {
	of := outFile{
		preset: "linkedin_doc_portrait",
		bundle: true,
		pack: []processor.OutputFile{
			{Name: "linkedin_doc_portrait.pdf", Data: []byte("%PDF-fake")},
		},
	}

	names := writeToZip(t, func(zw *zip.Writer) {
		if err := writeBundle(zw, of); err != nil {
			t.Fatalf("writeBundle: %v", err)
		}
	})

	if len(names) != 1 || names[0] != "linkedin_doc_portrait.pdf" {
		t.Errorf("bundle ZIP entries = %v, want [linkedin_doc_portrait.pdf]", names)
	}
}

// TestWritePackVsBundleNaming contrasts a pack (folder-namespaced) with a bundle
// (top-level) so the divergence is locked in.
func TestWritePackVsBundleNaming(t *testing.T) {
	pack := outFile{
		preset: "favicon",
		pack: []processor.OutputFile{
			{Name: "favicon.ico", Data: []byte("ico")},
		},
	}
	bundle := outFile{
		preset: "linkedin_doc_square",
		bundle: true,
		pack: []processor.OutputFile{
			{Name: "linkedin_doc_square.pdf", Data: []byte("%PDF-")},
		},
	}

	names := writeToZip(t, func(zw *zip.Writer) {
		if err := writePack(zw, pack, false); err != nil {
			t.Fatalf("writePack: %v", err)
		}
		if err := writeBundle(zw, bundle); err != nil {
			t.Fatalf("writeBundle: %v", err)
		}
	})

	want := map[string]bool{
		"favicon/favicon.ico":     true, // pack: under a preset folder
		"linkedin_doc_square.pdf": true, // bundle: at the root
	}
	if len(names) != len(want) {
		t.Fatalf("ZIP entries = %v, want keys %v", names, want)
	}
	for _, n := range names {
		if !want[n] {
			t.Errorf("unexpected ZIP entry %q", n)
		}
	}
}

// writeToZip runs fn against a fresh zip.Writer and returns the entry names.
func writeToZip(t *testing.T, fn func(*zip.Writer)) []string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fn(zw)
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}
