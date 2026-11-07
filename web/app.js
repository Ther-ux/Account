// Member 09: compose the three independently implemented feature modules.
const appFeatures = [
  {id: "record-panel", name: "记账", load: () => import("./record.js"), init: "initRecord"},
  {id: "recent-panel", name: "最近记录", load: () => import("./recent.js"), init: "initRecent"},
  {id: "stats-panel", name: "收支统计", load: () => import("./stats.js"), init: "initStats"},
];
const appStarted = new WeakSet();

function appUpdateNavigation() {
  const section = ["#record", "#recent", "#stats"].includes(window.location.hash) ? window.location.hash : "#record";
  for (const link of document.querySelectorAll(".app-nav a")) {
    if (link.getAttribute("href") === section) link.setAttribute("aria-current", "location");
    else link.removeAttribute("aria-current");
  }
}

async function appStartFeature(feature) {
  const container = document.getElementById(feature.id);
  if (!container || appStarted.has(container)) return;
  appStarted.add(container);
  try {
    const module = await feature.load();
    if (typeof module[feature.init] !== "function") throw new Error("Missing feature initializer: " + feature.init);
    await module[feature.init](container);
  } catch (error) {
    const message = document.createElement("p");
    message.className = "app-module-error";
    message.setAttribute("role", "alert");
    message.textContent = feature.name + "暂时无法加载，请刷新页面重试。";
    container.replaceChildren(message);
    // Keep diagnostics in the console, rather than exposing technical details in the UI.
    console.error("Unable to initialize " + feature.id, error);
  }
}

function appStart() {
  appUpdateNavigation();
  window.addEventListener("hashchange", appUpdateNavigation);
  // One module failing to load must not prevent the other two from initializing.
  void Promise.allSettled(appFeatures.map(appStartFeature));
}

if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", appStart, {once: true});
else appStart();

