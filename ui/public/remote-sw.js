// AgentDeck phone service worker (TS-08.R73). It only displays pushed
// attention notifications, routes a tap to the matching card, and keeps the
// app shell available. It never caches API responses: the Mac is the only
// source of truth, and stale data must look stale (FS-20.R23).
const SHELL = "agentdeck-shell-v1";

self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((key) => key !== SHELL).map((key) => caches.delete(key))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);
  if (event.request.method !== "GET" || url.origin !== self.location.origin || url.pathname.startsWith("/api/")) return;
  if (event.request.mode === "navigate") {
    event.respondWith(
      fetch(event.request)
        .then((response) => {
          const copy = response.clone();
          caches.open(SHELL).then((cache) => cache.put("/", copy));
          return response;
        })
        .catch(() => caches.match("/")),
    );
    return;
  }
  if (url.pathname.startsWith("/assets/")) {
    event.respondWith(
      caches.match(event.request).then(
        (hit) =>
          hit ||
          fetch(event.request).then((response) => {
            const copy = response.clone();
            caches.open(SHELL).then((cache) => cache.put(event.request, copy));
            return response;
          }),
      ),
    );
  }
});

// Notifications carry no actions: every decision happens inside the app
// (FS-20.R21). A shared tag replaces rather than stacks (FS-20.R20).
self.addEventListener("push", (event) => {
  let payload = {};
  try {
    payload = event.data ? event.data.json() : {};
  } catch {
    payload = {};
  }
  const title = typeof payload.title === "string" ? payload.title : "AgentDeck";
  event.waitUntil(
    self.registration.showNotification(title, {
      body: typeof payload.body === "string" ? payload.body : "",
      tag: typeof payload.tag === "string" ? payload.tag : "agentdeck",
      renotify: true,
      icon: "/remote-icon.svg",
      data: { url: typeof payload.url === "string" && payload.url.startsWith("/") ? payload.url : "/" },
    }),
  );
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const target = event.notification.data?.url || "/";
  event.waitUntil(
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((clients) => {
      for (const client of clients) {
        if ("focus" in client) {
          client.postMessage({ kind: "open", url: target });
          return client.focus();
        }
      }
      return self.clients.openWindow(target);
    }),
  );
});
