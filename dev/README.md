# Frontend

See the [repository README](../README.md) for runtime versions, environment and API setup.

`npm ci`, `npm run build`, and `npm test` use the committed lockfile. `npm run dev -- --background` starts Astro; `/api` proxies to Go on `127.0.0.1:3030`. `npm exec astro dev stop` stops it. Browser tests target the Go production build by default, so they also exercise the CSP and React hydration.

The guestbook limits match Go in Unicode code points, retains input on failure, waits for hydration before enabling submission, and announces moderation/loading/errors accessibly. User text is rendered through React text nodes.
