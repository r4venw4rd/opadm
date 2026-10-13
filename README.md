# opadm

Küçük ekipler için sunucu bakım ve yedekleme hizmeti — tek sayfalık tanıtım +
**kalıcı talep formu**. Enteksis işe alım değerlendirmesi için kurgusal ürün;
gerçek talep toplanmaz (footer'da da yazar). Yalnızca kurgusal test verisi
kullanılır.

- **Ne:** form → istemci ön-doğrulama → sunucu doğrulama → Postgres INSERT
  commit → **yalnızca o zaman 201** → istemci başarıyı gösterir.
- **Kime:** tam zamanlı DevOps'çusu olmayan küçük ekipler (yedek + izleme +
  sıkılaştırma ihtiyacı).
- **Hangi sorun:** yedek alınır ama hiç test edilmez; sunucu gece çöker, sabah
  öğrenilir; güncellemeyi takip eden olmaz.

## Teslim

| Alan | Durum |
|------|-------|
| Canlı URL | `<DAĞITIM SONRASI DOLDUR — adımlar: "Deploy" bölümü>` |
| Kaynak kod | Bu repo; değerlendiricinin erişimi yeterli (herkese açık olması şart değil). |
| Teslim commit | `git rev-parse HEAD` — dağıtılan sürümün tek kimliği. README/AI_LOG bu commit ile eşleşmelidir. |
| Kurulum | Aşağıda "Yerel kurulum" ve "Test" tam komutlarıyla. |
| Mimari notu | Bkz. aşağıda "Mimari" — Go + Postgres/Supabase + Vercel. |

> Süre hedefi 3–4 saat aktif emektir. Bu projede harcanan gerçek süre ve
> emeğin nasıl dağıldığı [AI_LOG.md](./AI_LOG.md) içinde dürüstçe yazılıdır.

## Mimari ve dosya yapısı

Go (stdlib) + Postgres (pgx), çerçevesiz vanilla frontend. Katmanlı mimari
(DDD): bağımlılık tek yönlüdür — domain kimseyi bilmez.

```
main.go                              binary girişi: env, mux, timeout, middleware
api/index.go                         Vercel fonksiyonu (tek giriş; rotalar main.go ile ortak)
internal/domain/                     Request + Validate/Normalize + AllowedOrigin + Repository/AuthStore arayüzü
internal/application/                Service: Submit / Snapshot / RunDemo / Signup / Login / Me
internal/infrastructure/http/        adaptör: JSON/başlık/HTTP kodu, iş kuralı yok
internal/infrastructure/live/        WebSocket hub: 5 sn ticker + kayıt push'u
internal/infrastructure/postgres/    Repository gerçekleşimi (parametrik SQL)
internal/infrastructure/ratelimit/   IP başına limit (10/dk) + janitor + Peek
internal/**/**_test.go               katman + uç testleri (fake repo ile, DB'siz)
internal/infrastructure/postgres/store_test.go  TEST_DATABASE_URL ile entegrasyon (yoksa skip)
web/                                 sayfalar + sayfa JS'leri + self-hosted fontlar
  / → tanıtım + açık talep formu · /login/ + /signup/ → üyelik
  /panel/ → giriş arkası canlı durum + demo
supabase/migrations/001_create_requests.sql   requests tablosu + CHECK + RLS
supabase/migrations/002_auth.sql              users + sessions + RLS
vercel.json, Dockerfile, docker-compose.yml, .env.example
```

Akış: `form → istemci ön-doğrulama (UX) → fetch JSON → sunucu doğrulama →
Postgres INSERT commit → 201 + kayıt id → istemci başarı mesajı`. Başarı
mesajı sunucudan `201` ve pozitif `id` gelmeden gösterilmez.

> **Uçtan uca akışın net incelenebilir hali:** [WORKFLOW.md](./WORKFLOW.md)
> (istek, oturum, demo, güvenlik katmanları, AI döngüsü, CI/dağıtım — her adım
dosya/fonksiyon referanslı + diyagramlı). CI: [`.github/workflows/ci.yml`](./.github/workflows/ci.yml).

## Yerel kurulum ve çalıştırma

```sh
docker compose up -d --build   # app + yerel postgres (migration ilk açılışta otomatik)
# http://localhost:8080

# ya da tek binary:
DATABASE_URL='postgres://opadm:opadm@localhost:5432/opadm?sslmode=disable' \
  PORT=8080 go run .
```

> **Migration notu:** `docker-compose.yml`, `supabase/migrations/*` dosyalarını
> Postgres konteynerinin init dizinine bağlar; init script'leri **yalnızca
> volume ilk oluştuğunda** koşar. Sonradan eklenen bir migration için elle
> uygulayın:
> ```sh
> docker compose exec -T db psql -U opadm -d opadm < supabase/migrations/002_auth.sql
> ```

Kayıtları görme (public listeleme uç noktası yok — kişisel veri koruması):
yerelde `psql`, bulutta Supabase Dashboard > Table Editor.

```sh
psql 'postgres://opadm:opadm@localhost:5432/opadm?sslmode=disable' \
  -c "SELECT id,name,email,service,created_at FROM requests;"
```

## Env değişkenleri

| Ad              | Varsayılan | Açıklama                                                   |
|-----------------|------------|------------------------------------------------------------|
| DATABASE_URL    | (yok)      | Postgres bağlantısı; yoksa program açılmaz                 |
| SIMPLE_PROTOCOL | 0          | `1`: simple protocol (Supabase pooler transaction mode'da şart) |
| TRUST_PROXY     | 0          | `1` ise proxy başlıklarına güven (Vercel/Cloudflare Tunnel arkası) |
| SESSION_SECURE  | 0          | `1` çereze Secure ekler (HTTPS; Vercel'de 1, yerelde 0)    |
| APP_VERSION     | dev        | Canlıdaki kod kimliği (demo/sürüm kanıtında görünür)       |
| PORT            | 8080       | Dinlenecek port (yalnızca binary; Vercel'de geçersiz)      |

`.env.example` şablondur; gerçek değerler repoya girmez (`gitignore`) ve
Vercel Dashboard > Environment Variables'ta tutulur.

## API sözleşmesi

`POST /api/requests` — istek `Content-Type: application/json` olmalı.

```jsonc
// istek
{"name":"Ad Soyad","email":"a@b.co","service":"backup|monitoring|hardening","message":"...10-1000 karakter"}
// yanıt 201  (id: kalıcı satırın kimliği → başarının kanıtı)
{"ok":true,"id":42}
// yanıt 400
{"ok":false,"errors":{"email":"Geçerli bir e-posta adresi yazın."}}
// yanıt 403 (yabancı Origin/Referer) / 415 (yanlış Content-Type) / 429 / 500
{"ok":false,"error":"...genel mesaj..."}
```

`GET /healthz` → `{"ok":true}` (DB ping'i başarısızsa 500).

`GET /api/status` → **giriş ister** (çerez yoksa 401). Kişisel veri içermez.

```jsonc
{"ok":true,"time":"...","uptime_s":1,"requests_total":1,"host":"a1b2c3",
 "in_docker":true,"go":"go1.26.0","goroutines":5,"cpu":24,"mem_alloc_mb":3,"version":"..."}
```

`GET /api/live` → WebSocket; bağlanınca ilk görüntü + 5 sn'de bir + her kayıtta
push. Yabancı Origin 403. **Yalnızca uzun ömürlü süreçte (binary/Docker);**
Vercel fonksiyonu 501 döner, istemci `fetch`'e düşer.

`GET /api/demo` → **giriş ister**. Özellikleri canlı dener; DB'ye kalıcı yazmaz,
kota yakmaz (probe temizlenir). İki katman döner:

- **`summary`** — canlı servis metrikleri: `uptime`, `requests_total`,
  `last_request_at`, `write_latency_ms`, `db_latency_us`, `submissions_ok`,
  `submissions_failed`, `error_rate_pct`.
- **`checks`** — 8 kontrol; her biri `name` + `why` (müşteriye "ne işe yarar") +
  `group` + `pass` + `detail` (mühendise teknik kanıt). Gruplar: **Ürün**
  (doğrulama, kalıcı kayıt) ve **Mühendislik** (veritabanı+şema, CSRF, hız
  limiti, bcrypt, oturum, sürüm).

`POST /api/auth/signup` `{email,password}` → 201 (kayıtlıysa 409) ·
`POST /api/auth/login` → 200 + HttpOnly çerez (yanlışsa 401) ·
`GET /api/auth/me` → `{ok:true,email}` ya da 401 ·
`POST /api/auth/logout` → 200 + çerez silinir.

Kurallar: name 2-80, email format + ≤254 (küçük harfe normalize), service
allowlist, message 10-1000. Limit: 16 KB body; bilinmeyen alan reddedilir.
DB tarafında aynı kurallar CHECK constraint olarak da vardır (ikinci savunma).

## Test ve doğrulama

```sh
go vet ./...
go test ./...                 # fake repo ile DB'siz: tüm katmanlar
go test -race ./...

# Gerçek Postgres'e karşı entegrasyon (migration uygulanmış olmalı):
TEST_DATABASE_URL='postgres://opadm:opadm@localhost:5432/opadm?sslmode=disable' \
  go test ./internal/infrastructure/postgres/ -run Integration -v
```

Kapsam (DB'siz): geçerli kayıt (201 + `id` + kayıt var + e-posta normalize),
geçersiz e-posta/diğer alanlar (400 + kayıt yok), bilinmeyen alan / yanlış
Content-Type (415) / 16 KB üstü body, DB arızası (500, başarı yok), hız limiti
(429), yabancı Origin (403), TRUST_PROXY kapalı/açık, limiter temizliği,
`/healthz`, `/api/status` (girişsiz 401, girişli 0→1), `/api/demo` (8 kontrol +
`summary` geçer, iz bırakmaz), üyelik akışı (201/409/401/me/logout), WebSocket push +
Origin kapısı. Domain ve application katmanlarının ayrı testleri vardır.

## Deploy: Supabase + Vercel

Tam adım adım runbook: **[DEPLOY.md](./DEPLOY.md)**.

```sh
# 1. Supabase: app.supabase.com > New project (Free) > SQL Editor'da
#    supabase/migrations/001_create_requests.sql ve 002_auth.sql içeriğini çalıştır.
#    Project Settings > Database > Connection string (pooler, 6543) + şifreyi al.
# 2. Vercel: import repo > Environment Variables:
#    DATABASE_URL=<pooler string>, SIMPLE_PROTOCOL=1, TRUST_PROXY=1, SESSION_SECURE=1
#    Deploy. Statik site web/'den, API tek fonksiyondan (api/index.go) gelir.
# 3. Canlı doğrulama:
#    a. Canlı URL'den formu gönder → başarı mesajı + "Kayıt no: #N"
#    b. Supabase Table Editor'de N numaralı kaydı gör
#    c. Geçersiz e-postayla gönder → 400 ve kayıt yok
```

`vercel.json`: `"framework": null` (kökteki `main.go` yüzünden Vercel'in Go
framework preset'ini kapatır) + `outputDirectory: web` + `/api/*` ve `/healthz`
için `/api/index`'e `rewrites`. Böylece **statik `web/` Vercel'den**, tüm API ise
**tek Go fonksiyonundan** (`api/index.go`) sunulur. Fonksiyon rotaları
`App.RegisterRoutes` ile `main.go` ile **aynı haritadan** çözer; iki ortamda rota
farkı olmaz. (`api/<yol>/handler.go` dosya→rota eşlemesine güvenmek yerine bu
kanıtlanmış desen seçildi.)

Alternatif (yerel/ev sunucusu): `docker compose up -d --build` — app + postgres
birlikte ayağa kalkar. `opadm.service` systemd örneği de duruyor.

## Güvenlik notları

- Parametrik sorgu (`$1..`, SQL injection yok); ham IP saklanmaz, SHA-256 özeti
  (`ip_hash`) tutulur.
- `http.MaxBytesReader` (16 KB), Content-Type zorunluluğu, `DisallowUnknownFields`.
- Origin/Referer Host eşleşmesi (yabancı kaynak 403).
- Başarı yalnızca INSERT commit sonrası 201; DB hatasında 500 + genel mesaj,
  iç detay sızdırılmaz.
- Başlıklar: CSP (same-origin), X-Content-Type-Options, X-Frame-Options DENY,
  Referrer-Policy. Sunucu timeout'ları + panic recovery middleware.
- Hız limiti: IP başına 10 istek/dakika, instance-içi in-memory (Vercel'de
  instance başına); üyelik/giriş de aynı limitin arkasında (bcrypt ~100ms fren).
- Üyelik: parola bcrypt özeti; oturum jetonunun DB'de yalnız SHA-256 özeti
  (7 gün); HttpOnly + SameSite=Lax çerez; giriş hatasında oracle yok.
- Supabase'te RLS açık, anon politikası yok: yazma yalnızca service_role
  anahtarlı backend'den.
- Secret'lar repoda yok; DOM'a hep `textContent` (HTML enjeksiyonu yok).

## Değerlendirme ölçütlerine göre kanıt

- **Ürün (25):** çalışan form + kalıcı kayıt + mobil/masaüstü uyumlu sayfa.
  Kanıt: bu oturumda gerçek Postgres'e karşı 201 + satır + 400 + kayıt yok
  (bkz. AI_LOG "Doğrulama").
- **Mühendislik (20):** parametrik sorgu, sunucu doğrulama, sınırlar, güvenli
  oturum, CSP, RLS, hız limiti. Kanıt: `internal/` katmanları + migration dosyaları.
- **AI ile üretim (20):** araç, görev dağılımı, kabul/red ve doğrulama adımları
  [AI_LOG.md](./AI_LOG.md)'de; komut + gerçek çıktılarla.
- **Kullanılabilirlik (10):** görünür odak halkası, etiketler, alan-bazlı
  hata + hata özeti (klavye/ekran okuyucu), `prefers-reduced-motion`, ≥44px
  hedefler, açık/koyu tema, WCAG kontrast düzeltmeleri.
- **Test/teslim (10):** `go vet` + `go test -race` + entegrasyon testi + E2E
  curl; kurulum adımları yukarıda; bilinen eksikler aşağıda.
- **Yazılı problem çözme (10):** risk/tercih/sonuç AI_LOG'da; teşhis→sınırlama→
  düzeltme→iletişim izlenebilir.

## Bilinen eksikler

- CSRF token yok (Origin kontrolü var; klasik CSRF kesilir, başlıksız `curl`
  kırılmaz).
- Hız limiti instance-içi: Vercel'de instance başına çalışır; dağıtık limit
  Redis ister (bilerek eklenmedi).
- `TRUST_PROXY=1` sahte başlığa açıktır; yalnızca başlıkları ezen proxy
  (Vercel/Cloudflare) arkasında açılır.
- E-posta onayı yok (SMTP altyapısı ister), admin panel yok (Supabase Table
  Editor kullanılır), IP hash tuzsuz.
- Vercel'de soğuk başlatma gecikmesi olabilir; uptime sayacı instance ömrü
  kadardır ve WebSocket Vercel'de çalışmaz (501 + fetch fallback).
- `MaxOpenConns(5)`: küçük havuz, pooler arkasında bilinçli tercih.
- Docker volume ilk init'ten sonra migration'lar otomatik uygulanmaz (yukarıda
  elle komut var).

## Kaynaklar

- Kütüphane: `github.com/jackc/pgx/v5` (Postgres sürücüsü),
  `github.com/coder/websocket` (canlı akış), `golang.org/x/crypto/bcrypt`.
- Şablon/hazır tema kullanılmadı; HTML/CSS/JS el yazımı.
- Fontlar self-hosted (`web/fonts/`: Inter + JetBrains Mono, OFL lisanslı);
  Google Fonts/Tailwind CDN yok — CSP `script-src 'self'` ile uyumlu. Tek
  istisna, tema kırpışmasını önleyen minik inline script'tir (sha256 allowlist,
  `main.go`'da notlu; script değişirse hash yeniden hesaplanmalı).

## AI ile nasıl çalışıldı

Ayrıntı ve karar gerekçeleri: [AI_LOG.md](./AI_LOG.md). Özet: kod büyük ölçüde
AI kodlama aracısıyla üretildi; ürün/mimari/deploy kararları kullanıcıdan
(opadm adı, Docker, Vercel + Supabase'e geçiş dahil). Her değişiklik `go vet` +
`go test -race` + gerçek Postgres'e karşı E2E ile doğrulandı.
