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
  edince uyguladı. Ayrıca **DDD + canlı demo** ("mimari ddd'den aşağısı kurtarmaz",
  "live'daki app özelliklerimiz var desin, demo liveda olacak") ve **üyelik**
  ("tanıtım sitesinde üye ol, giriş yap; girişten sonra fonkları test etsinler").
- **AI yazdı:** `main.go`, `api/*/handler.go`, `internal/{domain,application,infrastructure/...}`,
  testler, `web/{index.html,app.js,style.css}`, migration'lar, Dockerfile/compose/systemd/vercel.json, README, bu dosya.
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
  listelemeyi yasaklıyor, PII riski; Supabase'te Table Editor var), e-posta onayı
  (SMTP altyapısı ister), MX kontrolü (DNS'e bağımlı, fail-open anlamsız olurdu).
- DDD geçişi: domain (Request/Validate/AllowedOrigin/Repository) +
  application (Submit/Snapshot/RunDemo) + infrastructure (http/postgres/ratelimit);
  handler ince adaptör. `/api/demo` + sayfada "Canlı demo" bölümü: 6 check canlı
  koşar, yazma/kota yok. `go vet` + `go test -race` yeşil (domain/app/http/pg-skip/ratelimit).
- Gerçek platform isteği **kullanıcıdan** ("login ve üye olma için ayrı
  sayfalar, durum/demo authenticated olunca görülsün"): `/login/`, `/signup/`,
  giriş arkası `/panel/` (girişsiz `/login/'e yollanır); `/api/status` ve soket
  de kapı arkasında (401); JS sayfa başına bölündü (theme/landing/auth/panel),
  ID çapraz kontrolü script'le doğrulandı; ölü `.auth-grid` CSS'i silindi.
- Demo kanıt sertleştirme **kullanıcı isteği** ("daha kanıtlı bi şey"):
  koşu kimliği + süreler, DB `version()` çıktısı, gerçek INSERT+ROLLBACK
  (önce/sonra sayaç deltasıyla), probe anahtarında 1 kota yakıp delta gösterme.
  `TestDemo` run_id + check kimlikleri + sayaç oynamaması kilitler.
- Demo "gerçek ihtiyaç" turu **kullanıcı isteği** ("daha gerçekçi, gerçekten
  ihtiyaç duyulabilecek testler"): kayıt turu (yaz→oku→sil, sayaç deltasıyla),
  şema (3 tablo), havuz sayıları, DB boyutu, saat sapması, gerçek bcrypt turu
  (~100ms), oturum yansıması, sürüm (APP_VERSION). 10 check; `TestDemo` kimlik
  listesini kilitler.
- Dümen-hissi düzeltmesi (önceki tur): probe `Release` ile silinir, ping µs
  ("0ms" yuvarlağı kalktı); iz bırakmama test kilitli.
- Polling yerine soket **kullanıcı önerisi** ("polling ile yaptığın şeyleri
  sokete ekle"): `GET /api/live` WebSocket (hub + 5 sn ticker + kayıt push'u),
  sayfa sokete bağlanır, olmazsa fetch'e düşer (3 deneme cap'i). Vercel
  serverless soket tutamaz → 501 + fetch fallback (README'de açık). Test:
  gerçek istemcili `TestPush` (ilk mesaj + push + takılmama) + Origin 403.
  Uçtan uca soket kanıtı Docker'da kullanıcıya kaldı (yerelde DB yok).
- Üyelik: users+sessions migration, bcrypt, SHA-256 jeton özeti, HttpOnly çerez,
  giriş arkası demo (401 kapısı). Testler: TestAuth (application) + TestAuthFlow
  (HTTP: 201/409/400/401/me/logout/demo-kapısı).
- Frontend toparlama (kullanıcı fiziksel kısmı güncellemişti, AI mismatch
  avladı): Tailwind CDN silindi (CSP `script-src 'self'` ile bloklanıyordu,
  class da kullanılmıyordu); Google Fonts self-host edildi (`web/fonts/`,
  CSP katı kaldı); inline tema script'i sha256 allowlist ile yaşatıldı;
  demo buton kapısı `loggedIn` bayrağına bağlandı; postAuth'a timeout;
  light-theme yeşil metinler #15803d (4.79:1), skip/nav-cta/btn-ghost renkleri
  temaya bağlandı; hero-meta wrap; "%100" iddiası "Her kurulumda" oldu.
  Kontrastlar hesapla, hash sayfadan yeniden türetilerek doğrulandı.
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

- Kodun tamama yakını bu oturumda AI tarafından üretildi. İstisna (dürüstlük):
  kullanıcı başka bir uçta frontend'e tema düğmesi (`initTheme`, localStorage)
  + form hata helper'ları (`showFormErrors`/`hideFormErrors`) ekledi; bu satırlar
  `d557d5e` commit'ine `git add -A` ile karıştı. Kalan 2 satırlık app.js diff'i
  commitlenmedi (sahibinin bitmemiş işi olabilir).
- Kullanıcı kararları yalnızca yukarıda listelenenlerdir; söylenmemiş düşünce
  kullanıcıya atfedilmedi.
- Test çıktıları gerçektir, uydurma sonuç yok. Canlı Supabase/Vercel doğrulaması
  yapılmadı (hesap/erişim yok); adımlar kullanıcının çalıştıracağı komutlar
  olarak README'de. Birim testler fake store ile DB'siz yeşil; entegrasyon
  testi TEST_DATABASE_URL ile kullanıcıda koşacak şekilde kapılı.
