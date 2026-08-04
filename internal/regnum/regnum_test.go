package regnum

import "testing"

func TestCandidates(t *testing.T) {
	got := Candidates("0103200007726000019_1")
	if len(got) != 2 || got[0] != "0103200007726000019_1" || got[1] != "0103200007726000019" {
		t.Fatalf("%v", got)
	}
	got = Candidates("32616257756")
	if len(got) != 1 || got[0] != "32616257756" {
		t.Fatalf("%v", got)
	}
	got = Candidates("  №0432200000826003978_2  ")
	if len(got) != 2 || got[1] != "0432200000826003978" {
		t.Fatalf("%v", got)
	}
	// _need не является _цифры — не трогаем
	got = Candidates("6160326_need")
	if len(got) != 1 || got[0] != "6160326_need" {
		t.Fatalf("%v", got)
	}
}

func TestStripEditionSuffix(t *testing.T) {
	if StripEditionSuffix("abc_12") != "abc" {
		t.Fatal()
	}
	if HasEditionSuffix("abc") {
		t.Fatal()
	}
	if !HasEditionSuffix("abc_1") {
		t.Fatal()
	}
}
