package domain

import "testing"

func TestNormalize(t *testing.T) {
	r := Request{Name: "  Ada  ", Email: "  ADA@Ornek.COM ", Service: "backup", Message: "  merhaba dünya  "}
	r.Normalize()
	if r.Name != "Ada" || r.Email != "ada@ornek.com" || r.Message != "merhaba dünya" {
		t.Fatalf("normalize olmadı: %+v", r)
	}
}

func TestValidate(t *testing.T) {
	good := Request{Name: "Ada", Email: "a@b.co", Service: "monitoring", Message: "yeterince uzun mesaj"}
	good.Normalize()
	if errs := good.Validate(); len(errs) != 0 {
		t.Fatalf("sağlam kayıt hata verdi: %v", errs)
	}
	bad := Request{Name: "A", Email: "bozuk", Service: "vpn", Message: "kisa"}
	bad.Normalize()
	if errs := bad.Validate(); len(errs) != 4 {
		t.Fatalf("bozuk kayıt 4 hata vermeli: %v", errs)
	}
}

func TestAllowedOrigin(t *testing.T) {
	if !AllowedOrigin("ornek.site", "https://ornek.site", "") {
		t.Fatal("öz Origin geçmeli")
	}
	if AllowedOrigin("ornek.site", "https://kotusu.ornek", "") {
		t.Fatal("yabancı Origin geçmemeli")
	}
	if !AllowedOrigin("ornek.site", "", "") {
		t.Fatal("başlıksız (curl) geçmeli")
	}
	if AllowedOrigin("ornek.site", "%zz", "") {
		t.Fatal("bozuk Origin geçmemeli")
	}
}
