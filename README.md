# opadm

Küçük ekipler için sunucu bakım ve yedekleme hizmeti — tek sayfalık tanıtım +
kalıcı talep formu. Enteksis işe alım değerlendirmesi için kurgusal ürün;
gerçek talep toplanmaz (footer'da da yazar).

- **Canlı URL:** `TODO(kullanıcı): deploy sonrası doldur` (örn. `https://opadm.ornek.site`)
- **Teslim commit ID:** `git rev-list -n 1 teslim` çıktısı esas alınır; hash ayrıca teslim raporunda bildirilir.

## Mimari ve dosya yapısı

Tek Go binary'si, gömülü vanilla frontend, SQLite kalıcılık. Framework yok.

```
main.go                        PORT/DB_PATH config, mux, timeout, middleware
internal/store/store.go        SQLite açma, şema, INSERT (parametrik), Ping
internal/handler/handler.go    POST /api/requests, GET /healthz, doğrulama
internal/ratelimit/ratelimit.go in-memory IP başına limit (10/dk)
internal/handler/handler_test.go 8 senaryo (bkz. Test)
web/index.html, app.js, style.css gömülü statik dosyalar (embed)
Dockerfile, docker-compose.yml, opadm.service  deploy seçenekleri
```

Akış: form → istemci ön-doğrulama (UX) → `fetch` JSON → sunucu doğrulama →
SQLite INSERT commit → **yalnızca o zaman 201** → istemci başarıyı gösterir.

## Yerel kurulum ve çalıştırma

```sh
go build -o opadm .
PORT=8080 DB_PATH=./data/requests.db ./opadm
# http://localhost:8080
```

Kayıtları görme (public listeleme endpoint'i yok, kişisel veri koruması):

```sh
sqlite3 ./data/requests.db "SELECT id,name,email,service,created_at FROM requests;"
```

## Env değişkenleri

| Ad          | Varsayılan         | Açıklama                                                    |
|-------------|--------------------|-------------------------------------------------------------|
| PORT        | 8080               | Dinlenecek port                                             |
| DB_PATH     | ./data/requests.db | SQLite dosya yolu                                           |
| TRUST_PROXY | 0                  | `1` ise proxy başlıklarına güven (yalnızca tunnel arkası)   |

## Test

```sh
go vet ./...
go test ./...
go test -race ./...
```

Kapsananlar: geçerli kayıt (201 + DB'de var), geçersiz e-posta (400 + kayıt yok),
alan uzunlukları (400 + alan adı), geçersiz service, bilinmeyen alan / yanlış
Content-Type (415) / 16 KB üstü body (4xx), kapalı DB (500, başarı yok),
rate limit (429), yabancı Origin (403), TRUST_PROXY kapalı/açık IP seçimi,
limiter temizliği (janitor), `/healthz` (açık DB 200, kapalı DB 500).

Elle uçtan uca:

```sh
curl -X POST localhost:8080/api/requests -H 'Content-Type: application/json' \
  -d '{"name":"Deneme","email":"a@b.co","service":"backup","message":"Yedek kurulumu istiyorum."}'
# {"ok":true} + HTTP 201, ardından sqlite3 ile kontrol
```

## Deploy (kullanıcı yapacak)

Ev sunucusu + Cloudflare Tunnel önerilir (alternatif: Fly.io/Render, kalıcı
volume şart). Kimlik bilgisi/SSH bende yok; adımlar kullanıcının çalıştıracağı
şekilde:

```sh
# 1. Sunucuda:
git clone <repo> opadm && cd opadm
docker compose up -d --build
# 2. Tunnel: cloudflared ile localhost:8080'i canlı host'a bağla
#    (örn. cloudflared tunnel --url http://localhost:8080)
# 3. Canlı doğrulama (checklist):
#    a. Canlı URL'yi aç, formu doldurup gönder → başarı mesajı
#    b. Sunucuda: sqlite3 /var/lib/docker/volumes/opadm_opadm-data/_data/requests.db "SELECT * FROM requests;"
#       (compose volume yolu değişebilir: docker volume inspect opadm_opadm-data)
#    c. docker compose restart opadm → kayıt hâlâ duruyor mu kontrol et
#    d. Geçersiz e-postayla gönder → 400 ve kayıt yok
```

Docker'sız seçenek: `opadm.service` dosyasındaki komutlarla systemd kurulumu.

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

Kurallar: name 2-80, email regex + ≤254 (küçük harfe normalize), service
allowlist, message 10-1000. Limit: 16 KB body; bilinmeyen alan reddedilir.

## Güvenlik notları

- Parametrik sorgu (SQL injection yok); ham IP saklanmaz, SHA-256 özeti (`ip_hash`) tutulur.
- `http.MaxBytesReader` (16 KB), Content-Type zorunluluğu, `DisallowUnknownFields`.
- Başarı yalnızca INSERT commit sonrası 201; DB hatasında 500 + genel mesaj, iç detay sızdırılmaz.
- Başlıklar: CSP (same-origin), X-Content-Type-Options, X-Frame-Options DENY, Referrer-Policy.
- Sunucu timeout'ları (ReadHeader/Read/Write/Idle); panic recovery middleware.
- Rate limit: IP başına 10 istek/dakika, in-memory (restart'ta sıfırlanır, tek instance).
- Kayıt listeleme endpoint'i yok; kullanıcı verisi DOM'a hep `textContent` ile basılır.

## Bilinen eksikler

- CSRF: Origin/Referer Host eşleşmesi aranır (yabancı kaynak 403). Token yok;
  tarayıcı POST'ta Origin'i her zaman gönderdiği için klasik CSRF kesilir,
  başlıksız curl trafiği kırılmaz.
- Rate limit in-memory: restart'ta sıfırlanır, çok instance'ta çalışmaz.
  Süresi dolan girdiler Allow anında + 5 dk janitor ile silinir.
- TRUST_PROXY=0 iken proxy başlıkları yoksayılır; `1` yalnızca Cloudflare
  Tunnel gibi başlıkları ezen proxy arkasında açılır (compose'da açık,
  systemd örneğinde kapalı).
- E-posta onayı yok (sadece format kontrolü; SMTP altyapısı gerektirir, bilerek eklenmedi).
- E-posta onayı yok (sadece format kontrolü), admin panel yok (kontrol `sqlite3` ile), IP hash tuzsuz SHA-256.
- SQLite `MaxOpenConns(1)`: düşük trafik için doğru, ölçeklenmez. TLS uygulamada yok, tunnel'a (Cloudflare) emanet.

## Varsayımlar

- Değerlendirme rehberi (`ai.enteksis.com.tr/degerlendirme-rehberi.md`) okundu; brief ile çelişki bulunamadı (rehber: tek commit geçerli, hata uydurulması beklenmez, görüşmede kod anlatımı + küçük değişiklik).
- Repo/modül adı `opadm`; ürün adı da sayfada `opadm` (kullanıcı kararı).
- "Ortalama 2 iş günü yanıt" somut, doğrulanabilir olmayan vaat içermeyen bir taahhüt olarak yazıldı.
- Süre hedefi 3-4 saat aktif emek; gerçek süre AI_LOG'da tahmini işaretli.

## Kaynaklar

- Kütüphane: `modernc.org/sqlite` (CGO'suz SQLite sürücüsü). Başka bağımlılık yok.
- Şablon/hazır tema kullanılmadı; HTML/CSS/JS el yazımı.

## AI ile nasıl çalışıldı (özet)

Ayrıntı: [AI_LOG.md](./AI_LOG.md). Özet: iskelet ve kod AI (OpenCode/Muse Spark)
tarafından yazıldı, stack/ürün/deploy kararları kullanıcıdan; `go vet` + `go test
-race` + curl/sqlite3 ile doğrulandı.
