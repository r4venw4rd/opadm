-- opadm requests tablosu. Supabase SQL Editor'de bir kez çalıştır.
-- Go tarafı aynı kuralları uygular; buradaki CHECK'ler ikinci savunmadır.

create table if not exists public.requests (
  id bigint generated always as identity primary key,
  name text not null check (char_length(name) between 2 and 80),
  email text not null check (char_length(email) <= 254 and email like '%@%.%'),
  service text not null check (service in ('backup', 'monitoring', 'hardening')),
  message text not null check (char_length(message) between 10 and 1000),
  ip_hash text,
  created_at timestamptz not null default now()
);

-- Yazma yalnızca service_role anahtarıyla (Go backend); anon key'e izin yok.
alter table public.requests enable row level security;
