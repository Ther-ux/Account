// Member 08: independent statistics view, using the shared README API contract.
const statsInstances = new WeakMap();
const statsCategories = ["工资", "奖金", "其他收入", "饮食", "交通", "购物", "其他支出"];
let statsSequence = 0;

function statsValidDate(text) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(text) || text.startsWith("0000")) return false;
  const [year, month, day] = text.split("-").map(Number);
  const date = new Date(0);
  date.setHours(0, 0, 0, 0);
  date.setFullYear(year, month - 1, day);
  return date.getFullYear() === year && date.getMonth() === month - 1 && date.getDate() === day;
}

function statsParseYuan(text) {
  if (!/^\d+(?:\.\d{1,2})?$/.test(text)) throw new Error("金额须为非负数，最多保留两位小数。");
  const [yuan, fraction = ""] = text.split(".");
  const cents = BigInt(yuan) * 100n + BigInt(fraction.padEnd(2, "0"));
  if (cents > BigInt(Number.MAX_SAFE_INTEGER)) throw new Error("金额超出可计算范围。");
  return Number(cents);
}

function statsFormatCents(cents) {
  const value = BigInt(cents);
  const absolute = value < 0n ? -value : value;
  return (value < 0n ? "-" : "") + (absolute / 100n).toString() + "." +
    (absolute % 100n).toString().padStart(2, "0");
}

function statsValidateResult(result) {
  if (!result || typeof result !== "object" || Array.isArray(result)) throw new Error("统计接口未返回约定的结果。");
  for (const key of ["incomeCents", "expenseCents", "balanceCents"]) {
    if (!Number.isSafeInteger(result[key])) throw new Error("统计金额无效或超出安全显示范围。");
  }
  if (result.incomeCents < 0 || result.expenseCents < 0 ||
      BigInt(result.incomeCents) - BigInt(result.expenseCents) !== BigInt(result.balanceCents)) {
    throw new Error("统计金额不符合收支约定。");
  }
  return result;
}

function statsDescribeFilters(filters) {
  const parts = [];
  if (filters.from) parts.push("自 " + filters.from);
  if (filters.to) parts.push("至 " + filters.to);
  if (filters.category) parts.push(filters.category);
  if (filters.minCents !== undefined) parts.push("最低 " + statsFormatCents(filters.minCents) + " 元");
  if (filters.maxCents !== undefined) parts.push("最高 " + statsFormatCents(filters.maxCents) + " 元");
  return parts.length ? parts.join(" · ") : "全部账目";
}

// Idempotent initialization; optional controller.refresh() always uses applied filters.
export function initStats(container) {
  if (!container || typeof container.replaceChildren !== "function") throw new TypeError("统计容器无效");
  if (statsInstances.has(container)) return statsInstances.get(container);
  const prefix = "stats-" + (++statsSequence);
  const wrapper = document.createElement("div");
  wrapper.className = "stats-view";
  // Static template; user-provided strings are only assigned through DOM properties.
  wrapper.innerHTML = `
    <form class="stats-filter" novalidate aria-label="收支统计筛选">
      <div class="stats-field"><label for="${prefix}-from">起始日期</label>
        <input id="${prefix}-from" name="from" type="date"></div>
      <div class="stats-field"><label for="${prefix}-to">结束日期</label>
        <input id="${prefix}-to" name="to" type="date"></div>
      <div class="stats-field"><label for="${prefix}-category">类别</label>
        <select id="${prefix}-category" name="category"><option value="">全部类别</option></select></div>
      <div class="stats-field"><label for="${prefix}-min">最低金额（元）</label>
        <input id="${prefix}-min" name="minAmount" type="text" inputmode="decimal" placeholder="不限"
          aria-describedby="${prefix}-hint"></div>
      <div class="stats-field"><label for="${prefix}-max">最高金额（元）</label>
        <input id="${prefix}-max" name="maxAmount" type="text" inputmode="decimal" placeholder="不限"
          aria-describedby="${prefix}-hint"></div>
      <div class="stats-actions"><button type="submit" class="stats-query">查询统计</button>
        <button type="reset" class="stats-reset">清空筛选</button></div>
      <p id="${prefix}-hint" class="stats-hint">筛选可留空；日期与金额范围均包含端点。</p>
    </form>
    <p class="stats-feedback" role="status" aria-live="polite"></p>
    <button type="button" class="stats-retry" hidden>重试当前查询</button>
    <div class="stats-summary" aria-label="收支统计结果">
      <div class="stats-card stats-income-card"><span>总收入（元）</span>
        <output class="stats-income" aria-label="总收入（元）">—</output></div>
      <div class="stats-card stats-expense-card"><span>总支出（元）</span>
        <output class="stats-expense" aria-label="总支出（元）">—</output></div>
      <div class="stats-card stats-balance-card"><span>结余（元）</span>
        <output class="stats-balance" aria-label="结余（元）">—</output></div>
    </div>
    <p class="stats-range">尚未取得统计结果。</p>
    <p class="stats-empty" hidden></p>`;
  container.replaceChildren(wrapper);
  const form = wrapper.querySelector("form");
  const fromInput = form.elements.namedItem("from");
  const toInput = form.elements.namedItem("to");
  const categoryInput = form.elements.namedItem("category");
  const minInput = form.elements.namedItem("minAmount");
  const maxInput = form.elements.namedItem("maxAmount");
  const queryButton = wrapper.querySelector(".stats-query");
  const retryButton = wrapper.querySelector(".stats-retry");
  const feedback = wrapper.querySelector(".stats-feedback");
  const summary = wrapper.querySelector(".stats-summary");
  const range = wrapper.querySelector(".stats-range");
  const empty = wrapper.querySelector(".stats-empty");
  const outputs = ["income", "expense", "balance"].map(key => wrapper.querySelector(".stats-" + key));
  for (const category of statsCategories) {
    const option = document.createElement("option");
    option.value = category;
    option.textContent = category;
    categoryInput.append(option);
  }
  let appliedFilters = {};
  let displayedFilters = null;
  let requestSequence = 0;
  let activeRequest = null;
  let destroyed = false;

  function show(message, state) {
    feedback.textContent = message;
    feedback.dataset.state = state;
  }
  function fieldError(input, message) {
    input.setAttribute("aria-invalid", "true");
    const error = new Error(message);
    error.input = input;
    throw error;
  }
  function readFilters() {
    for (const input of [fromInput, toInput, categoryInput, minInput, maxInput]) input.removeAttribute("aria-invalid");
    const filters = {};
    for (const [key, input] of [["from", fromInput], ["to", toInput]]) {
      if (input.value) {
        if (!statsValidDate(input.value)) fieldError(input, "请填写真实有效的" + (key === "from" ? "起始" : "结束") + "日期。");
        filters[key] = input.value;
      }
    }
    if (filters.from && filters.to && filters.from > filters.to) fieldError(toInput, "结束日期不能早于起始日期。");
    if (categoryInput.value) {
      if (!statsCategories.includes(categoryInput.value)) fieldError(categoryInput, "请选择有效类别。");
      filters.category = categoryInput.value;
    }
    for (const [key, input] of [["minCents", minInput], ["maxCents", maxInput]]) {
      const text = input.value.trim();
      if (!text) continue;
      try { filters[key] = statsParseYuan(text); }
      catch (error) { fieldError(input, error.message); }
    }
    if (filters.minCents !== undefined && filters.maxCents !== undefined &&
        filters.minCents > filters.maxCents) fieldError(maxInput, "最高金额不能小于最低金额。");
    return filters;
  }
  async function load() {
    if (destroyed) return;
    const current = ++requestSequence;
    const filters = {...appliedFilters};
    activeRequest?.abort();
    activeRequest = new AbortController();
    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(filters)) params.set(key, String(value));
    const query = params.toString();
    wrapper.setAttribute("aria-busy", "true");
    summary.dataset.loading = "true";
    queryButton.textContent = "查询中…";
    retryButton.hidden = true;
    show("正在获取统计结果…", "loading");
    try {
      const response = await fetch("/api/stats" + (query ? "?" + query : ""), {signal: activeRequest.signal});
      let result;
      try { result = await response.json(); }
      catch { throw new Error("服务返回了无法识别的统计响应。"); }
      if (!response.ok) throw new Error(typeof result?.error === "string" ? result.error : "统计查询失败。");
      const checked = statsValidateResult(result);
      if (destroyed || current !== requestSequence) return;
      outputs.forEach((output, index) => {
        output.textContent = statsFormatCents(checked[["incomeCents", "expenseCents", "balanceCents"][index]]);
      });
      outputs[2].dataset.sign = checked.balanceCents < 0 ? "negative" : "nonnegative";
      displayedFilters = filters;
      summary.dataset.stale = "false";
      range.textContent = "当前结果范围：" + statsDescribeFilters(filters);
      empty.hidden = checked.incomeCents !== 0 || checked.expenseCents !== 0;
      empty.textContent = Object.keys(filters).length ? "没有匹配这些条件的收支记录。" : "还没有收支记录，记一笔后会自动更新。";
      show("统计已更新。", "success");
    } catch (error) {
      if (destroyed || current !== requestSequence || error.name === "AbortError") return;
      summary.dataset.stale = "true";
      empty.hidden = true;
      if (displayedFilters) range.textContent = "上次结果范围：" + statsDescribeFilters(displayedFilters);
      show((error instanceof TypeError ? "网络连接失败，请检查服务。" : error.message) +
        (displayedFilters ? " 当前保留上次查询结果。" : " 尚未取得有效结果。"), "error");
      retryButton.hidden = false;
    } finally {
      if (!destroyed && current === requestSequence) {
        wrapper.removeAttribute("aria-busy");
        delete summary.dataset.loading;
        queryButton.textContent = "查询统计";
      }
    }
  }
  function submit(event) {
    event.preventDefault();
    try {
      const filters = readFilters();
      // Only explicit, validated queries change applied filters; typing a draft does not.
      appliedFilters = filters;
      void load();
    } catch (error) {
      ++requestSequence;
      activeRequest?.abort();
      wrapper.removeAttribute("aria-busy");
      delete summary.dataset.loading;
      queryButton.textContent = "查询统计";
      retryButton.hidden = true;
      show(error.message, "error");
      error.input?.focus();
    }
  }
  function reset() {
    appliedFilters = {};
    for (const input of [fromInput, toInput, categoryInput, minInput, maxInput]) input.removeAttribute("aria-invalid");
    void load();
  }
  const onChanged = () => { void load(); };
  form.addEventListener("submit", submit);
  form.addEventListener("reset", reset);
  retryButton.addEventListener("click", onChanged);
  window.addEventListener("ledger:changed", onChanged);
  const controller = {
    refresh: load,
    destroy() {
      destroyed = true;
      ++requestSequence;
      activeRequest?.abort();
      window.removeEventListener("ledger:changed", onChanged);
      wrapper.remove();
      statsInstances.delete(container);
    },
  };
  statsInstances.set(container, controller);
  void load();
  return controller;
}

