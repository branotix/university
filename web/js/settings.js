Auth.requireLogin();

document.getElementById("logout-btn").onclick = () => Auth.logout();

let girlsOnlyEnabled = false;

async function loadMe() {
  try {
    const res = await apiRequest("/api/me", { auth: true });
    const me = res.data;
    document.getElementById("s-name").textContent = me.name;
    document.getElementById("s-email").textContent = me.email + (me.email_verified ? " ✅" : " ⚠️ (ভেরিফাই করা হয়নি)");
    document.getElementById("s-phone").textContent = me.phone;
    document.getElementById("s-role").textContent = me.role === "teacher" ? "সিনিয়র/টিচার" : "স্টুডেন্ট";

    if (me.role === "teacher") {
      renderKycBanner(me.kyc_status);
      girlsOnlyEnabled = !!me.girls_only_mode;
      renderTeacherSettings(me);
    }
  } catch (err) {
    showToast(err.message, "error");
  }
}

function renderKycBanner(status) {
  if (status === "approved") return;
  const el = document.getElementById("kyc-banner");
  const msg = status === "rejected" ? "তোমার KYC রিজেক্ট হয়েছে। সাপোর্টে যোগাযোগ করো।" : "তোমার KYC এখনো এডমিন ভেরিফাই করেননি। ভেরিফাই না হওয়া পর্যন্ত তোমার প্রোফাইল ফিডে দেখা যাবে না।";
  el.innerHTML = `<div class="card" style="border-color:var(--gold); margin-bottom:20px"><strong>⏳ KYC Pending</strong><p style="margin:8px 0 0" class="muted">${msg}</p></div>`;
}

function renderTeacherSettings(me) {
  const el = document.getElementById("teacher-settings");
  el.innerHTML = `
    <div class="card" style="margin-bottom:20px">
      <div class="section-title">
        <h2>Girls Only Mode</h2>
        <div id="girls-toggle" class="toggle ${girlsOnlyEnabled ? "on" : ""}"><div class="knob"></div></div>
      </div>
      <p class="muted" style="margin:0">অন করলে শুধু ফিমেল স্টুডেন্টরাই তোমাকে কল করতে পারবে।</p>
    </div>

    <div class="card" style="margin-bottom:20px">
      <div class="section-title"><h2>টাকা তোলো (Withdraw)</h2></div>
      <p class="muted" style="margin-top:0">ব্যালেন্স: <strong style="color:var(--gold)">৳${(me.balance || 0).toFixed(0)}</strong></p>
      <div class="field-row">
        <div class="field"><label>পরিমাণ (৳)</label><input type="number" id="wd-amount" min="10"></div>
        <div class="field"><label>bKash/Nagad নম্বর</label><input type="tel" id="wd-number" placeholder="017XXXXXXXX"></div>
      </div>
      <button class="btn btn-primary" id="wd-btn">উইথড্র রিকোয়েস্ট পাঠাও</button>
      <div class="divider"></div>
      <div id="wd-history"></div>
    </div>`;

  document.getElementById("girls-toggle").onclick = async () => {
    const newValue = !girlsOnlyEnabled;
    try {
      await apiRequest("/api/teacher/girls-only-mode", { method: "POST", auth: true, body: { enabled: newValue } });
      girlsOnlyEnabled = newValue;
      document.getElementById("girls-toggle").classList.toggle("on", girlsOnlyEnabled);
      showToast(newValue ? "Girls Only Mode চালু হয়েছে।" : "Girls Only Mode বন্ধ হয়েছে।", "success");
    } catch (err) {
      showToast(err.message, "error");
    }
  };

  document.getElementById("wd-btn").onclick = async () => {
    const amount = parseFloat(document.getElementById("wd-amount").value);
    const paymentNumber = document.getElementById("wd-number").value.trim();
    if (!amount || amount < 10) return showToast("সঠিক পরিমাণ দাও (কমপক্ষে ৳১০)।", "error");
    if (!paymentNumber) return showToast("bKash/Nagad নম্বর দাও।", "error");
    try {
      await apiRequest("/api/teacher/withdraw", { method: "POST", auth: true, body: { amount, payment_number: paymentNumber, method: "bkash" } });
      showToast("উইথড্র রিকোয়েস্ট পাঠানো হয়েছে।", "success");
      document.getElementById("wd-amount").value = "";
      document.getElementById("wd-number").value = "";
      loadMe();
      loadWithdrawals();
    } catch (err) {
      showToast(err.message, "error");
    }
  };

  loadWithdrawals();
}

async function loadWithdrawals() {
  const el = document.getElementById("wd-history");
  if (!el) return;
  try {
    const res = await apiRequest("/api/teacher/withdrawals", { auth: true });
    const list = res.data || [];
    if (list.length === 0) {
      el.innerHTML = `<p class="muted" style="margin:0">এখনো কোনো উইথড্র রিকোয়েস্ট নেই।</p>`;
      return;
    }
    const statusLabel = { pending: "⏳ পেন্ডিং", approved: "✅ অ্যাপ্রুভড", paid: "✅ পেইড হয়ে গেছে", rejected: "❌ বাতিল হয়েছে" };
    el.innerHTML = list
      .map(
        (wd) => `
      <div style="display:flex; justify-content:space-between; padding:8px 0; border-bottom:1px solid var(--border); font-size:13.5px">
        <span>৳${wd.amount.toFixed(0)} — ${wd.payment_number}</span>
        <span class="muted">${statusLabel[wd.status] || wd.status}</span>
      </div>`
      )
      .join("");
  } catch (err) {
    el.innerHTML = "";
  }
}

loadMe();
