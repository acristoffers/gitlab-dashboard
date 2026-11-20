(() => {
  const content = document.getElementById("content");
  const stamp = document.getElementById("stamp");
  const button = document.getElementById("refresh");
  const every = (parseInt(document.body.dataset.refresh, 10) || 60) * 1000;
  let lastLoad = Date.now();
  let busy = false;

  function ago(sec) {
    const s = Math.max(0, Math.round(Date.now() / 1000 - sec));
    if (s < 10) return "just now";
    if (s < 60) return `${s}s ago`;
    if (s < 3600) return `${Math.floor(s / 60)}m ago`;
    return `${Math.floor(s / 3600)}h ago`;
  }

  function tick() {
    const el = document.getElementById("fetched-at");
    const at = el && parseInt(el.dataset.at, 10);
    stamp.textContent = at ? `Synced ${ago(at)}` : "";
  }

  async function reload(force) {
    if (busy) return;
    busy = true;
    button.classList.add("spinning");
    try {
      const res = await fetch(`/?partial=1${force ? "&force=1" : ""}`, { cache: "no-store" });
      if (!res.ok) throw new Error(res.statusText);
      content.innerHTML = await res.text();
      lastLoad = Date.now();
    } catch (err) {
      stamp.textContent = "Dashboard server unreachable";
      busy = false;
      button.classList.remove("spinning");
      return;
    }
    busy = false;
    button.classList.remove("spinning");
    tick();
  }

  button.addEventListener("click", () => reload(true));
  document.addEventListener("keydown", (e) => {
    if (e.key === "r" && !e.metaKey && !e.ctrlKey && !e.altKey && !/INPUT|TEXTAREA/.test(e.target.tagName)) {
      reload(true);
    }
  });
  document.addEventListener("visibilitychange", () => {
    if (!document.hidden && Date.now() - lastLoad > every) reload(false);
  });
  setInterval(() => { if (!document.hidden) reload(false); }, every);
  setInterval(tick, 5000);
  tick();
})();
