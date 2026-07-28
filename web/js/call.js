Auth.requireLogin();

// ---------- Read call parameters from URL ----------
const params = new URLSearchParams(window.location.search);
const role = params.get("role"); // "student" or "teacher"
const sessionId = parseInt(params.get("session_id"));
const peerId = parseInt(params.get("peer_id"));
const peerName = params.get("peer_name") || "সহপাঠী";
const packageMinutes = parseInt(params.get("minutes") || "0");

const myUserId = parseInt(Auth.getUserId());

// ---------- ICE server config ----------
// Free public STUN always included. TURN credentials (if the server has one
// configured) are fetched fresh from the backend on every call — see
// getIceServers() below — instead of being hardcoded here, since permanent
// TURN credentials embedded in frontend JS could otherwise be copied out and
// used by anyone to relay unrelated traffic through your server.
const STUN_SERVERS = [{ urls: "stun:stun.l.google.com:19302" }];

async function getIceServers() {
  try {
    const res = await apiRequest("/api/turn-credentials", { auth: true });
    if (res.configured && res.turn) {
      return [
        ...STUN_SERVERS,
        { urls: res.turn.urls, username: res.turn.username, credential: res.turn.credential },
      ];
    }
  } catch (err) {
    console.warn("Could not fetch TURN credentials, falling back to STUN-only:", err);
  }
  return STUN_SERVERS;
}

let pc = null;
let localStream = null;
let iceServers = STUN_SERVERS;
let ws = null;
let candidateQueue = [];
let remoteDescSet = false;
let offerResendTimer = null;
let callTimerInterval = null;
let secondsElapsed = 0;
let micOn = true;
let camOn = true;
let callEnded = false;

const remoteVideo = document.getElementById("remote-video");
const localVideo = document.getElementById("local-video");
const callStatusName = document.getElementById("call-status-name");
const callTimerEl = document.getElementById("call-timer");
const waitingEl = document.getElementById("call-waiting");

callStatusName.textContent = peerName;

function showTapToPlayOverlay() {
  if (document.getElementById("tap-to-play")) return;
  const btn = document.createElement("button");
  btn.id = "tap-to-play";
  btn.textContent = "🔊 কল শুরু করতে ট্যাপ করো";
  btn.className = "btn btn-primary";
  btn.style.cssText = "position:absolute; top:50%; left:50%; transform:translate(-50%,-50%); z-index:10;";
  btn.onclick = () => {
    remoteVideo.play().catch(() => {});
    btn.remove();
  };
  document.querySelector(".call-stage").appendChild(btn);
}

async function init() {
  // getUserMedia only works on secure origins (https, or exactly "localhost").
  // If you're testing over a plain http:// LAN IP or a non-https tunnel,
  // Chrome will silently refuse camera/mic access (some other browsers are
  // more lenient in certain local setups, which is a common source of
  // "works in Firefox but not Chrome" confusion). Use an https tunnel like
  // ngrok, or open the app via http://localhost during local testing.
  const isSecure = window.location.protocol === "https:" || window.location.hostname === "localhost" || window.location.hostname === "127.0.0.1";
  if (!isSecure) {
    showToast("ক্যামেরা/মাইক অ্যাক্সেসের জন্য HTTPS বা localhost দরকার। এই URL-এ (http:// + IP) Chrome-এ কাজ নাও করতে পারে।", "error");
  }

  try {
    localStream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
    localVideo.srcObject = localStream;
  } catch (err) {
    showToast("ক্যামেরা/মাইক্রোফোন অ্যাক্সেস দরকার। ব্রাউজার পারমিশন চেক করো।", "error");
    return;
  }

  iceServers = await getIceServers();
  connectSignaling();
}

function connectSignaling() {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  ws = new WebSocket(`${protocol}//${window.location.host}/ws?user_id=${myUserId}`);

  ws.onopen = () => {
    if (role === "student") {
      startAsCaller();
    } else {
      waitingEl.textContent = "সংযোগ স্থাপন হচ্ছে...";
      loadPendingOfferAndAnswer();
    }
  };

  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    handleSignal(msg);
  };

  ws.onclose = () => {
    if (!callEnded) showToast("সংযোগ বিচ্ছিন্ন হয়ে গেছে।", "error");
  };
}

function createPeerConnection() {
  const conn = new RTCPeerConnection({ iceServers: iceServers });

  localStream.getTracks().forEach((track) => conn.addTrack(track, localStream));

  conn.ontrack = (event) => {
    remoteVideo.srcObject = event.streams[0];
    waitingEl.style.display = "none";
    startCallTimerUI();

    // Chrome (unlike Firefox in many local test setups) can block autoplay
    // of a video with sound until there's been a user gesture on the page.
    // Since the user already clicked "Accept"/purchased the package, this
    // usually plays fine — but just in case, retry and show a manual
    // "tap to enable video/audio" button if the browser still blocks it.
    const playPromise = remoteVideo.play();
    if (playPromise && playPromise.catch) {
      playPromise.catch(() => showTapToPlayOverlay());
    }
  };

  conn.onicecandidate = (event) => {
    if (event.candidate) {
      sendSignal("candidate", { candidate: event.candidate });
    }
  };

  conn.onconnectionstatechange = () => {
    if (conn.connectionState === "disconnected" || conn.connectionState === "failed") {
      if (!callEnded) showToast("কলের সংযোগ দুর্বল বা বিচ্ছিন্ন।", "error");
    }
  };

  return conn;
}

function sendSignal(type, dataExtra) {
  if (!ws || ws.readyState !== WebSocket.OPEN) return;
  ws.send(JSON.stringify({ type, target_id: peerId, data: dataExtra }));
}

// ---------- CALLER (student) flow ----------
async function startAsCaller() {
  waitingEl.textContent = "কলার সাথে সংযোগ করা হচ্ছে, একটু অপেক্ষা করো...";

  // Tell the backend to start the auto-disconnect timer for this session
  ws.send(JSON.stringify({
    type: "start_call",
    target_id: peerId,
    data: { session_id: sessionId, package_minutes: packageMinutes },
  }));

  pc = createPeerConnection();
  const offer = await pc.createOffer();
  await pc.setLocalDescription(offer);

  // The teacher might not have opened call.html yet (they need to click
  // "Accept" first), so we keep resending the offer every 1.5s until we
  // get an answer back. Once "answer" arrives, clearOfferResend() stops this.
  offerResendTimer = setInterval(() => {
    sendSignal("offer", {
      sdp: pc.localDescription.sdp,
      type: pc.localDescription.type,
      session_id: sessionId,
      package_minutes: packageMinutes,
      caller_name: localStorage.getItem("vn_name") || "Student",
    });
  }, 1500);
  // send immediately too
  sendSignal("offer", {
    sdp: pc.localDescription.sdp,
    type: pc.localDescription.type,
    session_id: sessionId,
    package_minutes: packageMinutes,
  });
}

function clearOfferResend() {
  if (offerResendTimer) {
    clearInterval(offerResendTimer);
    offerResendTimer = null;
  }
}

// ---------- CALLEE (teacher) flow ----------
// The offer is captured on the dashboard (teacher.js) and stashed in
// sessionStorage before navigating here, since the WS connection used to
// receive the offer is closed on page navigation.
async function loadPendingOfferAndAnswer() {
  const raw = sessionStorage.getItem("vn_pending_offer");
  if (!raw) {
    waitingEl.textContent = "কোনো ইনকামিং কল পাওয়া যায়নি।";
    return;
  }
  const offerData = JSON.parse(raw);
  sessionStorage.removeItem("vn_pending_offer");

  pc = createPeerConnection();
  await pc.setRemoteDescription({ type: "offer", sdp: offerData.sdp });
  remoteDescSet = true;
  flushCandidateQueue();

  const answer = await pc.createAnswer();
  await pc.setLocalDescription(answer);
  sendSignal("answer", { sdp: pc.localDescription.sdp, type: pc.localDescription.type, session_id: sessionId });
}

// ---------- Incoming signal handling ----------
async function handleSignal(msg) {
  switch (msg.type) {
    case "answer":
      if (pc && !remoteDescSet) {
        clearOfferResend();
        await pc.setRemoteDescription({ type: "answer", sdp: msg.data.sdp });
        remoteDescSet = true;
        flushCandidateQueue();
      }
      break;

    case "candidate":
      if (msg.data && msg.data.candidate) {
        if (remoteDescSet && pc) {
          try { await pc.addIceCandidate(msg.data.candidate); } catch (e) { /* ignore */ }
        } else {
          candidateQueue.push(msg.data.candidate);
        }
      }
      break;

    case "leave":
      endCall(false, "অপরপক্ষ কল কেটে দিয়েছে।");
      break;

    case "call_ended":
      endCall(false, "প্যাকেজের সময় শেষ — কল অটোমেটিক্যালি শেষ হয়ে গেছে।");
      break;
  }
}

function flushCandidateQueue() {
  candidateQueue.forEach(async (c) => {
    try { await pc.addIceCandidate(c); } catch (e) { /* ignore */ }
  });
  candidateQueue = [];
}

// ---------- UI: timer, mute, camera, hangup ----------
function startCallTimerUI() {
  if (callTimerInterval) return;
  callTimerInterval = setInterval(() => {
    secondsElapsed++;
    const m = String(Math.floor(secondsElapsed / 60)).padStart(2, "0");
    const s = String(secondsElapsed % 60).padStart(2, "0");
    callTimerEl.textContent = `${m}:${s}`;
  }, 1000);
}

document.getElementById("mic-btn").onclick = (e) => {
  micOn = !micOn;
  localStream.getAudioTracks().forEach((t) => (t.enabled = micOn));
  e.currentTarget.classList.toggle("off", !micOn);
  e.currentTarget.textContent = micOn ? "🎤" : "🔇";
};

document.getElementById("cam-btn").onclick = (e) => {
  camOn = !camOn;
  localStream.getVideoTracks().forEach((t) => (t.enabled = camOn));
  e.currentTarget.classList.toggle("off", !camOn);
  e.currentTarget.textContent = camOn ? "📹" : "🚫";
};

document.getElementById("end-btn").onclick = () => {
  sendSignal("leave", {});
  finalizeSession();
  endCall(true, "");
};

async function finalizeSession() {
  try {
    await apiRequest("/api/sessions/end", { method: "POST", auth: true, body: { session_id: sessionId } });
  } catch (err) {
    // Non-fatal for the UI — the server-side package timer is a safety net
    // that will settle this session anyway once the purchased time runs out.
    console.error("Failed to finalize session:", err);
  }
}

function endCall(silent, message) {
  if (callEnded) return;
  callEnded = true;
  clearOfferResend();
  if (callTimerInterval) clearInterval(callTimerInterval);
  if (pc) pc.close();
  if (localStream) localStream.getTracks().forEach((t) => t.stop());
  if (ws) ws.close();

  if (message) showToast(message, "info");

  setTimeout(() => {
    if (role === "student") {
      window.location.href = `/review.html?session_id=${sessionId}&teacher_id=${peerId}&teacher_name=${encodeURIComponent(peerName)}`;
    } else {
      window.location.href = "/feed.html";
    }
  }, silent ? 300 : 1800);
}

window.addEventListener("beforeunload", () => {
  // Best-effort: tell the peer we're leaving. We can't reliably call the
  // authenticated /api/sessions/end endpoint from beforeunload (no way to
  // attach the Authorization header via sendBeacon), so if the tab is just
  // closed without clicking "end call", the package timer on the server
  // (services.StartCallTimer) is what eventually settles this session.
  if (!callEnded) sendSignal("leave", {});
});

init();
