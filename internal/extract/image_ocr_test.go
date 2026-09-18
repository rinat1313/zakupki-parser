package extract

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSupportedImageExtensions(t *testing.T) {
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".tif", ".tiff"} {
		if !SupportedExtensions[ext] {
			t.Fatalf("missing %s", ext)
		}
	}
	if SupportedExtensions[".gif"] {
		t.Fatal(".gif should stay unsupported in minimal image OCR")
	}
}

func TestImageOCRPNG(t *testing.T) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Skip("tesseract not installed")
	}
	convert, err := exec.LookPath("convert")
	if err != nil {
		convert, err = exec.LookPath("magick")
		if err != nil {
			t.Skip("ImageMagick not installed")
		}
	}

	tmp := t.TempDir()
	pngPath := filepath.Join(tmp, "sample.png")
	txtPath := filepath.Join(tmp, "sample.out.txt")
	label := "Contract delivery equipment document sample text"
	var cmd *exec.Cmd
	if filepath.Base(convert) == "magick" {
		cmd = exec.Command(convert, "-background", "white", "-fill", "black",
			"-font", "DejaVu-Sans", "-pointsize", "28", "label:"+label, pngPath)
	} else {
		cmd = exec.Command(convert, "-background", "white", "-fill", "black",
			"-font", "DejaVu-Sans", "-pointsize", "28", "label:"+label, pngPath)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		cmd = exec.Command(convert, "-background", "white", "-fill", "black",
			"-pointsize", "28", "label:"+label, pngPath)
		if out2, err2 := cmd.CombinedOutput(); err2 != nil {
			t.Skipf("cannot render sample png: %v (%s; %s)", err2, out, out2)
		}
	}

	r := ToTextOut(pngPath, txtPath)
	if r.Error != "" {
		t.Fatalf("ToTextOut: %s engine=%s", r.Error, r.Engine)
	}
	if r.Engine != "ocr" {
		t.Fatalf("engine=%s", r.Engine)
	}
	b, err := os.ReadFile(txtPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.ToLower(string(b))
	if !strings.Contains(got, "contract") && !strings.Contains(got, "document") {
		t.Fatalf("ocr body=%q", string(b))
	}
}
