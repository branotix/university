Auth.requireLogin();

let currentRole = "";
let currentUniversity = "";

async function loadStats() {
  const el = document.getElementById("platform-stats");
  try {
    const res = await apiRequest("/api/stats");
    const s = res.data;
    el.innerHTML = `
      <div class="stat-box"><div class="num">🟢 ${s.online_students}</div><div class="label">স্টুডেন্ট অনলাইনে আছে</div></div>
      <div class="stat-box"><div class="num">🟢 ${s.online_teachers}</div><div class="label">সিনিয়র/টিচার অনলাইনে আছে</div></div>
      <div class="stat-box"><div class="num">📞 ${s.total_completed_sessions}</div><div class="label">মোট সম্পন্ন সেশন</div></div>`;
  } catch (err) {
    el.innerHTML = "";
  }
}

async function loadMe() {
  try {
    const res = await apiRequest("/api/me", { auth: true });
    document.getElementById("wallet-chip").textContent = "৳ " + (res.data.balance || 0).toFixed(0);
  } catch (err) {
    /* non-fatal */
  }
}

async function loadFeed() {
  const grid = document.getElementById("feed-grid");
  grid.innerHTML = `<div class="empty-state">লোড হচ্ছে...</div>`;
  try {
    const params = new URLSearchParams();
    if (currentRole) params.set("role", currentRole);
    if (currentUniversity) params.set("university", currentUniversity);
    const res = await apiRequest("/api/feed?" + params.toString());
    const list = res.data || [];
    if (list.length === 0) {
      grid.innerHTML = `<div class="empty-state">কোনো প্রোফাইল পাওয়া যায়নি। অন্য ফিল্টার ট্রাই করো।</div>`;
      return;
    }
    grid.innerHTML = list.map(renderCard).join("");
  } catch (err) {
    grid.innerHTML = `<div class="empty-state">লোড করতে সমস্যা হয়েছে।</div>`;
    showToast(err.message, "error");
  }
}

function renderCard(p) {
  const initials = (p.name || "?").trim().charAt(0).toUpperCase();
  const rating = p.average_rating ? p.average_rating.toFixed(1) : null;
  const roleLabel = p.role === "teacher" ? (p.university || "Teacher") : "Student";
  return `
    <a href="/profile.html?id=${p.id}" class="teacher-card" style="text-decoration:none; color:inherit">
      <div class="head">
        <div class="avatar-ring ${p.is_online ? "online" : ""}">
          <div class="avatar-inner">${p.profile_photo_url ? `<img src="${p.profile_photo_url}" style="width:100%;height:100%;object-fit:cover;border-radius:50%">` : initials}</div>
        </div>
        <div>
          <div class="name-row">
            <span class="name">${escapeHtml(p.name)}</span>
            <span class="badge-status ${p.is_online ? "badge-online" : "badge-offline"}">${p.is_online ? "Online" : "Offline"}</span>
          </div>
          <div class="uni">${escapeHtml(roleLabel)}${p.girls_only_mode ? ' &middot; <span class="badge-girls">Girls Only</span>' : ""}</div>
        </div>
      </div>
      <div class="bio">${escapeHtml(p.headline || p.expertise || "কোনো হেডলাইন দেওয়া নেই।")}</div>
      <div class="meta-row">
        <span>${rating ? `<span class="stars">★ ${rating}</span>` : '<span class="muted">নতুন</span>'}</span>
        ${p.role === "teacher" ? `<span>${p.total_services_given || 0} সেশন</span>` : `<span class="muted">${escapeHtml(p.location || "")}</span>`}
      </div>
    </a>`;
}

function escapeHtml(str) {
  const div = document.createElement("div");
  div.textContent = str || "";
  return div.innerHTML;
}

document.querySelectorAll("#role-filters .chip").forEach((chip) => {
  chip.onclick = () => {
    document.querySelectorAll("#role-filters .chip").forEach((c) => c.classList.remove("active"));
    chip.classList.add("active");
    currentRole = chip.dataset.role;
    loadFeed();
  };
});
document.querySelectorAll("#university-filters .chip").forEach((chip) => {
  chip.onclick = () => {
    document.querySelectorAll("#university-filters .chip").forEach((c) => c.classList.remove("active"));
    chip.classList.add("active");
    currentUniversity = chip.dataset.uni;
    loadFeed();
  };
});

// ---------- Top-up (bKash) ----------
document.getElementById("topup-btn").onclick = () => {
  document.getElementById("topup-modal").style.display = "flex";
};
document.getElementById("confirm-topup-btn").onclick = async () => {
  const amount = parseFloat(document.getElementById("topup-amount").value);
  if (!amount || amount < 10) {
    showToast("সঠিক পরিমাণ দাও (কমপক্ষে ৳১০)", "error");
    return;
  }
  try {
    const res = await apiRequest("/api/wallet/bkash/create", { method: "POST", auth: true, body: { amount } });
    window.location.href = res.bkash_url;
  } catch (err) {
    showToast(err.message, "error");
  }
};

loadMe();
loadFeed();
loadStats();
setInterval(loadFeed, 20000);
setInterval(loadStats, 20000);
