// ---- Varsity Network: shared top nav ----
// Include after api.js. Renders into <div id="app-nav"></div>.
// Usage: <script>renderNav("feed")</script> where the argument is the current page key.
function renderNav(activePage) {
  const mount = document.getElementById("app-nav");
  if (!mount) return;

  const role = Auth.getRole();
  const links = [
    { key: "feed", label: "🏠 ফিড", href: "/feed.html" },
    { key: "messages", label: "💬 মেসেজ", href: "/messages.html" },
    { key: "profile", label: "👤 প্রোফাইল", href: `/profile.html?id=${Auth.getUserId()}` },
    { key: "settings", label: "⚙️ সেটিংস", href: "/settings.html" },
  ];

  mount.innerHTML = `
    <div class="topbar">
      <div class="brand"><div class="seal"><span class="seal-mark">VN</span></div> <span class="hide-mobile">Varsity Network</span></div>
      <div class="nav-links">
        ${links
          .map(
            (l) =>
              `<a href="${l.href}" class="nav-link ${activePage === l.key ? "nav-link-active" : ""}">${l.label}</a>`
          )
          .join("")}
        <span data-presence-indicator class="chip">•</span>
        <button id="nav-logout-btn" class="btn btn-sm btn-danger">লগআউট</button>
      </div>
    </div>`;

  document.getElementById("nav-logout-btn").onclick = () => Auth.logout();
}
