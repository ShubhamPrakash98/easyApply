# extension

Chrome extension (Manifest V3) for OneApply.

## Dev

```bash
pnpm dev
```

Then open `chrome://extensions`, enable Developer Mode, click **Load unpacked**, and pick `extension/dist`. HMR will rebuild on save; hit the reload icon on the extension card if needed.

## Build for distribution

```bash
pnpm build
```

Output: `extension/dist`.

## Icons

Add `public/icon-16.png`, `public/icon-48.png`, `public/icon-128.png` before your first build. Placeholders are fine for dev.
