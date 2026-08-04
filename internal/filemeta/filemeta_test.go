package filemeta

import "testing"

func TestEnsureExt(t *testing.T) {
	if g := EnsureExt("ТЗ_файл", ".docx", ""); g != "ТЗ_файл.docx" {
		t.Fatalf("got %q", g)
	}
	if g := EnsureExt("a.pdf", ".docx", ".pdf"); g != "a.pdf" {
		t.Fatalf("keep existing: %q", g)
	}
	if g := EnsureExt("Запрос_КП__СЭМД-3_", "", ".pdf"); g != "Запрос_КП__СЭМД-3.pdf" {
		t.Fatalf("trim+sniff: %q", g)
	}
}

func TestDotsInNameAreNotExtensions(t *testing.T) {
	cases := []struct {
		name, sniff, want string
	}{
		{"III. ТЕХНИЧЕСКАЯ ЧАСТЬ", ".docx", "III. ТЕХНИЧЕСКАЯ ЧАСТЬ.docx"},
		{"17.07.2026_11-5432_Исх-09_Пещерская_Е.Ю._по_списку", ".pdf", "17.07.2026_11-5432_Исх-09_Пещерская_Е.Ю._по_списку.pdf"},
		{"19.06.2026_исх-10-53761_Ташимов", ".pdf", "19.06.2026_исх-10-53761_Ташимов.pdf"},
		{"ТЗ тех.поддержка подсистемы сайтов комитетов_2027", ".docx", "ТЗ тех.поддержка подсистемы сайтов комитетов_2027.docx"},
		{"Проект ТЗ БС_по позициям_(2026-2027)_27.07.2026", ".docx", "Проект ТЗ БС_по позициям_(2026-2027)_27.07.2026.docx"},
		{"1.Проект ООЗ__1_07.07.26", ".docx", "1.Проект ООЗ__1_07.07.26.docx"},
		{"1.Проект ООЗ__1_07.07_5.26", ".docx", "1.Проект ООЗ__1_07.07_5.26.docx"},
	}
	for _, tc := range cases {
		if ExtFromFilename(tc.name) != "" {
			t.Fatalf("ExtFromFilename(%q) should be empty, got %q", tc.name, ExtFromFilename(tc.name))
		}
		got := EnsureExt(tc.name, "", tc.sniff)
		if got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestExtFromIconAndAlt(t *testing.T) {
	if g := ExtFromIconSrc("/epz/static/img/icons/type/docx.svg"); g != ".docx" {
		t.Fatalf("icon: %q", g)
	}
	if g := ExtFromIconSrc("/epz/static/img/icons/type/pdf.svg"); g != ".pdf" {
		t.Fatalf("icon pdf: %q", g)
	}
	if g := ExtFromAlt("Microsoft Word Document"); g != ".docx" {
		t.Fatalf("alt: %q", g)
	}
	if g := ExtFromAlt("Adobe Acrobat Document"); g != ".pdf" {
		t.Fatalf("alt pdf: %q", g)
	}
}

func TestSniffExt(t *testing.T) {
	if g := SniffExt([]byte("%PDF-1.4")); g != ".pdf" {
		t.Fatalf("pdf: %q", g)
	}
	docx := append([]byte("PK\x03\x04"), make([]byte, 100)...)
	docx = append(docx, []byte("word/document.xml")...)
	if g := SniffExt(docx); g != ".docx" {
		t.Fatalf("docx: %q", g)
	}
	if g := SniffExt([]byte("просто текст без разметки")); g != ".txt" {
		t.Fatalf("txt: %q", g)
	}
}

func TestIsKnownExt(t *testing.T) {
	if IsKnownExt(".2026") || IsKnownExt(".26") || IsKnownExt("._по_списку") {
		t.Fatal("date-like must not be known")
	}
	if !IsKnownExt(".pdf") || !IsKnownExt("DOCX") {
		t.Fatal("pdf/docx must be known")
	}
}
