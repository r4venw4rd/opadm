# AI_LOG.md — opadm

Bu dosya, projenin AI ile nasıl üretildiğini ve **nasıl doğrulandığını** kaydeder.
Amaç övünç değil, izlenebilirlik: hangi karar kimden çıktı, ne test edildi, ne
edilemedi. Uydurma hata/başarı yoktur; yapılmayan doğrulama "yapılmadı" yazar.

## Araçlar

- OpenCode kodlama aracısı (model: Muse Spark) — üretim + oturum 1.
- Freebuff/Buffy kodlama aracısı (bu oturum: "revizyon/redesign") — inceleme,
  düzeltme, belge, doğrulama.
- Araç zinciri: Go 1.27 (yerel), `go vet`, `go test -race`, `curl`, `psql`,
  Docker/Compose (kurulu), Chrome (mevcut).
- Değerlendirme rehberi
  `https://ai.enteksis.com.tr/degerlendirme-rehberi.md` ve görev özeti okundu.

---

## Oturum 1 — ilk üretim (OpenCode / Muse Spark)

### Görev dağılımı

- **Kullanıcı kararları (gerçekten verilen):** repo/ürün adı `opadm` ("sayfada
  nöbetçiops kalmasın"); Go + vanilla JS stack'i; 3 hizmet (backup/monitoring/
  hardening); **Docker ile deploy** ("dockerize etsene"); **"canlı durum" demosu**
  ("reel demo, sistem bilgileri çeken demomsu şey"); **Vercel + Supabase'e geçiş**
  ("vercel gibi bi yere deploy edicez, database supabase olacak"); **DDD + canlı
  demo** ("mimari ddd'den aşağısı kurtarmaz"); **üyelik** ("üye ol, giriş yap;
  girişten sonra fonkları test etsinler"); **polling yerine soket**; **gerçekçi
  testler**; **ayrı login/signup sayfaları**.
- **AI yazdı:** `main.go`, `api/*`, `internal/**`, testler, `web/**`, migration'lar,
  Dockerfile/compose/systemd/vercel.json, README.
- Kullanıcı ayrıca diğer uçta frontend'e tema düğmesi (`initTheme`, localStorage)
  + form hata helper'ları ekledi (dürüstlük notu; `d557d5e`'ye karıştı).

### Önemli doğrulamalar (oturum 1)

- `go vet` kullanılmayan import yakaladı → düzeltildi.
- Docker build: `golang:1.23-alpine` ↔ `go.mod 1.26` uyuşmazlığı → `1.26-alpine`;
  nonroot için `/data` sahipliği düzeltildi. Kullanıcı build + restart-kalıcılık
  doğruladı.
- `go test -race` yeşil; curl ile 201/400 ve `sqlite`/Postgres kaydı doğrulandı.

### Bilinen sınır (oturum 1)

- Canlı Supabase/Vercel dağıtımı yapılmadı (hesap/erişim yok); adımlar README'de.

---

## Oturum 2 — revizyon / yeniden dizayn (Freebuff)

### Kullanıcı isteği

"Varolan projeyi incele ve isteklerim + mülakatın gerektirdikleri düzeyinde
yeniden dizayn et." Belirsizliği gidermek için 3 soru soruldu; **kullanıcı
seçimleri:**

1. **Kapsam:** "Keep everything, polish + complete" — auth/demo/live/panel kalır.
2. **Görsel/içerik:** "Keep the opadm service + look, just refine."
3. **Deploy:** "Vercel + Supabase (current plan)."

Yani iş = mevcut mimariyi koruyup **eksikleri kapatmak, hataları düzeltmek,
teslimi tamamlamak**; özellik silmek yok.

### İncelemede bulunan somut sorunlar (ve çözümleri)

1. **`vercel.json` dağıtım belirsizliği.** Kökteki `main.go`, Vercel'in (beta)
   Go "framework preset"ini tetikleyip projeyi tek sunucu gibi ele alabilirdi;
   bu, statik `web/` + `api/` serverless akışıyla çakışır. Vercel Go runtime
   dokümanı incelendi; `"framework": null` eklendi → statik `web/` + `api/`
   fonksiyonları belirsizliksiz çalışır.
2. **Erişilebilirlik hatası.** `input:focus{outline:none}` kuralı, global
   `:focus-visible` halkasını (specificity ile) eziyordu; form alanları klavyeyle
   gezilirken görünür odak kayboluyordu. `outline:none` kaldırıldı.
3. **Başarı kanıtı zayıf.** 201 gövdesi yalnız `{"ok":true}` dönüyordu; kalıcı
   kaydın kanıtı görünmüyordu. Sunucu artık `{"ok":true,"id":N}` döner, arayüz
   "Kayıt no: #N" gösterir. Test bunu kilitler.
4. **Teslim hijyeni.** `.opencode/` ve `.freebuff/` (node_modules dahil) teslime
   girmemeli → `.gitignore` + `.dockerignore` güncellendi.
5. **Yerel DB sadece `requests` içeriyordu.** Çalışan compose DB'sinde `users`/
   `sessions` yoktu; çünkü Postgres init script'leri yalnızca volume ilk
   oluştuğunda koşar. README'ye uyarı + elle uygulama komutu eklendi.
6. **README entegrasyon test yolu yanlıştı** (`internal/store` →
   `internal/infrastructure/postgres`). Düzeltildi. README, ölçüt eşlemesi,
   teslim tablosu ve güncel API sözleşmesiyle baştan yazıldı.

### Kabul / değiştirilen / reddedilen öneriler

- **Kabul:** framework:null, odak fix, id kanıtı, ignore hijyeni, README/AI_LOG.
- **Değiştirilen:** "Keep everything" gereği hiçbir özellik (auth/demo/live/panel)
  silinmedi; sadece eksik/tamamlanmış hale getirildi.
- **Reddedilen/yapılmayan:** Canlı Vercel/Supabase dağıtımı (hesap/erişim yok →
  canlı URL hâlâ yer tutucu). Commit geçmişi yeniden yazılmadı (yıkıcı; kullanıcı
  istemedi).

### Doğrulama (komutlar + **gerçek** çıktılar)

Statik kontroller:

```sh
$ go vet ./...
# çıktı yok → temiz

$ go test -race ./...
ok  opadm/pkg/application
ok  opadm/pkg/domain
ok  opadm/pkg/infrastructure/http          6.981s
ok  opadm/pkg/infrastructure/live
ok  opadm/pkg/infrastructure/postgres
ok  opadm/pkg/infrastructure/ratelimit
```

Gerçek Postgres'e karşı uçtan uca (izole `opadm_e2e` DB, migration 001+002
uygulandı; kullanıcının çalışan DB'si/app'i elle tutulmadı). Binary, 18080'de
çalıştırıldı:

```text
healthz:           200 {"ok":true}
statik:            / 200 · /style.css 200 · /landing.js 200 · /theme.js 200 ·
                   /panel/ 200 · /login/ 200 · /signup/ 200
valid POST:        201 {"id":1,"ok":true}
DB satırı:         1|E2E Kullanici|e2e-test@ornek.com|backup   ← e-posta küçük harfe normalize
invalid POST:      400 {"errors":{name,email,service,message...},"ok":false}
DB toplam:         1 (hatalı istekte değişmedi → kayıt yok)
yanlış Content-Type: 415 · bilinmeyen alan: 400 · yabancı Origin: 403
status (çerezsiz): 401
signup:            201 · dupe signup: 409 · login: 200 · me: 200
status (çerezli):  200 {"requests_total":1,"version":"e2e-session"}
demo (çerezli):    200 {"run_id":"8c89c949","took_ms":120,"checks":10,"all_pass":true}
logout sonrası me: 401

Gerçek DB arızası (requests tablosu geçici rename):
POST → 500 {"error":"Kayıt şu anda alınamadı...","ok":false}   ← başarı YOK
tablo geri → POST → 201 {"id":7,"ok":true}
Hız limiti: kota dolunca 429 (429 sınırı gözlendi)
Entegrasyon testi: TEST_DATABASE_URL ile TestIntegration PASS
```

Temizlik: test binary durduruldu, `opadm_e2e` drop edildi, geçici dosyalar
silindi; kullanıcının `:8080`'deki çalışan uygulaması ve DB'si dokunulmadan
kaldı (`/healthz` → `{"ok":true}` ile doğrulandı).

### Dürüstlük / sınırlar

- **Canlı Vercel/Supabase dağıtımı yapılmadı** (hesap/erişim yok). `vercel.json`
  düzeltmesi Vercel dokümanına dayanır, canlı dağıtımla test edilmedi → canlı URL
  kullanıcı tarafından doldurulacak.
- E2E, **yerel/Compose Postgres 16.15**'e karşı yapıldı; Supabase pooler moduna
  karşı değil (pooler için `SIMPLE_PROTOCOL=1` kod yolu var, entegrasyon testine
  `SIMPLE_PROTOCOL` env'i geçirilebilir).
- "DB arızası" senaryosu tablo yeniden adlandırılarak kuruldu (gerçek INSERT
  hatası); ilk denemede `REVOKE INSERT` etkisizdi çünkü `opadm` tablo sahibi →
  yöntem düzeltildi, kod hatası değildi.
- `go vet`/`go test` çıktıları gerçektir; kopyala-yapıştır ile bu dosyaya
  alınmıştır.

### Bu oturumda değişen dosyalar

`vercel.json`, `web/style.css`, `web/landing.js`,
`internal/infrastructure/http/handler.go`,
`internal/infrastructure/http/handler_test.go`, `.gitignore`, `.dockerignore`,
`README.md`, `AI_LOG.md`.

---

## Oturum 3 — demo yeniden tasarımı (Freebuff)

### Kullanıcı isteği

Kullanıcı paneldeki demo çıktısını gösterip **"daha yararlı bilgiler versek?"**
dedi. Netleştirme sorusunda dört yönün **hepsini** seçti: iş/servis metrikleri,
daha derin teknik kanıt, sadeleştirme, açıklama/etiket iyileştirmesi. Demo
konumu için seçim yapmadı → giriş arkası panelde bırakıldı.

### Yapılan

- **Üst katman: iş değeri.** `/api/demo` artık `summary` döner: `uptime`,
  `requests_total`, `last_request_at`, `write_latency_ms`, `db_latency_us`,
  `submissions_ok`, `submissions_failed`, `error_rate_pct`. İlk ikisi için
  `Repository.LastRequestAt()` eklendi; `submissions_*` süreç ömrü boyunca
  `Submit` sonuçlarını sayan atomik sayaçlardır.
- **Sadeleştirme + açıklama.** 10 teknik check → **8 kontrol**, iki gruba
  ayrıldı (**Ürün** / **Mühendislik**). Her kontrol `name` + `why` + `group` +
  `detail` taşır: `why` müşteriye "ne işe yarar", `detail` mühendise kanıt.
  `schema` ve `limiter-state` check'leri kaldırıldı (şema, veritabanı
  detayına katıldı; limiter durumu gereksizdi).
- **Panel UI** (`web/panel/index.html`, `web/panel.js`, `style.css`): üstte
  servis metrikleri ızgarası, altta gruplanmış kontroller (✓/✗ + ad + why +
  mono detay). Renk tek başına anlam taşımasın diye `.sr-only` ile "Geçti/Kaldı"
  metni eklendi.
- Testler yeni şekle güncellendi (fake store'lara `LastRequestAt`; `TestDemo` ve
  `TestRunDemo` artık `summary` + 8 kontrol + `why/group` zorunlu kılar).

### Doğrulama (gerçek çıktılar)

```sh
$ go vet ./...            # temiz
$ go test -race ./...     # tüm paketler ok
$ go build ...            # ok
$ node --check web/panel.js  # ok
```

Gerçek Postgres'e karşı (yine izole `opadm_e2e`), `APP_VERSION=demo-v2`:

```text
/api/demo summary: uptime "8 sn", requests_total 0, last_request_at "—",
  write_latency_ms 10, db_latency_us 133, submissions_ok 0, error_rate_pct "0.0"
checks: 8, all_pass: true, grupos: [Ürün]×2 + [Mühendislik]×6
write-cycle detay: satır #3 yazıldı → geri okundu → silindi (sayaç 0→0)
Gerçek form POST (201) sonrası summary: requests_total 1, submissions_ok 1,
  last_request_at "2026-10-13T15:37:19Z"  → metrikler gerçekten hareket ediyor
```

Temizlik: test sunucusu durduruldu, `opadm_e2e` drop edildi, geçici dosyalar
silindi; kullanıcının `:8080` uygulaması dokunulmadı.

### Dürüstlük notu

- `/api/demo` içindeki `submissions_*` sayaçları yalnız **gerçek form**
  gönderimlerini sayar; demonun kendi `FullCycle` turu bu sayaca girmez
  (bilinçli: demo gerçek trafiği şişirmesin).
- `write_latency_ms`, o koşudaki gerçek INSERT süresidir (tek örneklem),
  uzun vadeli ortalama değildir.
- Bu oturumda da canlı Vercel/Supabase dağıtımı yapılmadı.

### Bu oturumda değişen dosyalar

`internal/domain/repository.go`, `internal/infrastructure/postgres/store.go`,
`internal/application/service.go`, `internal/application/service_test.go`,
`internal/infrastructure/http/handler.go`,
`internal/infrastructure/http/handler_test.go`, `web/panel/index.html`,
`web/panel.js`, `web/style.css`, `README.md`, `AI_LOG.md`.

---

## Oturum 4 — Vercel + Supabase'e bağlama (Freebuff)

### Kullanıcı isteği

"Şimdi bunu vercel + supabase'e bağlamamız lazım."

### Ortam tespiti

- `vercel` CLI **kurulu** (v63.1.2) ama **logged out** → dağıtım ben yapamam.
- `gh` **girişli** (github.com, r4venw4rd) — Git ile import mümkün.
- `supabase` CLI yok; `.env` yok; git remote yok.
- Gravity Index araması Supabase yerine **Tiger Cloud** önerdi; kullanıcı açıkça
  **Supabase** dediği için öneri uygulanmadı (kullanıcı tercihi önce gelir).

### Teknik düzeltme (dağıtım doğruluğu)

Sorun: Vercel'de `api/<yol>/handler.go` dosyalarının rotaya eşlenmesi
belgelenmemiş/belirsizdi (dosya yolu → rota; `handler.go`'nun `/handler`
üretmesi riski). Kanıtlanmış desen (resmî Fiber Vercel tarifi) **tek
`api/index.go` + `vercel.json` rewrite**tır.

- `api/requests|status|demo|auth/*/handler.go` **kaldırıldı**; yerine **tek**
  `api/index.go` geldi.
- Rota tanımları tek kaynağa alındı: `App.RegisterRoutes(mux)`
  (`internal/infrastructure/http/routes.go`). `main.go` ve `api/index.go` **aynı
  haritayı** kullanır → iki ortamda rota farkı olamaz.
- `vercel.json`: `framework:null` + `outputDirectory:web` + `rewrites`
  (`/healthz` ve `/api/*` → `/api/index`). Statik `web/` Vercel'den, API tek
  fonksiyondan sunulur.
- `/api/live`, serverless'ta soket tutulamadığı için `api/index.go` içinde net
  **501** döner (istemci fetch'e düşer) — önceki davranış korundu.

### Doğrulama (gerçek çıktılar)

```sh
$ go build ./...        # OK
$ go vet ./...          # temiz
$ go test -race ./...   # exit=0 (TestRoutes dahil: /api/requests,/healthz,
                        #   /api/status,/api/demo,/api/auth/*, bilinmeyen→404)
$ jq . vercel.json      # geçerli JSON
```

`TestRoutes`, main.go ile api/index.go'nun paylaştığı rota haritasını kilitler.

### Yapılamayan / engel

- **Dağıtım yapılmadı:** Vercel CLI logged out (interaktif `vercel login`
  gerekir) ve Supabase proje/bağlantı bilgisi yok. Canlı URL hâlâ yer tutucu.
- Supabase projesi + migration + env değerleri kullanıcı tarafından girilecek;
  sıra `DEPLOY.md` içinde.

### Bu oturumda değişen dosyalar

`internal/infrastructure/http/routes.go` (yeni), `api/index.go` (yeni),
`main.go`, `vercel.json`, `internal/infrastructure/http/handler_test.go`,
`README.md`, `WORKFLOW.md`, `AI_LOG.md`, `DEPLOY.md` (yeni). Silinen:
`api/requests|status|live|demo|auth/**`.

## Bilinen riskler/eksikler (güncel)

README "Bilinen eksikler" bölümüyle aynıdır: CSRF token yok (Origin kontrolü
var), hız limiti instance-içi, `TRUST_PROXY` riski, e-posta onayı/admin panel
yok, tuzsuz IP hash, Vercel'de WebSocket yok (501 + fetch fallback), soğuk
başlatma, docker volume sonrası migration'ların elle uygulanması.

## Süre

- Oturum 1: ölçülemedi; commit zaman damgalarına göre **tahmini ~30-45 dk**.
- Oturum 2 (bu): inceleme + düzeltme + doğrulama; **tahmini ~1 saat aktif emek**.
- Toplam gerçek aktif emek hedefi 3-4 saat bandındadır; kesin süre kullanıcı
  beyanıyla netleşir.
