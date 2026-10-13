# AI_LOG.md — opadm

## Araçlar

- OpenCode kodlama aracısı, model: Muse Spark (`muse-spark`).
- Go 1.27 araç zinciri, `go vet`, `go test -race`, curl, sqlite3 (yerel doğrulama).
- Değerlendirme rehberi `https://ai.enteksis.com.tr/degerlendirme-rehberi.md` okundu;
  brief ile çelişen/eklenen şart bulunamadı (tek commit geçerli, hata uydurma
  beklenmez, görüşmede anlatım + küçük değişiklik not edildi).

## Görev dağılımı

- **Kullanıcı kararları (konuşmada gerçekten verilen):** repo/proje adı `opadm`;
  ürün sayfadaki adın da NöbetçiOps değil `opadm` olması; Go + SQLite + vanilla JS
  stack'i, 3 hizmet (backup/monitoring/hardening), ev sunucusu + Cloudflare Tunnel
  deploy yolu (hepsi görev brief'inden); ayrıca sonraki oturumda açıkça istenen
  **Docker ile deploy kararı** ("dockerize etsene").
- **AI yazdı:** `main.go`, `internal/{store,handler,ratelimit}`, testler,
  `web/{index.html,app.js,style.css}`, Dockerfile/compose/systemd, README, bu dosya.
- **Kullanıcı seçimi/kararı bu oturumda ayrıca istenmedi** (brief "sormadan makul
  varsay" diyordu); doldurulması gereken alanlar README'de `TODO(kullanıcı)` olarak
  açık bırakıldı (canlı URL, aktif emek süresi).

## Önemli yönlendirmeler (özet)

1. Görev brief'i (Türkçe, 11 bölüm) — tüm gereksinimler.
2. "projenin adı opadm btw" — modül/repo adı.
3. "sayfada nöbetçiops olarak kalmasın, replace nöbetçiops with opadm" — ürün adı da opadm.

## Kabul / değiştirilen / reddedilen öneriler

- Bu oturumda AI, kullanıcıya alternatif öneri sunmadı; brief doğrudan uygulandı.
  Kabul/red hikayesi uydurmamak için: **yok**.
- `ip_hash` saklama kararı: brief "istersen hiç saklama" diyordu; tuzsuz SHA-256
  özet saklandı, gerekçesi README'de (KVKK temkinliliği + bilinen eksik notu).

## Doğrulama (komutlar + gerçek çıktılar)

```sh
$ go vet ./...
# ilk çalışmada hata: internal/handler/handler_test.go:6:2: "net/http" imported and not used
# → kullanılmayan import silindi, ikinci çalıştırma temiz.

$ go test ./...
?   opadm [no test files]
ok  opadm/internal/handler 0.004s
?   opadm/internal/ratelimit [no test files]
?   opadm/internal/store [no test files]

$ go test -race -v ./internal/handler/
--- PASS: TestValidInsert / TestInvalidEmail / TestFieldLengths (5 alt test) /
    TestInvalidService / TestBadEnvelope / TestDBFailure / TestRateLimit / TestHealthz
PASS ok opadm/internal/handler 1.041s

# Elle uçtan uca (PORT=18080, geçici DB):
$ curl -s http://localhost:18080/healthz
{"ok":true}
$ curl -s -X POST .../api/requests -d '{"name":"E2E Deneme","email":"E2E@Ornek.com",...}' -w 'HTTP:%{http_code}'
{"ok":true} / HTTP:201
$ curl -s -X POST ... (geçersiz alanlar)
{"errors":{"email":...,"message":...,"name":...,"service":...},"ok":false} / HTTP:400
$ sqlite3 ... "SELECT name,email,service,message FROM requests;"
E2E Deneme|e2e@ornek.com|monitoring|Izleme kurulumu icin deneme talebidir.
# → e-posta küçük harfe normalize edilmiş, kayıt kalıcı. Geçici DB ve binary silindi.
```

## Bulunan hatalar

- Yukarıdaki kullanılmayan import (`go vet` yakaladı, düzeltilip tekrar doğrulandı).
- Docker build hatası (kullanıcı bildirdi): `go.mod requires go >= 1.26.0` ama
  Dockerfile'da `golang:1.23-alpine` vardı → `golang:1.26-alpine` yapıldı; ayrıca
  `.dockerignore` eklendi ve `/data` dizini nonroot'a chown'landı (named volume
  root-owned gelir, yoksa nonroot DB'yi açamaz). Bu oturumda Docker daemon'a
  erişim yoktu (soket `root:docker`, şifresiz sudo yok); imaj build + compose
  doğrulaması kullanıcıya kaldı, sonucu buraya işlenmedi.
- Başka hata bulunamadı. Test senaryoları ilk çalışmada geçti; "düzelttim" hikayesi
  uydurulmadı. DB-hatası (500, başarı yok) ve rate limit (429) davranışları test +
  kod incelemesiyle doğrulandı.

## Bilinen riskler/eksikler

README'deki "Bilinen eksikler" bölümüyle aynıdır: CSRF token yok, in-memory limit,
proxy başlık güveni, e-posta doğrulaması yok, yönetim paneli yok, tuzsuz IP hash.

## Süre

- Oturum süresi ölçülemedi; git commit zaman damgalarına göre **tahmini ~30-45 dk
  (tahmini; kullanıcı teyit etmeli)**.
- `TODO(kullanıcı): gerçek aktif emek süresi` — kullanıcı dolduracak.

## Dürüstlük notu

- Kodun tamamı bu oturumda AI tarafından üretildi; kullanıcı kod yazmadı/değiştirmedi.
- Kullanıcı kararları yalnızca yukarıda listelenenlerdir; söylenmemiş düşünce
  kullanıcıya atfedilmedi.
- Test çıktıları gerçektir, uydurma sonuç yok. Canlı deploy yapılmadı (erişim yok);
  deploy adımları kullanıcının çalıştıracağı komutlar olarak README'de.
