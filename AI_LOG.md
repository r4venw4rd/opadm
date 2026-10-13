# AI_LOG.md — opadm

Bu dosya, projenin AI ile nasıl üretildiğini ve **nasıl doğrulandığını** kaydeder.
Amaç övünç değil, izlenebilirlik: hangi karar kimden çıktı, ne test edildi, ne
edilemedi. Uydurma hata/başarı yoktur; yapılmayan doğrulama "yapılmadı" yazar.

## Araçlar

- OpenCode kodlama aracısı (model: Muse Spark) — ilk üretim (oturum 1).
- Freebuff / AI asistanı — mimari revizyon, hata ayıklama, dağıtım (oturum 2, 3, 4).
- Antigravity AI asistanı — "Engineering-First Minimalist" UI/UX yeniden tasarımı ve teslim hazırlığı (oturum 5).
- Araç zinciri: Go 1.27 (yerel), `go vet`, `go test -race`, `curl`, `psql`,
  Docker/Compose, GitHub CLI (`gh`), Vercel CLI, Chrome DevTools.
- Değerlendirme rehberi
  `https://ai.enteksis.com.tr/degerlendirme-rehberi.md` ve görev özeti okundu.

---

## Oturum 1 — İlk Üretim (İskelet ve Temel İş Kuralları)

### Görev dağılımı

- **Kullanıcı kararları (gerçekten verilen):**
  - Repo ve ürün adı `opadm` ("küçük ekipler için DevOps/bakım").
  - Go + vanilla JS mimarisi (şişkin SPA framework'leri yerine sıfır harici JS kütüphanesi).
  - 3 net hizmet kapsamı (`backup`, `monitoring`, `hardening`).
  - Docker ile yerel geliştirme ve ayağa kaldırma gereksinimi.
  - Canlı durum ve telemetri için WebSocket temeli.
  - Form verisinin kalıcı Postgres'e yazılması ve yalnız 201 döndüğünde başarı mesajı verilmesi.
- **AI yazdı:** `main.go`, `api/*`, `pkg/**` katmanları, birim testler, `web/**`, ilk SQL migration'ları, Dockerfile ve docker-compose.

### Önemli doğrulamalar

- `go vet` ile kullanılmayan import'lar temizlendi.
- Docker build test edildi; `golang:1.23-alpine` ↔ `go.mod 1.26` uyuşmazlığı giderildi, `/data` sahipliği non-root kullanıcıya bağlandı.
- `go test -race` ile eşzamanlılık hatası olmadığı doğrulandı.

---

## Oturum 2 — Kod Kalitesi, Güvenlik ve Mimari İyileştirme

### Kullanıcı isteği

Mevcut projeyi inceleyip teslim standartlarına ve mülakat kriterlerine uygun olarak eksiklerin giderilmesi, hataların ayıklanması.

### İncelemede bulunan somut sorunlar ve çözümleri

1. **Erişilebilirlik hatası (Focus Outline):** `input:focus { outline:none }` kuralı, global `:focus-visible` halkasını specificity ile eziyordu. Form alanlarında klavye ile gezinirken görünür odak kayboluyordu. `outline:none` kaldırıldı, görünür odak garantiye alındı.
2. **Başarı kanıtı zayıflığı:** Form gönderildiğinde sunucu yalnız `{"ok":true}` dönüyordu; kalıcı kaydın veritabanı kanıtı kullanıcıya yansımıyordu. Sunucu artık `{"ok":true, "id": N}` döner, arayüz "Kayıt no: #N" gösterir.
3. **Teslim hijyeni:** IDE/araç kirlilikleri (`.opencode/`, `.freebuff/` vb.) `.gitignore` ve `.dockerignore` dosyalarına eklendi.
4. **Güvenlik Başlıkları & CSP:** `Content-Security-Policy` sıkılaştırılarak `default-src 'self'` seviyesine çekildi. Sayfa açılışındaki tema titremesini (flash of unstyled theme) önleyen tek satırlık inline betik için SHA-256 hash'i allowlist'e alındı.

---

## Oturum 3 — Canlı Demo ve Telemetri Paneli Tasarımı

### Kullanıcı isteği

Paneldeki demo çıktısının daha anlamlı, gerçek mühendislik kanıtı sunan bir yapıya kavuşturulması.

### Yapılanlar

- **İş Metrikleri Özeti:** `/api/demo` uç noktası `summary` nesnesi döndürecek şekilde genişletildi (`uptime`, `requests_total`, `last_request_at`, `write_latency_ms`, `db_latency_us`, `submissions_ok`, `error_rate_pct`).
- **8 Mühendislik Kontrolü:** Kapsam 8 somut kontrole ayrıldı (`Ürün` ve `Mühendislik` grupları). Her kontrol `name`, `why` (ürün değeri) ve `detail` (teknik kanıt) içerecek şekilde standardize edildi.
- **Canlı Döngü Testi (FullCycle):** Demonun kalıcı çöp kayıt bırakmaması için geçici satır yazma → okuma → silme döngüsü izole transaction ile test edildi.
- **Doğrulama:** `go test -race ./...` ile `TestDemo` ve `TestRunDemo` başarıyla geçirildi.

---

## Oturum 4 — Vercel Serverless ve Supabase Dağıtım Mimarisi

### Teknik sorunlar ve çözümleri

1. **Vercel Build Hatası (`internal` → `pkg`):**
   - Vercel'in Go serverless derleyicisi her API fonksiyonunu bağımsız bir derleme kökü (`handler/api`) altında çalıştırdığından, Go'nun yerleşik `internal/` paket görünürlük kuralı derlemenin kırılmasına yol açtı (`use of internal package ... not allowed`).
   - `internal/` dizini doğrudan `pkg/` olarak yeniden yapılandırıldı ve tüm import yolları güncellendi.
2. **Rota Eşleme Standartlaştırması:**
   - Çoklu dağınık API dosyaları yerine `api/index.go` altında tek bir giriş noktası ve `pkg/infrastructure/http/routes.go` üzerinde paylaşılan tek bir `RegisterRoutes(mux)` haritası kuruldu. `main.go` (standalone) ile `api/index.go` (serverless) birebir aynı rotaları çalıştırır.
3. **Supabase Entegrasyonu:**
   - Supabase Transaction Pooler (port 6543) bağlantısı için `SIMPLE_PROTOCOL=1` desteği eklendi (`pgx` transaction pooler uyumu).
   - `001_create_requests.sql` ve `002_auth.sql` migration'ları canlı Postgres ortamına uygulandı.

---

## Oturum 5 — Engineering-First Minimalist UI/UX Yeniden Tasarımı

### Kullanıcı isteği

Arayüzün (ana sayfa, form, login/signup ve /panel/ sayfası) "Vercel, Supabase ve Linear estetiği" doğrultusunda baştan sona yeniden tasarlanması; JS mantığına ve API sözleşmelerine dokunulmadan saf görsel kalite, kontrast ve erişilebilirliğin en üst seviyeye çıkarılması.

### Alınan tasarım ve mimari kararlar

1. **Renk Paleti & Atmosfer:**
   - Arka plan derin koyu siyah/gri tonlarına çekildi (`#09090b` ve `#0d1117`).
   - Kararlılık ve güven vurgusu için canlı zümrüt yeşili (`#10b981`) ve saf beyaz tipografi kullanıldı. Kart ve bölücüler için zarif koyu gri hatlar (`#1f2937`) tercih edildi.
2. **Kurgusal "Sandbox Test Environment" Banner'ı:**
   - Projenin bir işe alım değerlendirmesi olduğunu belirten feragat metni, sayfa üstlerine yarı şeffaf sarı/turuncu çerçeveye ve parlayan durum noktasına sahip şık bir Sandbox Banner bileşeni olarak eklendi.
3. **Form & Akış UX İyileştirmeleri:**
   - Giriş alanlarına (`input`, `textarea`, `select`) odaklanma durumunda zümrüt yeşili neon parlama efekti verildi.
   - Form gönderiminde (`submit`) buton için akıcı bir döner spinner (CSS animation) tanımlandı.
   - Başarılı `201` yanıtı ve `#ID` bilgisi, terminalden dönen başarı logu gibi parlayan yeşil bir "Commit Success" kutusuna dönüştürüldü.
4. **İzleme & Panel Bileşenleri (/panel/):**
   - WebSocket metrikleri yan yana dizilen, köşesinde anlık yanıp sönen (pulse) yeşil telemetri noktası bulunan minimalist izleme kartlarına dönüştürüldü.
   - 8 mühendislik kontrolü birer terminal satırı gibi biçimlendirildi; teknik kanıt (`detail`) bilgisi tıklandığında açılan `<details>` / accordion kod bloğu içerisine yerleştirildi.
5. **Erişilebilirlik & Uyumluluk Doğrulaması:**
   - Harici hiçbir CSS/JS framework (Tailwind CDN, React vs.) projeye dahil edilmedi; CSP (`default-src 'self'`) bütünlüğü korundu.
   - Tüm interaktif öğelerin klavye gezinimi (`:focus-visible`), metin kontrast oranları (WCAG 2.1 AA) ve ekran okuyucu etiketleri (`.sr-only`) test edildi.

### Doğrulama

```sh
$ go vet ./...
# temiz

$ go test -race ./...
ok  opadm/pkg/application
ok  opadm/pkg/domain
ok  opadm/pkg/infrastructure/http
ok  opadm/pkg/infrastructure/live
ok  opadm/pkg/infrastructure/postgres
ok  opadm/pkg/infrastructure/ratelimit
```

---

## Kabul Edilen / Değiştirilen / Reddedilen AI Önerileri Özeti

| Konu | AI Önerisi | Verilen Karar / Neden |
|---|---|---|
| **Veritabanı Sağlayıcısı** | Tiger Cloud / alternatif DB sağlayıcıları | **Reddedildi.** Kullanıcı tercihi olan Supabase Postgres ve yerel Docker PG korundu. |
| **Frontend Kütüphanesi** | Tailwind CDN veya harici UI kütüphanesi yükleme | **Reddedildi.** CSP kurallarını ihlal etmemek ve harici ağ bağımlılığı yaratmamak için saf CSS mimarisi tercih edildi. |
| **Serverless Yapısı** | Vercel üzerinde çoklu dağınık fonksiyon dosyaları | **Değiştirildi.** Tek `api/index.go` ve paylaşılan `RegisterRoutes` deseni ile standalone ve serverless davranışları eşitlendi. |
| **Paket Mimarisi** | `internal/` paketlerini `api/` içine kopyalama | **Değiştirildi.** Kod tekrarı yerine Go standartlarına uygun olarak `pkg/` dizinine taşındı. |
| **Erişilebilirlik** | Standart tarayıcı outline'ını kapatma | **Düzeltildi.** Klavye kullanıcıları için `:focus-visible` ve yüksek kontrastlı neon halka oluşturuldu. |

---

## Bilinen Riskler ve Sınırlar

1. **Vercel WebSocket Sınırı:** Vercel Serverless ortamı stateless çalıştığı için `/api/live` soket bağlantılarını uzun süre açık tutamaz; sistem HTTP fallback ile çalışır. Canlı WebSocket deneyimi Docker veya bağımsız Go binary'sinde tam olarak çalışır.
2. **Kişisel Veri Koruma (KVKK):** Kayıtların toplu listelendiği halka açık bir GET uç noktası güvenlik ve gizlilik gereği bilerek açılmamıştır; kayıtlar yalnız yetkili panel veya doğrudan DB üzerinden incelenebilir.
3. **Dağıtılmış Hız Limiti:** Rate limiter yerel bellek içi (in-memory token bucket) çalışmaktadır; çoklu serverless container durumlarında limitler instance bazlı izole çalışır (prodüksiyon için Redis adaptörü gerektirir).

---

## Süre Beyanı

- **Oturum 1 (İlk Üretim & Testler):** ~45 dk
- **Oturum 2 (Mimari Revizyon & Güvenlik Başlıkları):** ~45 dk
- **Oturum 3 (Canlı Demo & Metrik Motoru):** ~30 dk
- **Oturum 4 (Vercel Serverless & Supabase Göçü):** ~35 dk
- **Oturum 5 (Engineering-First UI/UX & Erişilebilirlik):** ~45 dk
- **Toplam Aktif Emek:** **~3 saat 20 dakika** (Enteksis 3–4 saatlik hedef emek penceresine tam uyumlu).
