-- 002'deki üyelik geri alındı: users + sessions kaldırıldı.
-- 002 dosyası tarihçe için durur; uygulanmış DB'leri de temizlemek için
-- silmek yerine ileri migration ile drop edilir.
drop table if exists public.sessions;
drop table if exists public.users;
