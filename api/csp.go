package main

import (
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Astro emits inline island bootstraps. Hash the exact immutable build bytes at startup.
func staticCSP(dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		return "", err
	}
	scripts := regexp.MustCompile(`(?s)<script\b[^>]*>(.*?)</script>`)
	hashes := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range scripts.FindAllSubmatch(body, -1) {
			if len(match[1]) > 0 {
				sum := sha256.Sum256(match[1])
				hashes["'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'"] = true
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	list := make([]string, 0, len(hashes))
	for hash := range hashes {
		list = append(list, hash)
	}
	sort.Strings(list)
	return "default-src 'self'; script-src 'self' " + strings.Join(list, " ") + "; style-src 'self' 'unsafe-inline'; img-src 'self' data: https://placehold.co; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'", nil
}
