# DEPLOY.md — Supabase + Vercel

Bu, teslimi canlıya çıkarmak için takip edilecek sıradır. İki yol var: **(A)
Vercel CLI** ve **(B) Vercel panosu (Git ile)**. Supabase adımları ikisinde ortak.

Ön koşul: Supabase hesabı, Vercel hesabı, yerelde Go + `psql` (migration'lar
Supabase SQL Editor'den de çalıştırılabilir).

---

## 1) Supabase — proje + şema

1. https://app.supabase.com → **New project** (Free). Bölge ve DB şifresini seç,
   şifreyi kaydet.
2. Projede **SQL Editor**'ı aç ve sırayla çalıştır:
   - `supabase/migrations/001_create_requests.sql`
   - `supabase/migrations/002_auth.sql`

   (Ya da `psql` ile: `psql "<DATABASE_URL>" -f supabase/migrations/001_create_requests.sql`
   ve ardından `002_auth.sql`.) Tablolar `anon` rolüne kapalı (RLS + politika yok);
   yazma yalnızca backend'in service bağlantısından yapılır.
3. **Project Settings → Database → Connection string → Pooler** (Transaction,
   port **6543**) satırını kopyala. Şifreyi içine yaz:
   ```
   postgres://postgres.<ref>:<SIFRE>@aws-0-<bolge>.pooler.supabase.com:6543/postgres?sslmode=require
   ```

## 2) Vercel — ortam değişkenleri

Vercel projesinde **Settings → Environment Variables** (Production + Preview):

| Ad | Değer |
|----|-------|
| `DATABASE_URL` | Supabase **pooler** bağlantı dizesi (yukarıdaki) |
| `SIMPLE_PROTOCOL` | `1` (pooler transaction mode için şart) |
| `TRUST_PROXY` | `1` (Vercel başlıkları ezer; gerçek istemci IP'si için) |
| `SESSION_SECURE` | `1` (HTTPS'te çerez `Secure` olsun) |

## 3) Dağıtım

**A) Vercel CLI (bu repoda):**
```sh
vercel login          # etkileşimli; tarayıcıdan onayla
vercel link           # bu klasörü bir Vercel projesine bağla (yoksa oluştur)
vercel --prod         # canlıya dağıt
```
> `vercel login` tarayıcı açar (interaktif). Token'ı sohbete yazma; gerekirse
> `vercel --token <TOKEN>` kullan ama token'ı repoya koyma.

**B) Vercel panosu (Git):**
Repo'yu GitHub'a push et → Vercel → **Add New → Project → Import** → env
değişkenlerini gir → **Deploy**.

## 4) Canlı doğrulama

```sh
BASE=https://<senin-projen>.vercel.app

curl -s $BASE/healthz                      # {"ok":true}

curl -i -X POST $BASE/api/requests -H 'Content-Type: application/json' \
  -d '{"name":"Canli Test","email":"canli@ornek.com","service":"backup","message":"Canli kalicilik dogrulamasi icin yeterli mesaj."}'
# → 201 {"ok":true,"id":N}

curl -i -X POST $BASE/api/requests -H 'Content-Type: application/json' \
  -d '{"name":"X","email":"bozuk","service":"vpn","message":"kisa"}'
# → 400 {"ok":false,"errors":{...}}  (kayıt oluşmaz)
```

5. Supabase **Table Editor → requests** içinde `canli@ornek.com` satırını gör
   (kalıcı kayıt kanıtı).
6. Tarayıcıda `$BASE` aç: form çalışır, başarıda **"Kayıt no: #N"** görünür.
   `/panel/` ve demo giriş ister; `/signup/` ile üye ol.

---

## Nasıl çalışır (rota modeli)

- **Statik site:** `vercel.json` → `outputDirectory: web` (Vercel sunar).
- **API:** `api/index.go` (tek Go fonksiyonu). `vercel.json` → `"rewrites":
  /api/* ve /healthz → /api/index`. Fonksiyon, rotaları `App.RegisterRoutes` ile
  **main.go ile aynı haritadan** çözer; dosya→rota eşlemesine güvenilmez.
- **`framework: null`:** kökteki `main.go` yüzünden Vercel'in Go framework
  preset'ini kapatır (yoksa projeyi tek sunucu gibi ele alıp bu yapıyı bozar).
- **`/api/live`:** serverless soket tutamaz → `api/index.go` net **501** döner;
  panel istemcisi `fetch`'e düşer.

## Notlar / sık karşılaşılanlar

- Pooler **transaction mode** server-side prepare kabul etmez → `SIMPLE_PROTOCOL=1`.
- Serverless'ta her fonksiyon instance'ı kendi in-memory hız limitini tutar
  (Vercel'de instance başına). Dağıtık limit için Redis gerekir (bilerek yok).
- Soğuk başlatmada ilk istek yavaş olabilir.
- Migration'ları unutma: tablolar yoksa form `500` döner (başarı göstermez).
