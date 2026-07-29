# Phase 1 — Google OAuth Setup

Everything on the code side is done. To actually log in you need to create an OAuth 2.0 client in Google Cloud Console and drop the credentials into `.env`.

## 1. Create a Google Cloud project (skip if you already have one)

1. Open https://console.cloud.google.com/
2. Top-left project picker → **New Project** → name it e.g. `oneapply-dev` → **Create**.

## 2. Enable the Gmail API

1. In your project, go to **APIs & Services → Library** (https://console.cloud.google.com/apis/library).
2. Search for **Gmail API** → **Enable**.

(No need to enable a separate "Google+" or "People" API — the OpenID Connect userinfo endpoint we use is always on.)

## 3. Configure the OAuth consent screen

1. **APIs & Services → OAuth consent screen** (https://console.cloud.google.com/apis/credentials/consent).
2. User type: **External** → **Create**.
3. Fill in the required fields:
   - App name: `OneApply (dev)`
   - User support email: your email
   - Developer contact: your email
4. **Scopes** → **Add or remove scopes** → check:
   - `.../auth/userinfo.email`
   - `.../auth/userinfo.profile`
   - `openid`
   - `https://www.googleapis.com/auth/gmail.send`
   - `https://www.googleapis.com/auth/gmail.readonly`
5. **Test users** → **Add users** → add your own Gmail address. (While the app is in "Testing" mode only listed test users can complete the flow. That's fine for MVP.)
6. Save everything.

## 4. Create the OAuth 2.0 Client ID

1. **APIs & Services → Credentials** (https://console.cloud.google.com/apis/credentials).
2. **Create Credentials → OAuth client ID**.
3. Application type: **Web application**.
4. Name: `oneapply-local`.
5. **Authorized redirect URIs** → **Add URI**:
   ```
   http://localhost:8080/api/auth/google/callback
   ```
6. **Create**. Copy the **Client ID** and **Client Secret** shown in the modal.

## 5. Put credentials into `.env`

Open `/Users/shubham/Documents/Dumps/easyjob/.env` and fill:

```env
GOOGLE_CLIENT_ID=1234567890-xxxxxxx.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=GOCSPX-xxxxxxxxxxxx
```

(`JWT_SECRET` and `TOKEN_ENCRYPTION_KEY` are already generated in your `.env`. Leave them alone unless you're rotating.)

## 6. Restart the backend

```bash
cd backend && go run ./cmd/api
```

You should NO LONGER see `google oauth not configured` in the logs.

## 7. Test the flow end-to-end

1. Open http://localhost:5173 — you'll be bounced to `/login`.
2. Click **Sign in with Google** → Google consent screen appears.
3. Grant access (it will say the app isn't verified — click **Advanced → Go to OneApply (unsafe)** since you're the test user).
4. You get redirected back to `/outreach`. Sidebar shows your email at the bottom.
5. In the Chrome extension popup (from `chrome://extensions` → Load unpacked `extension/dist`), you should see "Signed in as your@email.com. Gmail connected."

## Troubleshooting

- **`redirect_uri_mismatch`** — the URI in the OAuth client must exactly match `http://localhost:8080/api/auth/google/callback` (no trailing slash).
- **`access_denied`** — you're logging in with a Google account that isn't in the OAuth consent screen's "Test users" list. Add it.
- **Extension popup keeps saying "Sign in"** — Chrome extensions and the dashboard share cookies via `host_permissions: http://localhost:8080/*`, which is already set. If it's still not working, check `chrome://extensions → OneApply → details → Site access` and confirm it has access to `localhost:8080`.
- **CORS error in browser console** — the backend's CORS handler accepts any `chrome-extension://` origin plus `http://localhost:5173`. If you see errors, restart the backend.
- **DB error `citext type does not exist`** — the `users` migration creates the `citext` extension. If Postgres is fresh you're fine; if you had an old DB, drop and re-migrate.

## What Phase 1 gives you

- Real Google login on the dashboard.
- Google-issued refresh token encrypted at rest (AES-256-GCM) in `users.gmail_refresh_token_enc`.
- Session cookie shared by dashboard + extension (both send `credentials: 'include'`, backend accepts both origins).
- `/api/auth/me` returning the current user across both surfaces.
- Auth middleware ready to protect every Phase 2+ endpoint.
