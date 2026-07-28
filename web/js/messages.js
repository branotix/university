Auth.requireLogin();

const myId = parseInt(Auth.getUserId());
const qs = new URLSearchParams(window.location.search);
let openPeerId = parseInt(qs.get("with")) || null;
let pollTimer = null;

async function loadConversations() {
  const sidebar = document.getElementById("msg-sidebar");
  try {
    const res = await apiRequest("/api/messages/conversations", { auth: true });
    let list = res.data || [];

    // If we're opening a fresh conversation (from a profile page) that
    // doesn't have any messages yet, synthesize an entry so it still shows.
    if (openPeerId && !list.some((c) => c.peer_id === openPeerId)) {
      try {
        const profileRes = await apiRequest(`/api/profiles/${openPeerId}`);
        list = [{ peer_id: openPeerId, name: profileRes.data.name, profile_photo_url: profileRes.data.profile_photo_url, role: profileRes.data.role, is_online: profileRes.data.is_online, last_message: "", unread: 0 }, ...list];
      } catch (e) {
        /* ignore */
      }
    }

    if (list.length === 0) {
      sidebar.innerHTML = `<div class="empty-state">এখনো কোনো মেসেজ নেই। কারো প্রোফাইলে গিয়ে "মেসেজ" চাপো।</div>`;
      return;
    }

    sidebar.innerHTML = list
      .map(
        (c) => `
      <div class="msg-sidebar-item ${openPeerId === c.peer_id ? "active" : ""}" data-peer="${c.peer_id}" data-name="${escapeHtml(c.name)}">
        <div class="avatar-ring ${c.is_online ? "online" : ""}" style="width:42px;height:42px"><div class="avatar-inner" style="font-size:14px">${(c.name || "?").charAt(0).toUpperCase()}</div></div>
        <div style="flex:1; min-width:0">
          <div class="name">${escapeHtml(c.name)}</div>
          <div class="preview">${escapeHtml(c.last_message || "কথা শুরু করো...")}</div>
        </div>
        ${c.unread > 0 ? '<div class="unread-dot"></div>' : ""}
      </div>`
      )
      .join("");

    sidebar.querySelectorAll(".msg-sidebar-item").forEach((el) => {
      el.onclick = () => openThread(parseInt(el.dataset.peer), el.dataset.name);
    });

    if (openPeerId) {
      const match = list.find((c) => c.peer_id === openPeerId);
      openThread(openPeerId, match ? match.name : "");
    }
  } catch (err) {
    sidebar.innerHTML = `<div class="empty-state">লোড করতে সমস্যা হয়েছে।</div>`;
  }
}

async function openThread(peerId, peerName) {
  openPeerId = peerId;
  document.querySelectorAll(".msg-sidebar-item").forEach((el) => el.classList.toggle("active", parseInt(el.dataset.peer) === peerId));

  const threadEl = document.getElementById("msg-thread");
  threadEl.classList.add("open");
  document.getElementById("msg-sidebar").classList.add("has-thread-open");

  threadEl.innerHTML = `
    <div class="msg-thread-header">
      <button class="btn btn-sm btn-secondary hide-mobile-inverse" onclick="closeThreadMobile()" style="margin-right:8px">←</button>
      <a href="/profile.html?id=${peerId}" style="color:var(--text)">${escapeHtml(peerName)}</a>
    </div>
    <div class="msg-thread-body" id="thread-body"><div class="empty-state">লোড হচ্ছে...</div></div>
    <div class="msg-input-row">
      <input type="text" id="msg-input" placeholder="মেসেজ লেখো...">
      <button class="btn btn-primary btn-sm" id="msg-send-btn">পাঠাও</button>
    </div>`;

  document.getElementById("msg-send-btn").onclick = sendMessage;
  document.getElementById("msg-input").onkeydown = (e) => {
    if (e.key === "Enter") sendMessage();
  };

  await loadThread();

  clearInterval(pollTimer);
  pollTimer = setInterval(loadThread, 5000);
}

function closeThreadMobile() {
  document.getElementById("msg-thread").classList.remove("open");
  document.getElementById("msg-sidebar").classList.remove("has-thread-open");
}

async function loadThread() {
  if (!openPeerId) return;
  const body = document.getElementById("thread-body");
  if (!body) return;
  try {
    const res = await apiRequest(`/api/messages/thread/${openPeerId}`, { auth: true });
    const list = res.data || [];
    const wasAtBottom = body.scrollTop + body.clientHeight >= body.scrollHeight - 30;
    body.innerHTML = list
      .map((m) => `<div class="msg-bubble ${m.sender_id === myId ? "mine" : "theirs"}">${escapeHtml(m.content)}</div>`)
      .join("");
    if (list.length === 0) body.innerHTML = `<div class="empty-state">কথা শুরু করো...</div>`;
    if (wasAtBottom) body.scrollTop = body.scrollHeight;
  } catch (err) {
    /* keep old content on transient errors */
  }
}

async function sendMessage() {
  const input = document.getElementById("msg-input");
  const content = input.value.trim();
  if (!content || !openPeerId) return;
  input.value = "";
  try {
    await apiRequest("/api/messages", { method: "POST", auth: true, body: { receiver_id: openPeerId, content } });
    await loadThread();
  } catch (err) {
    showToast(err.message, "error");
  }
}

function escapeHtml(str) {
  const div = document.createElement("div");
  div.textContent = str || "";
  return div.innerHTML;
}

loadConversations();
setInterval(loadConversations, 15000);
