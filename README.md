# README.md — opadm

Küçük ekipler için sunucu bakım ve yedekleme hizmeti — tek sayfalık tanıtım ve **kalıcı talep formu**. Enteksis işe alım değerlendirmesi için kurgulanan teknik örnektir; gerçek talep toplanmaz. Yalnızca kurgusal test verisi kullanılır.

- **Ne:** Form → istemci ön doğrulaması → sunucu doğrulaması → Postgres INSERT commit → **yalnızca o zaman 201** → istemci başarı bildirimi.
- **Kime:** Tam zamanlı DevOps uzmanı bulunmayan küçük yazılım ekipleri (yedekleme, izleme ve güvenlik sıkılaştırma).
- **Hangi sorun:** Yedek alınır ancak geri yükleme test edilmez; sunucu gece çöker ve sabah fark edilir; işletim sistemi ve paket güncellemeleri sahipsiz kalır.

## Teslim

| Alan | Durum |
|------|-------|
| Canlı URL | `https://opadm.vercel.app` |
| Kaynak kod | Bu repo; değerlendiricinin erişimi yeterlidir. |
| Teslim commit | `git rev-parse HEAD` — dağıtılan sürümün belirteci. |
| Kurulum | Aşağıda "Yerel kurulum ve çalıştırma" bölümünde. |
| Mimari | Go + Postgres (Supabase) + Vercel / Docker. |

> Hedeflenen aktif emek süresi 3–4 saattir. Bu projede harcanan net süre ve kararlar [AI_LOG.md](./AI_LOG.md) dosyasında belgelenmiştir.

## Mimari ve Dosya Yapısı

Go standart kütüphanesi + `pgx` sürücüsü, harici bağımlılığı olmayan saf JavaScript frontend. Bağımlılıkların içe doğru aktığı katmanlı mimari (Domain-Driven Design):

```text
main.go                              Binary girişi: ortam değişkenleri, mux, middleware
api/index.go                         Vercel Serverless girişi (rotalar main.go ile ortak)
pkg/domain/                          Request, Validate/Normalize, AllowedOrigin, Repository arayüzleri
pkg/application/                     Service: Submit, Snapshot, RunDemo, Signup, Login, Me
pkg/infrastructure/http/             HTTP adaptörü: JSON serileştirme, durum kodları, rota tanımları
pkg/infrastructure/live/             WebSocket hub: telemetri ticker ve anlık bildirim
pkg/infrastructure/postgres/         Repository somutlaması: parametrik SQL
pkg/infrastructure/ratelimit/        IP başına hız sınırlayıcı (10 istek/dk)
pkg/**/**_test.go                    Birim testleri (bellek içi sahte depo ile, DB bağımsız)
pkg/infrastructure/postgres/store_test.go  TEST_DATABASE_URL ile çalışan entegrasyon testi
web/                                 Arayüz sayfaları, sayfa mantığı ve yerel fontlar
  / → Tanıtım ve talep formu · /login/ & /signup/ → Üyelik
  /panel/ → Giriş korumalı durum ve telemetri paneli
supabase/migrations/001_create_requests.sql   requests tablosu, CHECK kısıtları ve RLS
supabase/migrations/002_auth.sql              users ve sessions tabloları, RLS
vercel.json, Dockerfile, docker-compose.yml, .env.example
```

Veri akışı: `Form → İstemci doğrulaması (UX) → JSON isteği → Sunucu doğrulaması → Postgres INSERT commit → 201 + kayıt numarası → İstemci başarı bildirimi`. Başarı bildirimi sunucudan HTTP 201 ve geçerli bir `id` değeri dönmeden kullanıcıya gösterilmez.

> Uçtan uca akış detayları için: [WORKFLOW.md](./WORKFLOW.md). Sürekli entegrasyon: [`.github/workflows/ci.yml`](./.github/workflows/ci.yml).

## Yerel Kurulum ve Çalıştırma

Docker ile uygulama ve veritabanını tek seferde başlatma:

```sh
docker compose up -d --build
# http://localhost:8080 adresinden erişilebilir
```

Tek Go ikili dosyası (binary) olarak çalıştırma:

```sh
DATABASE_URL='postgres://opadm:opadm@localhost:5432/opadm?sslmode=disable' \
  PORT=8080 go run .
```

> **Veritabanı migration notu:** `docker-compose.yml`, `supabase/migrations/*` dosyalarını Postgres konteynerinin ilk çalıştırma dizinine bağlar. Volume önceden mevcutsa yeni migration'ları manuel uygulayabilirsiniz:
> ```sh
> docker compose exec -T db psql -U opadm -d opadm < supabase/migrations/002_auth.sql
> ```

Kayıtları doğrudan veritabanından inceleme:

```sh
psql 'postgres://opadm:opadm@localhost:5432/opadm?sslmode=disable' \
  -c "SELECT id, name, email, service, created_at FROM requests;"
```

## Ortam Değişkenleri

| Değişken | Varsayılan | Açıklama |
|---|---|---|
| `DATABASE_URL` | (yok) | Postgres bağlantı adresi; zorunludur. |
| `SIMPLE_PROTOCOL` | 0 | `1`: pgx simple protocol modu (Supabase transaction pooler için zorunlu). |
| `TRUST_PROXY` | 0 | `1`: Ters vekil sunucu başlıklarını (X-Forwarded-For) kabul eder. |
| `SESSION_SECURE` | 0 | `1`: Oturum çerezine Secure bayrağı ekler (HTTPS ortamı). |
| `APP_VERSION` | dev | Çalışan kod sürümü (panelde görüntülenir). |
| `PORT` | 8080 | Dinlenecek port numarası (yalnızca yerel binary için). |

## API Sözleşmesi

`POST /api/requests` — İstek `Content-Type: application/json` başlığı taşımalıdır.

```jsonc
// İstek gövdesi
{"name":"Ad Soyad","email":"ornek@sirket.com","service":"backup","message":"Sunucu detayları..."}

// Yanıt 201 (id: kalıcı veritabanı kaydının kimliği)
{"ok":true,"id":42}

// Yanıt 400
{"ok":false,"errors":{"email":"Geçerli bir e-posta adresi yazın."}}

// Yanıt 403 (Origin uyuşmazlığı) / 415 (Geçersiz Content-Type) / 429 (Hız limiti) / 500 (Sunucu hatası)
{"ok":false,"error":"Hata açıklaması"}
```

- `GET /healthz` → `{"ok":true}` (Veritabanı bağlantısı yoksa 500 döner).
- `GET /api/status` → Giriş gerektirir (Oturum yoksa 401).
- `GET /api/live` → WebSocket akışı. Uzun ömürlü süreçte (Docker/binary) anlık telemetri iletir. Vercel ortamında 501 döner, arayüz HTTP yoklamasına geçer.
- `GET /api/demo` → Giriş gerektirir. Kalıcı kayıt oluşturmadan sunucu üzerinde 8 kontrolü test eder.
- `POST /api/auth/signup` & `login` & `logout` → Kullanıcı kaydı, parola doğrulaması ve güvenli çerez yönetimi.

## Test ve Doğrulama

```sh
go vet ./...
go test ./...
go test -race ./...

# Canlı veritabanı entegrasyon testi:
TEST_DATABASE_URL='postgres://opadm:opadm@localhost:5432/opadm?sslmode=disable' \
  go test ./pkg/infrastructure/postgres/ -run Integration -v
```

Test kapsamı:
- Form girdi sınırları ve format doğrulaması (RFC 5322 e-posta, karakter limitleri).
- Hatalı gövde, bilinmeyen alan ve 16 KB üzerindeki isteklerin reddedilmesi.
- Veritabanı kesintisinde 501/500 dönmesi ve yanıltıcı başarı verilmemesi.
- IP tabanlı hız sınırlayıcı (10 istek/dk).
- CSRF ve Origin kısıtlamaları.
- Parola hashleme (bcrypt) ve oturum yönetimi.

## Dağıtım: Supabase + Vercel

Ayrıntılı adımlar [DEPLOY.md](./DEPLOY.md) dosyasındadır:

1. **Supabase:** Proje açılır, `supabase/migrations/` dosyaları SQL Editor ile çalıştırılır. Transaction pooler bağlantı adresi (port 6543) alınır.
2. **Vercel:** Proje import edilir; `DATABASE_URL`, `SIMPLE_PROTOCOL=1`, `TRUST_PROXY=1` ve `SESSION_SECURE=1` tanımlanır.
3. **Mimarinin İşleyişi:** `vercel.json` içindeki `framework: null` yapılandırması kökteki `main.go` çakışmasını engeller. Statik dosyalar doğrudan `web/` dizininden, API istekleri ise tek Go fonksiyonundan (`api/index.go`) sunulur. Rota tanımları `pkg/infrastructure/http/routes.go` içinde ortaktır.

## Güvenlik Önlemleri

- **SQL Injection Koruması:** Tüm veritabanı sorguları parametrik yürütülür (`$1, $2`).
- **Gövde ve Zarf Sınırı:** `http.MaxBytesReader` ile 16 KB sınırı, `DisallowUnknownFields` ile tanımlanmamış alanların reddi.
- **Gizlilik:** Ham IP adresi saklanmaz; SHA-256 hash'i (`ip_hash`) tutulur.
- **Oturum Güvenliği:** Parolalar bcrypt ile saklanır. Oturum jetonları istemciye HttpOnly + SameSite=Lax çereziyle iletilir, veritabanında yalnız SHA-256 hash'i tutulur.
- **HTTP Başlıkları:** Sıkı `Content-Security-Policy` (`default-src 'self'`), `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`.
- **Veritabanı Kısıtları:** Tablolarda Row Level Security (RLS) etkindir; anonim erişim kapalıdır. Alan kuralları veritabanında `CHECK` kısıtları ile güvenceye alınmıştır.

## Bilinen Sınırlar

- **Hız Sınırlayıcı:** Süreç içi bellek üzerinde çalışır; dağıtık altyapıda Redis gerektirir.
- **WebSocket:** Vercel Serverless ortamının yapısı gereği stateful bağlantı tutulamaz; istemci HTTP üzerinden güncellenir. Tam WebSocket işlevi Docker / binary ortamında aktiftir.
- **Kişisel Veri:** Güvenlik ve KVKK gereği kayıtları toplu döndüren genel bir API uç noktası açılmamıştır.
