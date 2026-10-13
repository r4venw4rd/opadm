# opadm

Küçük ekipler için sunucu bakım ve yedekleme hizmeti — tek sayfalık tanıtım +
kalıcı talep formu. Enteksis işe alım değerlendirmesi için kurgusal ürün;
gerçek talep toplanmaz (footer'da da yazar).

- **Canlı URL:** `TODO(kullanıcı): deploy sonrası doldur` (örn. `https://opadm.vercel.app`)
- **Teslim commit ID:** `git rev-list -n 1 teslim` çıktısı esas alınır; hash ayrıca teslim raporunda bildirilir.

> **Mimari notu (dürüstlük):** Görev brief'i Go + SQLite + tek binary diyordu.
> Kullanıcı "her sistemde çalışsın, Vercel'e koyalım, DB Supabase olsun" diye
> karar verdi; mimari buna çevrildi. Sonuç: Go backend (ayni kod) + Supabase
> Postgres; Vercel'de serverless fonksiyon, yerelde/Docker'da tek binary.
> `sqlite3` ile kayıt kontrolü adımı Supabase Table Editor / SQL Editor'a dönüştü.
> Karar kullanıcıdan, sonucu AI_LOG'da.

## Mimari ve dosya yapısı

Go (stdlib) + Supabase Postgres (pgx), vanilla frontend. Mimari: DDD katmanları.
Framework yok; katmanlar arası bağımlılık tek yönlüdür (domain kimseyi bilmez).

```
main.go                              binary girişi: env, mux, timeout, middleware
api/requests|status|demo/            Vercel fonksiyonları (ince; aynı use case'ler)
internal/domain/                     Request + Validate/Normalize + AllowedOrigin + Repository arayüzü
internal/application/                Service: Submit / Snapshot / Ping / RunDemo (use case'ler)
internal/infrastructure/http/        adaptör: JSON/başlık/HTTP kodu, iş kuralı yok
internal/infrastructure/live/        WebSocket hub: 5 sn ticker + kayıt push'u
internal/infrastructure/postgres/    Repository gerçekleşimi (parametrik SQL)
internal/infrastructure/ratelimit/   IP başına limit (10/dk) + janitor + Peek
internal/{domain,application}/..._test.go katman testleri (fake repo, DB'siz)
internal/infrastructure/http/handler_test.go uç testler (fake repo ile 201/400/403/429/500/demo)
internal/infrastructure/postgres/store_test.go TEST_DATABASE_URL ile entegrasyon (yoksa skip)
web/                                 sayfalar + sayfa JS'leri (her sayfa yalnız
                                 kullandığını yükler; ortak inline tema hash'i)
  / → tanıtım + açık talep formu · /login/ + /signup/ → üyelik
  /panel/ → giriş arkası canlı durum + demo (girişsiz /login/'e yollanır)
supabase/migrations/001_create_requests.sql  tablo + CHECK + RLS (dashboard'da 1 kez)
vercel.json, Dockerfile, docker-compose.yml (app + yerel postgres), .env.example
```

Akış: form → istemci ön-doğrulama (UX) → `fetch` JSON → sunucu doğrulama →
Postgres INSERT commit → **yalnızca o zaman 201** → istemci başarıyı gösterir.

## Yerel kurulum ve çalıştırma

```sh
docker compose up -d --build   # app + yerel postgres (migration otomatik)
# http://localhost:8080
# ya da binary: DATABASE_URL=... PORT=8080 ./opadm
```

Kayıtları görme (public listeleme endpoint'i yok, kişisel veri koruması):
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
| TRUST_PROXY     | 0          | `1` ise proxy başlıklarına güven (Vercel/tunnel arkası)    |
| SESSION_SECURE  | 0          | `1` çereze Secure ekler (HTTPS; Vercel'de 1, yerelde 0)    |
| PORT            | 8080       | Dinlenecek port (yalnızca binary; Vercel'de geçersiz)      |

`.env.example` şablondur; gerçek değerler `.env`'de (repoya girmez) ve
Vercel Dashboard > Environment Variables'ta tutulur.

## Test

```sh
go vet ./...
go test ./...
go test -race ./...
# Supabase'e karşı entegrasyon (migration uygulanmış olmalı):
TEST_DATABASE_URL='postgres://...' SIMPLE_PROTOCOL=1 go test ./internal/store/ -run Integration -v
```

Birim testler fake repo ile DB'siz koşar: geçerli kayıt (201 + kayıt var +
e-posta normalize), geçersiz e-posta (400 + kayıt yok), alan uzunlukları,
geçersiz service, bilinmeyen alan / yanlış Content-Type (415) / 16 KB üstü
body, DB arızası (500, başarı yok), rate limit (429), yabancı Origin (403),
TRUST_PROXY kapalı/açık, limiter temizliği, `/healthz`, `/api/status` (girişsiz 401,
girişli sayaç 0→1),
`/api/demo` (6 check geçer, yazma/kota yok; girişli),
üyelik akışı (201/409/401/me/logout/demo-kapısı). Domain ve application katmanlarının
ayrı testleri var (Validate/Normalize/AllowedOrigin, Submit/RunDemo/Snapshot).

## Deploy: Supabase + Vercel (kullanıcı yapacak)

Kimlik bilgisi bende yok; adımlar kullanıcının çalıştıracağı şekilde:

```sh
# 1. Supabase: app.supabase.com > New project (Free) > SQL Editor'da
#    supabase/migrations/001_create_requests.sql içeriğini çalıştır.
#    Project Settings > Database > Connection string (pooler, 6543) + şifreyi al.
# 2. Vercel: import repo > Environment Variables:
#    DATABASE_URL=<pooler string>, SIMPLE_PROTOCOL=1, TRUST_PROXY=1, SESSION_SECURE=1
#    Deploy. Statik site web/'den, fonksiyonlar api/'den otomatik gelir.
# 3. Canlı doğrulama:
#    a. Canlı URL'den formu gönder → başarı mesajı + "Canlı durum" sayacı +1
#    b. Supabase Table Editor'de kaydı gör
#    c. Geçersiz e-postayla gönder → 400 ve kayıt yok
```

Alternatif (yerel/ev sunucusu): `docker compose up -d --build` — app +
postgres birlikte ayağa kalkar. `opadm.service` systemd örneği de duruyor
(`DATABASE_URL` doldurularak kullanılır).

## Üyelik (işveren demosu için)

`POST /api/auth/signup` `{email, password}` → 201 (kayıtlıysa 409) ·
`POST /api/auth/login` → 200 + HttpOnly çerez (yanlışsa 401, hangisi söylenmez) ·
`GET /api/auth/me` → `{ok:true, email}` ya da 401 ·
`POST /api/auth/logout` → 200 + çerez silinir.
Parola yalnız bcrypt özeti durur; oturum jetonunun DB'de yalnız SHA-256 özeti
durur (7 gün); ölü oturumlar saatte bir süpürülür. Demo endpoint'i giriş ister.

## API sözleşmesi

`POST /api/requests` — istek `Content-Type: application/json` olmalı.

```jsonc
// istek
{"name":"Ad Soyad","email":"a@b.co","service":"backup|monitoring|hardening","message":"...10-1000 karakter"}
// yanıt 201
{"ok":true}
// yanıt 400
{"ok":false,"errors":{"email":"Geçerli bir e-posta adresi yazın."}}
// yanıt 403 (yabancı Origin/Referer) / 429 / 500
{"ok":false,"error":"...genel mesaj..."}
```

`GET /healthz` → `{"ok":true}` (DB ping'i başarısızsa 500).

`GET /api/status` → **giriş ister** (çerez yoksa 401). Kişisel veri içermez ama
sistem bilgisidir; `/panel/` arkasından beslenir:

```jsonc
{"ok":true,"time":"2026-10-13T13:52:11Z","uptime_s":1,"requests_total":1,
 "host":"a1b2c3d4e5f6","in_docker":true,"go":"go1.26.0","goroutines":5,"cpu":24,"mem_alloc_mb":3}
```

`GET /api/live` → WebSocket. Bağlanınca ilk görüntü, sonra 5 sn'de bir +
her kayıtta itilir (aynı JSON). Yabancı Origin 403. **Yalnızca uzun ömürlü
süreçte (binary/Docker);** Vercel fonksiyonu 501 döner, sayfa fetch'e düşer
(3 başarısız denemeden sonra soketi bırakır, "Yenile" çalışır).

`GET /api/demo` → **giriş ister** (çerez yoksa 401). Özellikleri canlı dener;
DB'ye yazmaz, kota yakmaz. Sayfadaki "Canlı demo" bölümü buradan beslenir:

```jsonc
{"ok":true,"checks":[
  {"id":"validation","name":"Doğrulama","pass":true,"detail":"bozuk örnek 4 hata, sağlam örnek 0 hata"},
  // + normalize, csrf, ratelimit, database, limiter-state
]}
```

Kurallar: name 2-80, email format + ≤254 (küçük harfe normalize), service
allowlist, message 10-1000. Limit: 16 KB body; bilinmeyen alan reddedilir.
DB tarafında aynı kurallar CHECK constraint olarak da var (ikinci savunma).

## Güvenlik notları

- Parametrik sorgu (`$1..`, SQL injection yok); ham IP saklanmaz, SHA-256 özeti (`ip_hash`) tutulur.
- `http.MaxBytesReader` (16 KB), Content-Type zorunluluğu, `DisallowUnknownFields`.
- Origin/Referer Host eşleşmesi (yabancı kaynak 403).
- Başarı yalnızca INSERT commit sonrası 201; DB hatasında 500 + genel mesaj, iç detay sızdırılmaz.
- Başlıklar: CSP (same-origin), X-Content-Type-Options, X-Frame-Options DENY, Referrer-Policy.
- Sunucu timeout'ları (binary'de); panic recovery middleware.
- Rate limit: IP başına 10 istek/dakika, instance-içi in-memory (Vercel'de instance başına). Üyelik/giriş de aynı limitin arkasında (kaba kuvvet freni; bcrypt zaten ~100ms).
- Üyelik: parola bcrypt özeti, jetonun DB'de SHA-256 özeti, HttpOnly + SameSite=Lax çerez (Secure Vercel'de), 7 gün ömür, giriş hatasında oracle yok.
- Supabase'te RLS açık, anon politikası yok: yazma yalnızca service_role anahtarlı backend'den.
- Secret'lar repoda yok (`.env` gitignore'da); kayıt listeleme endpoint'i yok; DOM'a hep `textContent`.
- Pooler transaction mode + simple protocol: prepare'sız sorgu, havuz dostu.

## Bilinen eksikler

- CSRF token yok (Origin kontrolü var; klasik CSRF kesilir, başlıksız curl kırılmaz).
- Rate limit instance-içi: Vercel'de instance başına çalışır; dağıtık limit Redis ister (bilerek eklenmedi).
- TRUST_PROXY=1 sahte başlığa açıktır; yalnızca başlıkları ezen proxy (Vercel/Cloudflare) arkasında açılır.
- E-posta onayı yok (SMTP altyapısı ister), admin panel yok (Supabase Table Editor kullanılır), IP hash tuzsuz.
- Vercel'de soğuk başlatma gecikmesi olabilir; uptime sayacı instance ömrü kadardır.
- `MaxOpenConns(5)`: küçük havuz, pooler arkasında bilinçli tercih.

## Varsayımlar

- Değerlendirme rehberi okundu. Mimari değişikliği kullanıcı kararıdır; brief'in
  Go+SQLite şartından sapma bilinçlidir ve burada + AI_LOG'da açık yazılmıştır.
- "Ortalama 2 iş günü yanıt" somut, doğrulanamaz vaat içermeyen bir taahhüt olarak yazıldı.
- Süre hedefi 3-4 saat aktif emek; gerçek süre AI_LOG'da tahmini işaretli.

## Kaynaklar

- Kütüphane: `github.com/jackc/pgx/v5` (Postgres sürücüsü). Eski `modernc.org/sqlite` kaldırıldı.
- Şablon/hazır tema kullanılmadı; HTML/CSS/JS el yazımı.
- Fontlar self-hosted (`web/fonts/`: Inter + JetBrains Mono, OFL lisanslı);
  Google Fonts/Tailwind CDN yok — CSP `script-src 'self'` + inline tema hash'i ile uyumlu.
  Tek istisna inline tema script'idir (sha256 allowlist, `main.go`'da notlu).

## AI ile nasıl çalışıldı (özet)

Ayrıntı: [AI_LOG.md](./AI_LOG.md). Özet: kod AI (OpenCode/Muse Spark) tarafından
yazıldı, tüm mimari/deploy kararları kullanıcıdan (opadm adı, Docker, Vercel +
Supabase'e geçiş dahil); `go vet` + `go test -race` ile doğrulandı, canlı
Supabase/Vercel doğrulaması kullanıcıda.
