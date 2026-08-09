package worker

import "testing"

func TestParseSearchHTML(t *testing.T) {
	html := []byte(`
<div class="search-registry-entrys-block">
  <div class="search-registry-entry-block">
    <div class="registry-entry__form">
      <span class="registry-entry__header-mid__number">
        <a href="/epz/order/notice/ea20/view/common-info.html?regNumber=0334500000125000001">№ 0334500000125000001</a>
      </span>
      <div class="registry-entry__body-value">Поставка ПО</div>
      <div class="price-block__value">1 250 000,50</div>
    </div>
  </div>
  <div class="search-registry-entry-block">
    <div class="registry-entry__form">
      <a href="/epz/order/notice/notice223/common-info.html?regNumber=3231234567890123456">3231234567890123456</a>
      <div class="registry-entry__body-value">Услуги разработки</div>
    </div>
  </div>
</div>`)
	hits := ParseSearchHTML(html)
	if len(hits) != 2 {
		t.Fatalf("want 2 hits, got %d", len(hits))
	}
	if hits[0].RegNumber != "0334500000125000001" {
		t.Fatalf("reg: %s", hits[0].RegNumber)
	}
	if hits[0].ObjectName != "Поставка ПО" {
		t.Fatalf("object: %s", hits[0].ObjectName)
	}
	if hits[0].NMCK == nil || *hits[0].NMCK < 1000000 {
		t.Fatalf("nmck: %v", hits[0].NMCK)
	}
	if hits[1].Law != "223" {
		t.Fatalf("law: %s", hits[1].Law)
	}
}
