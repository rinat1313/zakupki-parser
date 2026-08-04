package detect

import "testing"

func TestLooksLike(t *testing.T) {
	html223 := `<div class="registry-entry__header">223-ФЗ</div>
<a href="/epz/order/notice/notice223/common-info.html?regNumber=32616257756">x</a>
<div class="common-text__title">Реестровый номер извещения</div>32616257756`
	if !looksLike223(html223, "32616257756") {
		t.Fatal("223")
	}
	html44 := `<div class="cardMainInfo__purchaseLink"><a>№ 0432200000826003978</a></div>`
	if !looksLike44(html44, "0432200000826003978") {
		t.Fatal("44")
	}
	if looksLike223(`сведения не найдены 32616257756 notice223`, "32616257756") {
		t.Fatal("should reject not found")
	}
}
