// Member 06: standalone ES module. No framework, global CSS or shared helper required.
const recordInstances = new WeakMap();
const recordCategories = {
  income: ["工资", "奖金", "其他收入"],
  expense: ["饮食", "交通", "购物", "其他支出"],
};
let recordSequence = 0;

function recordLocalDate() {
  const date = new Date();
  return [date.getFullYear(), String(date.getMonth() + 1).padStart(2, "0"),
    String(date.getDate()).padStart(2, "0")].join("-");
}

function recordValidDate(value) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value) || value.startsWith("0000")) return false;
  const [year, month, day] = value.split("-").map(Number);
  const date = new Date(0);
  date.setHours(0, 0, 0, 0);
  date.setFullYear(year, month - 1, day);
  return date.getFullYear() === year && date.getMonth() === month - 1 && date.getDate() === day;
}

function recordParseAmount(value) {
  if (!/^\d+(?:\.\d{1,2})?$/.test(value)) throw new Error("金额须为正数，最多保留两位小数。");
  const [yuan, fraction = ""] = value.split(".");
  const cents = BigInt(yuan) * 100n + BigInt(fraction.padEnd(2, "0"));
  if (cents <= 0n || cents > BigInt(Number.MAX_SAFE_INTEGER)) throw new Error("金额须大于 0 且在可计算范围内。");
  return Number(cents);
}

// Repeated initialization of the same container returns the same controller.
export function initRecord(container) {
  if (!container || typeof container.replaceChildren !== "function") throw new TypeError("记账容器无效");
  if (recordInstances.has(container)) return recordInstances.get(container);
  const prefix = "record-" + (++recordSequence);
  const form = document.createElement("form");
  form.className = "record-form";
  form.noValidate = true;
  // Only static markup and our own numeric prefix enter this template.
  form.innerHTML = `
    <fieldset class="record-fields">
      <legend>填写账目信息</legend>
      <div class="record-field"><label for="${prefix}-date">日期</label>
        <input id="${prefix}-date" name="date" type="date" required></div>
      <div class="record-field"><label for="${prefix}-type">收支类型</label>
        <select id="${prefix}-type" name="type"><option value="expense">支出</option><option value="income">收入</option></select></div>
      <div class="record-field"><label for="${prefix}-amount">金额（元）</label>
        <input id="${prefix}-amount" name="amount" type="text" inputmode="decimal" autocomplete="off"
          placeholder="例如 12.50" aria-describedby="${prefix}-amount-help" required>
        <small id="${prefix}-amount-help">大于 0，最多两位小数</small></div>
      <div class="record-field"><label for="${prefix}-category">类别</label>
        <select id="${prefix}-category" name="category" required></select></div>
      <div class="record-field record-field-wide"><label for="${prefix}-note">备注（可选）</label>
        <textarea id="${prefix}-note" name="note" rows="2" placeholder="记下这笔收支的用途"></textarea></div>
      <div class="record-actions"><button type="submit" class="record-submit">保存账目</button></div>
    </fieldset>
    <p class="record-feedback" role="status" aria-live="polite"></p>`;
  container.replaceChildren(form);
  const fields = form.querySelector("fieldset");
  const dateInput = form.elements.namedItem("date");
  const typeInput = form.elements.namedItem("type");
  const amountInput = form.elements.namedItem("amount");
  const categoryInput = form.elements.namedItem("category");
  const noteInput = form.elements.namedItem("note");
  const submitButton = form.querySelector('[type="submit"]');
  const feedback = form.querySelector(".record-feedback");
  let saving = false;
  let destroyed = false;
  let requestController = null;

  function show(message, state = "info") {
    feedback.textContent = message;
    feedback.dataset.state = state;
  }
  function updateCategories() {
    categoryInput.replaceChildren(...recordCategories[typeInput.value].map((category) => {
      const option = document.createElement("option");
      option.value = category;
      option.textContent = category;
      return option;
    }));
  }
  function validateField(input) {
    try {
      if (input === dateInput && !recordValidDate(dateInput.value)) throw new Error("请选择真实有效的日期。");
      if (input === amountInput) recordParseAmount(amountInput.value.trim());
      input.removeAttribute("aria-invalid");
      return true;
    } catch (error) {
      input.setAttribute("aria-invalid", "true");
      show(error.message, "error");
      return false;
    }
  }
  async function submit(event) {
    event.preventDefault();
    if (saving || destroyed) return;
    if (!validateField(dateInput)) { dateInput.focus(); return; }
    if (!validateField(amountInput)) { amountInput.focus(); return; }
    if (!recordCategories[typeInput.value]?.includes(categoryInput.value)) {
      show("请选择与收支类型匹配的类别。", "error");
      return;
    }
    const record = {
      date: dateInput.value, type: typeInput.value,
      amountCents: recordParseAmount(amountInput.value.trim()),
      category: categoryInput.value, note: noteInput.value,
    };
    saving = true;
    fields.disabled = true;
    form.setAttribute("aria-busy", "true");
    submitButton.textContent = "正在保存…";
    show("正在保存，请稍候。");
    requestController = new AbortController();
    let saved = false;
    try {
      const response = await fetch("/api/records", {
        method: "POST", headers: {"Content-Type": "application/json"},
        body: JSON.stringify(record), signal: requestController.signal,
      });
      let result;
      try { result = await response.json(); }
      catch { throw new Error("服务返回了无法识别的响应，请稍后核对账本再重试。"); }
      if (response.status !== 201) {
        throw new Error(typeof result?.error === "string" ? result.error : "保存失败，请稍后重试。");
      }
      if (!result || result.date !== record.date || result.type !== record.type ||
          result.amountCents !== record.amountCents || result.category !== record.category ||
          (result.note ?? "") !== record.note) {
        throw new Error("服务未返回约定的账目，请核对账本，避免重复保存。");
      }
      if (destroyed) return;
      amountInput.value = "";
      noteInput.value = "";
      show("保存成功。", "success");
      saved = true;
    } catch (error) {
      if (!destroyed) show(error.name === "AbortError" ? "保存请求已取消，请核对账本后重试。" :
        (error instanceof TypeError ? "网络连接失败，请检查服务后重试。" : error.message), "error");
    } finally {
      saving = false;
      if (!destroyed) {
        fields.disabled = false;
        form.removeAttribute("aria-busy");
        submitButton.textContent = "保存账目";
      }
    }
    if (saved && !destroyed) {
      window.dispatchEvent(new Event("ledger:changed"));
      amountInput.focus();
    }
  }
  dateInput.value = recordLocalDate();
  updateCategories();
  typeInput.addEventListener("change", updateCategories);
  for (const input of [dateInput, amountInput]) input.addEventListener("blur", () => {
    if (input.value) validateField(input);
  });
  form.addEventListener("submit", submit);
  const controller = {
    destroy() {
      destroyed = true;
      requestController?.abort();
      form.remove();
      recordInstances.delete(container);
    },
  };
  recordInstances.set(container, controller);
  return controller;
}

