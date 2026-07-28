// ---- Varsity Network: shared presence / incoming-call listener ----
// Include this (after api.js) on every page a logged-in user can be on, so
// a teacher can receive an incoming call no matter which page they're
// currently viewing (feed, profile, messages, settings) — not just one
// fixed "dashboard" page.
(function () {
  if (!Auth.getToken()) return;

  let ws = null;
  let pendingCaller = null;

  function injectModal() {
    if (document.getElementById("incoming-call-modal")) return;
    const wrap = document.createElement("div");
    wrap.innerHTML = `
      <div id="incoming-call-modal" class="modal-overlay" style="display:none">
        <div class="modal" style="text-align:center">
          <div class="avatar-ring online" style="margin:0 auto 14px"><div class="avatar-inner" id="caller-initial">?</div></div>
          <h3 id="caller-name" style="margin:6px 0 4px">একজন স্টুডেন্ট কল করছে</h3>
          <p class="muted" style="margin-top:0">ইনকামিং কল</p>
          <div style="display:flex; gap:10px; margin-top:16px">
            <button class="btn btn-danger btn-block" id="decline-call-btn">বাতিল</button>
            <button class="btn btn-teal btn-block" id="accept-call-btn">গ্রহণ করো</button>
          </div>
        </div>
      </div>`;
    document.body.appendChild(wrap.firstElementChild);

    document.getElementById("decline-call-btn").onclick = async () => {
      if (pendingCaller) {
        ws.send(JSON.stringify({ type: "leave", target_id: pendingCaller.senderId, data: {} }));
        const sessionId = pendingCaller.data.session_id;
        if (sessionId) {
          try {
            await apiRequest("/api/sessions/end", { method: "POST", auth: true, body: { session_id: sessionId } });
          } catch (err) {
            console.error("Failed to finalize declined session:", err);
          }
        }
      }
      document.getElementById("incoming-call-modal").style.display = "none";
      pendingCaller = null;
    };

    document.getElementById("accept-call-btn").onclick = () => {
      if (!pendingCaller) return;
      sessionStorage.setItem("vn_pending_offer", JSON.stringify(pendingCaller.data));
      const sessionId = pendingCaller.data.session_id;
      const peerId = pendingCaller.senderId;
      window.location.href = `/call.html?role=teacher&session_id=${sessionId}&peer_id=${peerId}`;
    };
  }

  function showIncomingCall(callerName) {
    injectModal();
    document.getElementById("caller-name").textContent = `${callerName} কল করছে`;
    document.getElementById("caller-initial").textContent = (callerName || "?").charAt(0).toUpperCase();
    document.getElementById("incoming-call-modal").style.display = "flex";
  }

  function connectWS() {
    const myUserId = parseInt(Auth.getUserId());
    if (!myUserId) return;
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    ws = new WebSocket(`${protocol}//${window.location.host}/ws?user_id=${myUserId}`);

    ws.onopen = () => {
      document.querySelectorAll("[data-presence-indicator]").forEach((el) => (el.textContent = "🟢 Online"));
    };
    ws.onclose = () => {
      document.querySelectorAll("[data-presence-indicator]").forEach((el) => (el.textContent = "🔴 Offline"));
      setTimeout(connectWS, 3000);
    };
    ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);
      if (msg.type === "offer" && Auth.getRole() === "teacher") {
        pendingCaller = { senderId: msg.sender_id, data: msg.data };
        showIncomingCall(msg.data.caller_name || "একজন স্টুডেন্ট");
      }
    };
  }

  connectWS();
})();
