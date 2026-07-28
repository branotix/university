// ---- Varsity Network: shared API helper ----
// Everything here talks to the Go backend running on the same origin.
const API_BASE = ""; // same-origin, since Go serves /web as static files

const Auth = {
  getToken() {
    return localStorage.getItem("vn_token");
  },
  getRole() {
    return localStorage.getItem("vn_role");
  },
  getUserId() {
    return localStorage.getItem("vn_user_id");
  },
  setSession(token, role, userId) {
    localStorage.setItem("vn_token", token);
    localStorage.setItem("vn_role", role);
    localStorage.setItem("vn_user_id", userId);
  },
  clearSession() {
    localStorage.removeItem("vn_token");
    localStorage.removeItem("vn_role");
    localStorage.removeItem("vn_user_id");
  },
  requireLogin() {
    if (!this.getToken()) {
      window.location.href = "/index.html";
    }
  },
  logout() {
    this.clearSession();
    window.location.href = "/index.html";
  },
};

async function apiRequest(path, { method = "GET", body = null, auth = false } = {}) {
  const headers = { "Content-Type": "application/json" };
  if (auth) {
    const token = Auth.getToken();
    if (!token) {
      Auth.logout();
      throw new Error("Not logged in");
    }
    headers["Authorization"] = "Bearer " + token;
  }

  const res = await fetch(API_BASE + path, {
    method,
    headers,
    credentials: "same-origin", // send the vn_token persistent-login cookie
    body: body ? JSON.stringify(body) : null,
  });

  let data = null;
  const text = await res.text();
  try {
    data = text ? JSON.parse(text) : null;
  } catch (e) {
    data = { message: text };
  }

  if (!res.ok) {
    const message = (data && data.message) || (typeof data === "string" ? data : text) || `Request failed (${res.status})`;
    if (res.status === 401) {
      Auth.logout();
    }
    throw new Error(message);
  }

  return data;
}

function showToast(message, type = "info") {
  const container = document.getElementById("toast-container") || createToastContainer();
  const toast = document.createElement("div");
  toast.className = `toast toast-${type}`;
  toast.textContent = message;
  container.appendChild(toast);
  requestAnimationFrame(() => toast.classList.add("toast-show"));
  setTimeout(() => {
    toast.classList.remove("toast-show");
    setTimeout(() => toast.remove(), 300);
  }, 3800);
}

function createToastContainer() {
  const el = document.createElement("div");
  el.id = "toast-container";
  document.body.appendChild(el);
  return el;
}

// Uploads a File object (from an <input type="file">) and returns the saved
// URL. type is "kyc" (student ID cards) or "avatars" (profile/cover photos).
async function uploadImage(file, type) {
  const formData = new FormData();
  formData.append("file", file);
  const res = await fetch(`/api/upload?type=${encodeURIComponent(type)}`, {
    method: "POST",
    credentials: "same-origin",
    body: formData,
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.message || "Upload failed");
  return data.url;
}
