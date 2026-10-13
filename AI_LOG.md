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
  deploy yolu (hepsi görev brief'inden); ayrıca sonraki oturumlarda açıkça istenen
  **Docker ile deploy kararı** ("dockerize etsene"), **"Canlı durum" demosu**
  ("docker üzerinden reel demo, sistem bilgileri çeken demomsu şey") ve
  **Vercel + Supabase'e geçiş** ("vercel gibi bi yere deploy edicez, database
  supabase olacak"). Son karar brief'in Go+SQLite şartından bilinçli sapmadır;
  AI önce brief'e bağlı kalmayı savundu (gerekçesi raporda), kullanıcı ısrar
  edince uyguladı.
- **AI yazdı:** `main.go`, `internal/{store,handler,ratelimit}`, testler,
  `web/{index.html,app.js,style.css}`, Dockerfile/compose/systemd, README, bu dosya.
- **Kullanıcı seçimi/kararı bu oturumda ayrıca istenmedi** (brief "sormadan makul
  varsay" diyordu); doldurulması gereken alanlar README'de `TODO(kullanıcı)` olarak
  açık bırakıldı (canlı URL, aktif emek süresi).

## Önemli yönlendirmeler (özet)

1. Görev brief'i (Türkçe, 11 bölüm) — tüm gereksinimler.
2. "projenin adı opadm btw" — modül/repo adı.
3. "sayfada nöbetçiops olarak kalmasın, replace nöbetçiops with opadm" — ürün adı da opadm.
4. "dockerize etsene" + "canlı durum demosu" + "Vercel + Supabase" — kullanıcı kararları (yukarıda).

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
  root-owned gelir, yoksa nonroot DB'yi açamaz). İkinci denemede run stage'deki
  `RUN` distroless'ta shell olmadığı için patladı (`/bin/sh` yok) → `mkdir/chown`
  builder stage'e taşındı, `--chown=65532:65532` ile kopyalandı. İmaj build,
  compose up, `201` kaydı ve restart sonrası kalıcılık kullanıcı tarafından
  doğrulandı (yeşil; volume'da `id=1` kaydı görüldü).
- Kullanıcı "eksikleri de çöz" dedi. Çözülenler: Origin/Referer same-origin
  kontrolü (403), limiter boş-girdi temizliği + 5 dk janitor, TRUST_PROXY env
  (varsayılan kapalı). Çözülmeyenler (gerekçeli): admin panel (brief kamuya açık
  listelemeyi yasaklıyor, PII riski), e-posta onayı (SMTP altyapısı ister),
  MX kontrolü (DNS'e bağımlı, fail-open anlamsız olurdu). `go vet` + `go test
  -race` yeşil (handler 11 + ratelimit 2 test).
- "Canlı durum" demosu (`GET /api/status` + sayfada sayaç bölümü) **kullanıcı
  fikri** ("docker üzerinden reel demo, sistem bilgileri çeken demomsu şey").
  Kişisel veri dönmez (uptime, saat, toplam kayıt); form başarısı sayacı
  artırır. curl ile doğrulandı: `requests_total` 0→1, bölüm HTML'de mevcut.
- Kullanıcı "status canlı docker sistemden veri çeksin" dedi: `host` (Docker'da
  container ID), `in_docker` (`/.dockerenv`), `go`, `goroutines`, `cpu`,
  `mem_alloc_mb` eklendi (hepsi stdlib `os`/`runtime`). Yerel curl çıktısı:
  `in_docker:false` (container dışı), `host` makine adı. Container-içi
  (`in_docker:true`) kanıtı kullanıcıya kaldı.
- Başka hata bulunamadı. Test senaryoları ilk çalışmada geçti; "düzelttim" hikayesi
  uydurulmadı. DB-hatası (500, başarı yok) ve rate limit (429) davranışları test +
  kod incelemesiyle doğrulandı.

## Bilinen riskler/eksikler

README'deki "Bilinen eksikler" bölümüyle aynıdır (güncel): CSRF token yerine
Origin kontrolü, instance-içi limit (Vercel'de instance başına), TRUST_PROXY,
e-posta onayı yok, admin panel yok (Table Editor), tuzsuz IP hash, küçük havuz,
soğuk başlatma. Ek risk: brief Go+SQLite istiyordu, teslim Supabase Postgres'li;
değerlendirici bunu sapma sayabilir — karar ve gerekçe kullanıcıya aittir,
burada açık yazılmıştır.

## Süre

- Oturum süresi ölçülemedi; git commit zaman damgalarına göre **tahmini ~30-45 dk
  (tahmini; kullanıcı teyit etmeli)**.
- `TODO(kullanıcı): gerçek aktif emek süresi` — kullanıcı dolduracak.

## Dürüstlük notu

- Kodun tamamı bu oturumda AI tarafından üretildi; kullanıcı kod yazmadı/değiştirmedi.
- Kullanıcı kararları yalnızca yukarıda listelenenlerdir; söylenmemiş düşünce
  kullanıcıya atfedilmedi.
- Test çıktıları gerçektir, uydurma sonuç yok. Canlı Supabase/Vercel doğrulaması
  yapılmadı (hesap/erişim yok); adımlar kullanıcının çalıştıracağı komutlar
  olarak README'de. Birim testler fake store ile DB'siz yeşil; entegrasyon
  testi TEST_DATABASE_URL ile kullanıcıda koşacak şekilde kapılı.
