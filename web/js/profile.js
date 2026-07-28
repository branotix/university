Auth.requireLogin();

const qs = new URLSearchParams(window.location.search);
const myId = parseInt(Auth.getUserId());
const profileId = parseInt(qs.get("id")) || myId;
const isOwnProfile = profileId === myId;

let meData = null; // only populated for own profile (has private fields)
let profileRole = "";

async function load() {
  try {
    const res = await apiRequest(`/api/profiles/${profileId}`);
    render(res.data);
  } catch (err) {
    document.getElementById("p-name").textContent = "প্রোফাইল পাওয়া যায়নি";
    showToast(err.message, "error");
    return;
  }

  if (isOwnProfile) {
    try {
      const me = await apiRequest("/api/me", { auth: true });
      meData = me.data;
      renderOwnBanner();
    } catch (err) {
      /* non-fatal */
    }
  }
}

function render(p) {
  profileRole = p.role;
  document.title = `${p.name} — Varsity Network`;

  const initials = (p.name || "?").trim().charAt(0).toUpperCase();
  document.getElementById("avatar-initial").innerHTML = p.profile_photo_url
    ? `<img src="${p.profile_photo_url}" style="width:100%;height:100%;object-fit:cover;border-radius:50%">`
    : initials;
  document.getElementById("avatar-ring").classList.toggle("online", p.is_online);

  if (p.cover_photo_url) {
    document.getElementById("cover-el").style.backgroundImage = `url('${p.cover_photo_url}')`;
  }

  document.getElementById("p-name").textContent = p.name;
  document.getElementById("p-headline").textContent = p.headline || (p.role === "teacher" ? p.expertise || "" : "");
  document.getElementById("p-role-badge").innerHTML =
    p.role === "teacher"
      ? `🎓 সিনিয়র/টিচার${p.university ? " · " + escapeHtml(p.university) : ""}${p.girls_only_mode ? ' · <span class="badge-girls">Girls Only</span>' : ""}`
      : "🧑‍🎓 স্টুডেন্ট";
  document.getElementById("p-location").textContent = p.location ? "📍 " + p.location : "";
  document.getElementById("p-languages").textContent = p.languages ? "🗣️ " + p.languages : "";
  document.getElementById("p-joined").textContent = p.joined_at ? "যোগ দিয়েছে: " + new Date(p.joined_at).toLocaleDateString("bn-BD") : "";
  document.getElementById("p-about").textContent = p.about || "কিছু লেখা নেই।";

  if (p.role === "teacher") {
    document.getElementById("teacher-info-card").style.display = "block";
    document.getElementById("p-university").textContent = p.university || "—";
    document.getElementById("p-expertise").textContent = p.expertise || "—";

    document.getElementById("stat-grid").innerHTML = `
      <div class="stat-box"><div class="num">${p.average_rating ? p.average_rating.toFixed(1) + " ★" : "New"}</div><div class="label">গড় রেটিং</div></div>
      <div class="stat-box"><div class="num">${p.total_services_given || 0}</div><div class="label">সম্পন্ন সেশন</div></div>
      <div class="stat-box"><div class="num">${p.is_online ? "🟢" : "🔴"}</div><div class="label">${p.is_online ? "Online" : "Offline"}</div></div>`;
  }

  renderHeaderActions(p);
}

function renderHeaderActions(p) {
  const el = document.getElementById("header-actions");
  if (isOwnProfile) {
    el.innerHTML = `<button class="btn btn-secondary btn-sm" id="edit-profile-btn">✏️ প্রোফাইল এডিট করো</button>`;
    document.getElementById("edit-profile-btn").onclick = openEditModal;
  } else {
    let buttons = `<button class="btn btn-secondary btn-sm" id="message-btn">💬 মেসেজ</button>`;
    if (p.role === "teacher" && p.kyc_approved) {
      buttons += ` <button class="btn btn-primary btn-sm" id="book-call-btn">📞 কল বুক করো</button>`;
    }
    el.innerHTML = buttons;
    document.getElementById("message-btn").onclick = () => (window.location.href = `/messages.html?with=${profileId}`);
    const bookBtn = document.getElementById("book-call-btn");
    if (bookBtn) bookBtn.onclick = () => openPkgModal(p);
  }
}

function renderOwnBanner() {
  const el = document.getElementById("own-profile-banner");
  let html = "";

  if (!meData.email_verified) {
    html += `<div class="card" style="border-color:var(--rose); margin:12px 0"><strong>⚠️ ইমেইল ভেরিফাই করা হয়নি</strong></div>`;
  }
  if (meData.role === "teacher" && meData.kyc_status !== "approved") {
    const msg = meData.kyc_status === "rejected" ? "তোমার KYC রিজেক্ট হয়েছে।" : "তোমার KYC এডমিন এখনো ভেরিফাই করেনি।";
    html += `<div class="card" style="border-color:var(--gold); margin:12px 0"><strong>⏳ KYC Pending</strong><p class="muted" style="margin:6px 0 0">${msg}</p></div>`;
  }

  const fields = [meData.headline, meData.about, meData.location, meData.languages, meData.profile_photo_url];
  if (meData.role === "teacher") fields.push(meData.bio, meData.expertise, meData.university);
  const filled = fields.filter((f) => f && f.trim && f.trim().length > 0).length;
  const pct = Math.round((filled / fields.length) * 100);

  html += `
    <div class="card" style="margin:12px 0">
      <div style="display:flex; justify-content:space-between; font-size:13px"><span>প্রোফাইল সম্পূর্ণতা</span><strong>${pct}%</strong></div>
      <div class="completion-bar"><div class="completion-fill" style="width:${pct}%"></div></div>
      ${pct < 100 ? '<p class="muted" style="font-size:12.5px; margin:8px 0 0">"প্রোফাইল এডিট করো" চেপে বাকি তথ্য যোগ করো।</p>' : ""}
    </div>`;

  el.innerHTML = html;
}

function escapeHtml(str) {
  const div = document.createElement("div");
  div.textContent = str || "";
  return div.innerHTML;
}

// ---------- Edit profile ----------
function openEditModal() {
  document.getElementById("edit-headline").value = meData.headline || "";
  document.getElementById("edit-about").value = meData.about || "";
  document.getElementById("edit-location").value = meData.location || "";
  document.getElementById("edit-languages").value = meData.languages || "";
  document.getElementById("edit-teacher-fields").style.display = meData.role === "teacher" ? "block" : "none";
  if (meData.role === "teacher") {
    document.getElementById("edit-university").value = meData.university || "";
    document.getElementById("edit-expertise").value = meData.expertise || "";
    document.getElementById("edit-bio").value = meData.bio || "";
  }
  document.getElementById("edit-modal").style.display = "flex";
}

document.getElementById("save-profile-btn").onclick = async () => {
  const btn = document.getElementById("save-profile-btn");
  btn.disabled = true;
  try {
    const body = {
      headline: document.getElementById("edit-headline").value,
      about: document.getElementById("edit-about").value,
      location: document.getElementById("edit-location").value,
      languages: document.getElementById("edit-languages").value,
    };

    const avatarFile = document.getElementById("edit-avatar-file").files[0];
    if (avatarFile) body.profile_photo_url = await uploadImage(avatarFile, "avatars");
    const coverFile = document.getElementById("edit-cover-file").files[0];
    if (coverFile) body.cover_photo_url = await uploadImage(coverFile, "avatars");

    if (meData.role === "teacher") {
      body.university = document.getElementById("edit-university").value;
      body.expertise = document.getElementById("edit-expertise").value;
      body.bio = document.getElementById("edit-bio").value;
    }

    await apiRequest("/api/me", { method: "PATCH", auth: true, body });
    showToast("প্রোফাইল আপডেট হয়েছে!", "success");
    document.getElementById("edit-modal").style.display = "none";
    location.reload();
  } catch (err) {
    showToast(err.message, "error");
  } finally {
    btn.disabled = false;
  }
};

// ---------- Book a call ----------
let selectedPkg = null;
let bookingTeacher = null;

function openPkgModal(teacherProfile) {
  bookingTeacher = teacherProfile;
  selectedPkg = null;
  document.getElementById("pkg-modal-title").textContent = `${teacherProfile.name}-এর সাথে কল বুক করো`;
  document.querySelectorAll(".pkg-option").forEach((el) => el.classList.remove("selected"));
  document.getElementById("confirm-pkg-btn").disabled = true;
  document.getElementById("pkg-modal").style.display = "flex";
}

document.querySelectorAll(".pkg-option").forEach((el) => {
  el.onclick = () => {
    document.querySelectorAll(".pkg-option").forEach((o) => o.classList.remove("selected"));
    el.classList.add("selected");
    selectedPkg = { minutes: parseInt(el.dataset.mins), amount: parseFloat(el.dataset.amount) };
    document.getElementById("confirm-pkg-btn").disabled = false;
  };
});

document.getElementById("confirm-pkg-btn").onclick = async () => {
  if (!bookingTeacher || !selectedPkg) return;
  try {
    const res = await apiRequest("/api/packages/purchase", {
      method: "POST",
      auth: true,
      body: { teacher_id: bookingTeacher.id, package_minutes: selectedPkg.minutes, amount: selectedPkg.amount },
    });
    const session = res.data;
    window.location.href = `/call.html?role=student&session_id=${session.session_id}&peer_id=${bookingTeacher.id}&peer_name=${encodeURIComponent(bookingTeacher.name)}&minutes=${selectedPkg.minutes}`;
  } catch (err) {
    showToast(err.message, "error");
  }
};

load();
