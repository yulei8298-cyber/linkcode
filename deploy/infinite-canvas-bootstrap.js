(() => {
  const storeKey = "infinite-canvas:ai_config_store";
  const signatureKey = "linkcode-infinite-canvas:config-signature";
  const fixedBaseUrl = "https://api-fast.linkcode.site/v1";
  let restoringStore = false;

  const normalizeStoreValue = (value) => {
    try {
      const parsed = JSON.parse(value);
      const config = parsed?.state?.config;
      if (!config || typeof config !== "object") return value;
      const normalized = {
        ...parsed,
        state: {
          ...parsed.state,
          config: {
            ...config,
            baseUrl: fixedBaseUrl,
            apiFormat: "openai",
            channels: Array.isArray(config.channels)
              ? config.channels.map((channel) => ({ ...channel, baseUrl: fixedBaseUrl, apiFormat: "openai" }))
              : config.channels,
          },
        },
      };
      return JSON.stringify(normalized);
    } catch {
      return value;
    }
  };
  const originalSetItem = Storage.prototype.setItem;
  Storage.prototype.setItem = function(key, value) {
    if (key === storeKey && !restoringStore) {
      const normalized = normalizeStoreValue(value);
      restoringStore = true;
      try { return originalSetItem.call(this, key, normalized); } finally { restoringStore = false; }
    }
    return originalSetItem.call(this, key, value);
  };
  const existingStore = window.localStorage.getItem(storeKey);
  if (existingStore) {
    const normalizedExistingStore = normalizeStoreValue(existingStore);
    if (normalizedExistingStore !== existingStore) {
      restoringStore = true;
      try { originalSetItem.call(window.localStorage, storeKey, normalizedExistingStore); } finally { restoringStore = false; }
    }
  }

  const route = () => window.location.pathname;
  let addDraft = null;
  const textOf = (element) => (element?.textContent || "").replace(/\s/g, "");
  const storeSnapshot = () => window.localStorage.getItem(storeKey);
  const restoreAddDraft = () => {
    if (!addDraft) return;
    if (addDraft === null) return;
    restoringStore = true;
    try {
      if (addDraft) originalSetItem.call(window.localStorage, storeKey, addDraft);
      else window.localStorage.removeItem(storeKey);
    } finally { restoringStore = false; }
    addDraft = null;
    window.location.reload();
  };
  document.addEventListener("click", (event) => {
    if (route() !== "/config") return;
    const target = event.target instanceof Element ? event.target.closest("button") : null;
    if (!target) return;
    const label = textOf(target);
    if (label === "新增渠道") {
      addDraft = storeSnapshot();
      return;
    }
    if (addDraft !== null && (label === "取消" || target.getAttribute("aria-label") === "关闭")) {
      restoreAddDraft();
      return;
    }
    if (addDraft !== null && label === "保存") addDraft = null;
  }, true);

  const enforceConfigInputs = () => {
    if (route() !== "/config") return;
    document.querySelectorAll('input[aria-label="接口地址"], input[placeholder="https://api.example.com"]').forEach((input) => {
      if (input.value !== fixedBaseUrl) {
        const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
        setter?.call(input, fixedBaseUrl);
        input.dispatchEvent(new Event("input", { bubbles: true }));
      }
      input.readOnly = true;
      input.setAttribute("aria-readonly", "true");
    });
  };
  new MutationObserver(enforceConfigInputs).observe(document.documentElement, { childList: true, subtree: true });
  window.setTimeout(enforceConfigInputs, 0);

  // Keep the upstream main navigation intact while removing its optional
  // utility toolbar from the embedded view.
  const style = document.createElement("style");
  style.textContent = "header > div > div:last-child { display: none !important; }";
  document.head.appendChild(style);

  // Report the upstream router path so the host can restore the active tab
  // after the embedded view is recreated.
  const notifyParent = () => {
    if (window.parent !== window) {
      window.parent.postMessage({
        type: "linkcode-infinite-canvas-route",
        path: `${window.location.pathname}${window.location.search}`,
      }, "*");
    }
  };
  const originalPushState = history.pushState.bind(history);
  const originalReplaceState = history.replaceState.bind(history);
  history.pushState = (data, unused, url) => { originalPushState(data, unused, url); notifyParent(); };
  history.replaceState = (data, unused, url) => { originalReplaceState(data, unused, url); notifyParent(); };
  window.addEventListener("popstate", notifyParent);
  window.addEventListener("hashchange", notifyParent);
  window.setTimeout(notifyParent, 0);

  try {
    const params = new URLSearchParams(window.location.hash.slice(1));
    const raw = params.get("linkcodeConfig");
    if (!raw) return;

    const payload = JSON.parse(raw);
    if (!payload || typeof payload.signature !== "string" || !payload.config || typeof payload.config !== "object") return;

    if (localStorage.getItem(signatureKey) !== payload.signature) {
      localStorage.setItem(storeKey, JSON.stringify({
        state: {
          config: payload.config,
          webdav: { url: "", username: "", password: "", directory: "infinite-canvas", lastSyncedAt: "" },
        },
        version: 0,
      }));
      localStorage.setItem(signatureKey, payload.signature);
    }

    window.history.replaceState(null, "", `${window.location.pathname}${window.location.search}`);
  } catch {
    // 配置缺失时由原版应用自己的配置界面处理。
  }
})();
