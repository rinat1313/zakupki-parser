package archiveutil

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/nwaples/rardecode/v2"
	"golang.org/x/text/encoding/charmap"
)

// IsArchive — zip/rar (по расширению).
func IsArchive(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".zip", ".rar":
		return true
	default:
		return false
	}
}

// Extract распаковывает архив в destDir, возвращает относительные пути извлечённых файлов.
// После успешной распаковки вызывающий должен удалить архив сам.
func Extract(archivePath, destDir string) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(archivePath))
	switch ext {
	case ".zip":
		return extractZip(archivePath, destDir)
	case ".rar":
		files, err := extractRAR(archivePath, destDir)
		if err == nil {
			return files, nil
		}
		// fallback: unar / unrar в PATH
		if files2, err2 := extractViaCmd(archivePath, destDir); err2 == nil {
			return files2, nil
		}
		return nil, err
	default:
		return nil, fmt.Errorf("unsupported archive: %s", ext)
	}
}

// ExtractRecursive распаковывает архив и вложенные архивы до maxDepth (включительно с корнем).
// Возвращает абсолютные пути листовых файлов (не архивов).
func ExtractRecursive(archivePath, destDir string, maxDepth int) ([]string, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}
	first, err := Extract(archivePath, destDir)
	if err != nil {
		return nil, err
	}
	var queue []string
	for _, rel := range first {
		queue = append(queue, filepath.Join(destDir, rel))
	}
	var leaves []string
	for depth := 0; depth < maxDepth && len(queue) > 0; depth++ {
		var next []string
		for _, p := range queue {
			if IsArchive(p) {
				sub := filepath.Join(filepath.Dir(p), strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))+"_unpacked")
				_ = os.MkdirAll(sub, 0o755)
				inner, err := Extract(p, sub)
				_ = os.Remove(p)
				if err != nil {
					leaves = append(leaves, p) // оставим архив как есть — вызывающий отметит ошибку
					continue
				}
				for _, rel := range inner {
					next = append(next, filepath.Join(sub, rel))
				}
				continue
			}
			leaves = append(leaves, p)
		}
		queue = next
	}
	// всё, что осталось в queue после maxDepth — тоже листья (в т.ч. архивы)
	leaves = append(leaves, queue...)
	return leaves, nil
}

func extractZip(archivePath, destDir string) ([]string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var out []string
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rawName := decodeZipName(f)
		name := filepath.Base(rawName) // без path traversal
		if name == "" || name == "." || name == ".." {
			continue
		}
		target := filepath.Join(destDir, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return out, err
		}
		rc, err := f.Open()
		if err != nil {
			return out, err
		}
		w, err := os.Create(target)
		if err != nil {
			rc.Close()
			return out, err
		}
		_, err = io.Copy(w, rc)
		w.Close()
		rc.Close()
		if err != nil {
			return out, err
		}
		out = append(out, name)
	}
	return out, nil
}

// decodeZipName: UTF-8 флаг или CP866 (типичные архивы с ЕИС/Windows).
func decodeZipName(f *zip.File) string {
	name := f.Name
	if f.NonUTF8 {
		if dec, err := charmap.CodePage866.NewDecoder().String(name); err == nil && utf8.ValidString(dec) {
			return dec
		}
	}
	if !utf8.ValidString(name) {
		if dec, err := charmap.CodePage866.NewDecoder().String(name); err == nil {
			return dec
		}
	}
	return name
}

func extractRAR(archivePath, destDir string) ([]string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rr, err := rardecode.NewReader(f)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}

	var out []string
	for {
		hdr, err := rr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, err
		}
		if hdr.IsDir {
			continue
		}
		name := filepath.Base(hdr.Name)
		if name == "" || name == "." || name == ".." {
			continue
		}
		target := filepath.Join(destDir, name)
		w, err := os.Create(target)
		if err != nil {
			return out, err
		}
		_, err = io.Copy(w, rr)
		w.Close()
		if err != nil {
			return out, err
		}
		out = append(out, name)
	}
	return out, nil
}

func extractViaCmd(archivePath, destDir string) ([]string, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}
	before, _ := listFiles(destDir)
	var cmd *exec.Cmd
	if _, err := exec.LookPath("unar"); err == nil {
		cmd = exec.Command("unar", "-f", "-o", destDir, archivePath)
	} else if _, err := exec.LookPath("unrar"); err == nil {
		cmd = exec.Command("unrar", "x", "-o+", archivePath, destDir+string(os.PathSeparator))
	} else {
		return nil, fmt.Errorf("no rar decoder and no unar/unrar in PATH")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, string(out))
	}
	after, err := listFiles(destDir)
	if err != nil {
		return nil, err
	}
	var added []string
	set := map[string]struct{}{}
	for _, b := range before {
		set[b] = struct{}{}
	}
	for _, a := range after {
		if _, ok := set[a]; !ok {
			added = append(added, a)
		}
	}
	return added, nil
}

func listFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
