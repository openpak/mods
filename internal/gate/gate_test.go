package gate

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func pack(t *testing.T, files map[string]string) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(body))
	}
	w.Close()
	return bytes.NewReader(buf.Bytes())
}

const good = `{"id":"blue-hat","name":"Blue Hat","version":"1.0","title_id":"0100000000010000","author":"someone","licence":"CC-BY-4.0","source_url":"https://example.org/blue-hat","layout":"romfs","summary":"A blue hat."}`

func TestGate(t *testing.T) {
	r := pack(t, map[string]string{"mod.json": good, "romfs/Model/hat.bfres": "x", "README.md": "hi"})
	meta, sum, err := Check(r, r.Size())
	if err != nil || meta.ID != "blue-hat" || meta.TitleID != "0100000000010000" || len(sum) != 64 {
		t.Fatalf("valid package refused: %v %+v", err, meta)
	}
	cases := map[string]map[string]string{
		"game data":      {"mod.json": good, "romfs/x.nca": "x"},
		"keys":           {"mod.json": good, "romfs/prod.keys": "x"},
		"outside layout": {"mod.json": good, "exefs/main.ips": "x"},
		"no licence":     {"mod.json": strings.Replace(good, `"licence":"CC-BY-4.0",`, "", 1), "romfs/a": "x"},
		"no source":      {"mod.json": strings.Replace(good, `https://example.org/blue-hat`, "", 1), "romfs/a": "x"},
		"no mod.json":    {"romfs/a": "x"},
		"empty":          {"mod.json": good},
		"path escape":    {"mod.json": good, "romfs/../../etc/passwd": "x"},
	}
	for name, files := range cases {
		r := pack(t, files)
		if _, _, err := Check(r, r.Size()); err == nil {
			t.Errorf("%s: accepted, should be refused", name)
		}
	}
	if _, _, err := Check(bytes.NewReader([]byte("nope")), 4); err == nil {
		t.Error("non-zip accepted")
	}
}
