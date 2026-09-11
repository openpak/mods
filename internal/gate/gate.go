// Package gate is the provenance gate (MOD-01, MOD-02): a package is a zip with a mod.json at
// its root, every file under the declared layout, a licence and a source, and nothing that
// looks like game data or keys. Manual review still follows; this only refuses the obvious.
package gate

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type Meta struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	TitleID string `json:"title_id"`
	Author  string `json:"author"`
	Licence string `json:"licence"`
	Source  string `json:"source_url"`
	Layout  string `json:"layout"`
	Summary string `json:"summary"`
}

const MaxPackageBytes = 256 << 20

var (
	// Executable and packaged game data, ticket and key material: never a cosmetic mod.
	denied  = regexp.MustCompile(`(?i)(\.(nca|nsp|nsz|xci|xcz|nso|npdm|kip|tik|cert|keys)$|(^|/)(prod|title|dev)\.keys$|(^|/)exefs/main$)`)
	slugRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,63}$`)
	titleRe = regexp.MustCompile(`^[0-9A-Fa-f]{16}$`)
)

// Check reads the whole package, returns its metadata and SHA-256, or the first reason it
// cannot be accepted. The reader must be the complete zip.
func Check(r io.ReaderAt, size int64) (*Meta, string, error) {
	if size > MaxPackageBytes {
		return nil, "", fmt.Errorf("package is %d bytes; the limit is %d", size, MaxPackageBytes)
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.NewSectionReader(r, 0, size)); err != nil {
		return nil, "", err
	}
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, "", errors.New("not a zip archive")
	}
	var meta *Meta
	var files int
	for _, f := range zr.File {
		name := strings.TrimPrefix(f.Name, "./")
		if strings.HasSuffix(name, "/") {
			continue
		}
		if strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
			return nil, "", fmt.Errorf("%s: path escapes the package", name)
		}
		if denied.MatchString(name) {
			return nil, "", fmt.Errorf("%s: game data, executable or key material is not accepted", name)
		}
		switch {
		case name == "mod.json":
			rc, err := f.Open()
			if err != nil {
				return nil, "", err
			}
			meta = &Meta{}
			err = json.NewDecoder(io.LimitReader(rc, 64<<10)).Decode(meta)
			rc.Close()
			if err != nil {
				return nil, "", fmt.Errorf("mod.json: %v", err)
			}
		case strings.HasPrefix(strings.ToUpper(name), "README"), strings.HasPrefix(strings.ToUpper(name), "LICENSE"), strings.HasPrefix(strings.ToUpper(name), "LICENCE"):
		default:
			files++
		}
	}
	if meta == nil {
		return nil, "", errors.New("mod.json is missing from the package root")
	}
	if err := meta.validate(zr); err != nil {
		return nil, "", err
	}
	if files == 0 {
		return nil, "", errors.New("the package has no files under " + meta.Layout + "/")
	}
	meta.TitleID = strings.ToUpper(meta.TitleID)
	return meta, hex.EncodeToString(h.Sum(nil)), nil
}

func (m *Meta) validate(zr *zip.Reader) error {
	switch {
	case !slugRe.MatchString(m.ID):
		return errors.New("mod.json: id must be 2-64 lower-case letters, digits, . _ or -")
	case strings.TrimSpace(m.Name) == "", strings.TrimSpace(m.Version) == "", strings.TrimSpace(m.Author) == "":
		return errors.New("mod.json: name, version and author are required")
	case !titleRe.MatchString(m.TitleID):
		return errors.New("mod.json: title_id must be the 16-hex-digit title id")
	case strings.TrimSpace(m.Licence) == "":
		return errors.New("mod.json: licence is required (an SPDX id or the licence text)")
	case !strings.HasPrefix(m.Source, "https://") && !strings.HasPrefix(m.Source, "http://"):
		return errors.New("mod.json: source_url is required and must be a URL")
	case m.Layout != "romfs" && m.Layout != "exefs":
		return errors.New(`mod.json: layout must be "romfs" or "exefs"`)
	}
	for _, f := range zr.File {
		name := strings.TrimPrefix(f.Name, "./")
		if strings.HasSuffix(name, "/") || name == "mod.json" || strings.HasPrefix(strings.ToUpper(name), "README") ||
			strings.HasPrefix(strings.ToUpper(name), "LICENSE") || strings.HasPrefix(strings.ToUpper(name), "LICENCE") {
			continue
		}
		if !strings.HasPrefix(name, m.Layout+"/") {
			return fmt.Errorf("%s: every file must be under %s/", name, m.Layout)
		}
	}
	return nil
}
