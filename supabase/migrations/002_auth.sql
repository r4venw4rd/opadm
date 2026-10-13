-- opadm üyelik: users + sessions. 001'den sonra çalıştır.
-- Parola yalnız bcrypt özeti olarak durur; açık parola asla saklanmaz.

create table if not exists public.users (
  id bigint generated always as identity primary key,
  email text not null,
  password_hash text not null,
  created_at timestamptz not null default now()
);
create unique index if not exists users_email_unique on public.users (lower(email));

create table if not exists public.sessions (
  token_hash text primary key,
  user_id bigint not null references public.users (id) on delete cascade,
  expires_at timestamptz not null,
  created_at timestamptz not null default now()
);
create index if not exists sessions_expires_idx on public.sessions (expires_at);

alter table public.users enable row level security;
alter table public.sessions enable row level security;
-- Yazma/okuma yalnızca service_role anahtarlı backend'den; anon politikası yok.
