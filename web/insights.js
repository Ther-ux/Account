// Optional enhancements: filtered insights, browser-local monthly budgets and native charts.
const insightInstances = new WeakMap();
let insightID = 0;
const svgNS = "http://www.w3.org/2000/svg";
function insightMoney(cents) {
  if (!Number.isSafeInteger(cents)) throw new Error("金额超出安全显示范围");
  const value=BigInt(cents), absolute=value<0n?-value:value;
  return (value<0n?"-":"")+(absolute/100n)+"."+(absolute%100n).toString().padStart(2,"0");
}
function insightLocalMonth() {
  const now=new Date();return now.getFullYear()+"-"+String(now.getMonth()+1).padStart(2,"0");
}
function insightBudgetCents(text) {
  if (!/^\d+(?:\.\d{1,2})?$/.test(text)) throw new Error("预算须为正数，最多两位小数。");
  const [yuan,fraction=""]=text.split(".");
  const cents=BigInt(yuan)*100n+BigInt(fraction.padEnd(2,"0"));
  if(cents<=0n||cents>BigInt(Number.MAX_SAFE_INTEGER))throw new Error("请输入可安全计算的正预算金额。");
  return Number(cents);
}
function insightSVG(tag,attributes={},text) {
  const element=document.createElementNS(svgNS,tag);
  for(const [key,value] of Object.entries(attributes))element.setAttribute(key,String(value));
  if(text!==undefined)element.textContent=text;
  return element;
}
function insightCheck(result) {
  if(!result||!Number.isSafeInteger(result.count)||result.count<0||!Array.isArray(result.categories)||!Array.isArray(result.trend)||
    !/^\d{4}-\d{2}$/.test(result.month)||!Number.isSafeInteger(result.monthExpenseCents)||result.monthExpenseCents<0)throw new Error("洞察数据格式无效。");
  const totals=result.totals;
  if(!totals||!["incomeCents","expenseCents","balanceCents"].every(k=>Number.isSafeInteger(totals[k]))||
    totals.incomeCents<0||totals.expenseCents<0||BigInt(totals.incomeCents)-BigInt(totals.expenseCents)!==BigInt(totals.balanceCents))throw new Error("洞察金额无效或超出安全范围。");
  let categoryTotal=0n;
  for(const category of result.categories){
    if(typeof category.category!=="string"||!Number.isSafeInteger(category.expenseCents)||category.expenseCents<=0)throw new Error("分类数据无效。");
    categoryTotal+=BigInt(category.expenseCents);
  }
  if(categoryTotal!==BigInt(totals.expenseCents))throw new Error("分类金额与总支出不一致。");
  if(result.trend.length>14)throw new Error("趋势数据格式无效。");
  for(const day of result.trend){
    if(typeof day.date!=="string"||!/^\d{4}-\d{2}-\d{2}$/.test(day.date)||
      !Number.isSafeInteger(day.incomeCents)||!Number.isSafeInteger(day.expenseCents)||day.incomeCents<0||day.expenseCents<0)throw new Error("趋势金额无效。");
  }
  return result;
}
export function initInsights(container) {
  if(!container||typeof container.replaceChildren!=="function")throw new TypeError("洞察容器无效");
  if(insightInstances.has(container))return insightInstances.get(container);
  const prefix="insight-"+(++insightID);
  const root=document.createElement("div");root.className="insights-view";
  root.innerHTML=`
    <p class="insights-feedback" role="status" aria-live="polite">正在整理账本洞察…</p>
    <div class="insights-highlights" aria-label="本次统计摘要"><span class="insights-count">—</span><span class="insights-leading">—</span></div>
    <div class="insights-grid">
      <section class="insights-card"><div class="insights-card-heading"><h3>收支趋势</h3><span>最近 14 个有记录的日期</span></div>
        <div class="insights-legend"><span class="insights-income-key">收入</span><span class="insights-expense-key">支出</span></div>
        <div class="insights-chart-scroll" tabindex="0" role="region" aria-label="收支趋势，可横向滚动"><div class="insights-trend"></div></div>
        <details class="insights-data"><summary>查看图表数据</summary><div class="insights-table-scroll"></div></details>
      </section>
      <section class="insights-card"><div class="insights-card-heading"><h3>支出构成</h3><span>与上方筛选条件一致</span></div><ul class="insights-categories"></ul></section>
    </div>
    <section class="insights-budget-card" aria-labelledby="${prefix}-budget-title">
      <div class="insights-budget-copy"><p class="insights-eyebrow">MONTHLY BUDGET</p><h3 id="${prefix}-budget-title">月度预算提醒</h3>
        <p>按所选月份的全部支出计算，独立于上方筛选。</p>
        <label for="${prefix}-month">预算月份</label><input id="${prefix}-month" class="insights-month" type="month">
      </div>
      <div class="insights-budget-main">
        <form class="insights-budget-form" novalidate><label for="${prefix}-amount">本月支出预算（元）</label>
          <div class="insights-budget-controls"><input id="${prefix}-amount" name="budget" type="text" inputmode="decimal" placeholder="例如 2000.00">
            <button type="submit">保存预算</button><button type="button" class="insights-budget-clear">取消预算</button></div>
        </form>
        <p class="insights-budget-result" role="status" aria-live="polite">尚未设置预算。</p>
        <div class="insights-progress" role="progressbar" aria-label="月度预算使用情况" hidden><span></span></div>
        <p class="insights-budget-meta">预算设置保存在当前浏览器；账目仍由本机文件保存。</p>
        <p class="insights-budget-feedback" role="status" aria-live="polite"></p>
      </div>
    </section>`;
  container.replaceChildren(root);
  const feedback=root.querySelector(".insights-feedback");
  const trend=root.querySelector(".insights-trend");
  const categoryList=root.querySelector(".insights-categories");
  const monthInput=root.querySelector(".insights-month");
  const budgetForm=root.querySelector(".insights-budget-form");
  const budgetInput=budgetForm.elements.namedItem("budget");
  const budgetResult=root.querySelector(".insights-budget-result");
  const budgetFeedback=root.querySelector(".insights-budget-feedback");
  const progress=root.querySelector(".insights-progress");
  let filters={},requestNumber=0,request=null,destroyed=false,monthData=null;
  const budgetFallback=new Map();
  function budgetKey(){return "ledger.budget.v1."+monthInput.value;}
  function readBudget(){
    let raw=budgetFallback.get(budgetKey());
    if(!budgetFallback.has(budgetKey())){try{raw=localStorage.getItem(budgetKey());}catch{}}
    const number=Number(raw);
    return raw&&Number.isSafeInteger(number)&&number>0?number:null;
  }
  function renderBudget(){
    const budget=readBudget();
    const spent=monthData?.month===monthInput.value?monthData.monthExpenseCents:null;
    progress.hidden=!budget||spent===null;
    budgetResult.dataset.state="info";
    if(spent===null){budgetResult.textContent="尚未取得所选月份的支出。";return;}
    if(!budget){budgetResult.textContent="所选月份已支出 "+insightMoney(spent)+" 元，尚未设置预算。";return;}
    const ratio=Math.min(100,spent/budget*100);
    progress.querySelector("span").style.width=ratio+"%";
    progress.setAttribute("aria-valuemin","0");progress.setAttribute("aria-valuemax",String(budget));
    progress.setAttribute("aria-valuenow",String(Math.min(spent,budget)));
    progress.setAttribute("aria-valuetext","已支出 "+insightMoney(spent)+" 元，预算 "+insightMoney(budget)+" 元");
    progress.dataset.state=spent>=budget?"warning":"normal";
    if(spent>budget){budgetResult.textContent="已超出预算 "+insightMoney(spent-budget)+" 元。";budgetResult.dataset.state="error";}
    else if(spent===budget){budgetResult.textContent="本月预算已用完。";budgetResult.dataset.state="error";}
    else{budgetResult.textContent="已支出 "+insightMoney(spent)+" 元，预算还剩 "+insightMoney(budget-spent)+" 元。";}
  }
  function renderTrend(days){
    trend.replaceChildren();
    const scroll=root.querySelector(".insights-table-scroll");scroll.replaceChildren();
    if(!days.length){const p=document.createElement("p");p.className="insights-empty";p.textContent="暂无匹配的收支趋势。";trend.append(p);return;}
    const width=620,height=230,left=18,right=18,top=28,bottom=37;
    const baseline=height-bottom,chartHeight=baseline-top;
    const max=Math.max(...days.flatMap(day=>[day.incomeCents,day.expenseCents]),1);
    const svg=insightSVG("svg",{viewBox:"0 0 "+width+" "+height,role:"img","aria-label":"最近有记录日期的收入和支出柱状图"});
    svg.append(insightSVG("title",{},"收支趋势；精确数值可在下方图表数据中查看"));
    for(let i=0;i<4;i++){const y=top+i*chartHeight/3;svg.append(insightSVG("line",{x1:left,y1:y,x2:width-right,y2:y,class:"insights-grid-line"}));}
    const step=(width-left-right)/days.length;
    days.forEach((day,index)=>{
      const middle=left+step*(index+.5),bar=Math.min(18,step*.28),gap=3;
      for(const [key,color,offset,label] of [["incomeCents","#217b6a",-bar-gap/2,"收入"],["expenseCents","#b87350",gap/2,"支出"]]){
        const pixels=day[key]/max*chartHeight;
        const rect=insightSVG("rect",{x:middle+offset,y:baseline-pixels,width:bar,height:Math.max(day[key]?1:0,pixels),rx:3,fill:color});
        rect.append(insightSVG("title",{},day.date+" "+label+" "+insightMoney(day[key])+" 元"));svg.append(rect);
      }
      svg.append(insightSVG("text",{x:middle,y:height-14,"text-anchor":"middle",class:"insights-axis-label"},day.date.slice(5)));
    });
    trend.append(svg);
    const table=document.createElement("table");table.className="insights-data-table";
    const head=table.createTHead().insertRow();
    for(const label of ["日期","收入（元）","支出（元）"]){const th=document.createElement("th");th.scope="col";th.textContent=label;head.append(th);}
    const body=table.createTBody();
    for(const day of days){const row=body.insertRow();for(const text of [day.date,insightMoney(day.incomeCents),insightMoney(day.expenseCents)]){row.insertCell().textContent=text;}}
    scroll.append(table);
  }
  function renderCategories(result){
    categoryList.replaceChildren();
    if(!result.categories.length){const li=document.createElement("li");li.className="insights-empty";li.textContent="当前条件下没有支出记录。";categoryList.append(li);return;}
    for(const category of result.categories){
      const percent=category.expenseCents/result.totals.expenseCents*100;
      const li=document.createElement("li");
      const heading=document.createElement("div");heading.className="insights-category-heading";
      const label=document.createElement("span");label.textContent=category.category;
      const money=document.createElement("strong");money.textContent=insightMoney(category.expenseCents)+" 元";
      const share=document.createElement("span");share.textContent=percent.toFixed(1)+"%";share.className="insights-share";
      heading.append(label,money,share);
      const track=document.createElement("div");track.className="insights-category-track";track.setAttribute("aria-hidden","true");
      const fill=document.createElement("span");fill.style.width=percent+"%";track.append(fill);
      li.append(heading,track);categoryList.append(li);
    }
  }
  async function load(){
    if(destroyed)return;
    const current=++requestNumber,month=monthInput.value;
    if(!/^\d{4}-\d{2}$/.test(month)||month.startsWith("0000")){feedback.textContent="请选择有效的预算月份。";feedback.dataset.state="error";return;}
    request?.abort();request=new AbortController();
    const params=new URLSearchParams({...filters,month});
    feedback.textContent="正在更新账本洞察…";feedback.dataset.state="loading";
    try{
      const response=await fetch("/api/insights?"+params.toString(),{signal:request.signal});
      let result;try{result=await response.json();}catch{throw new Error("无法识别洞察响应。");}
      if(!response.ok)throw new Error(typeof result?.error==="string"?result.error:"读取洞察失败。");
      result=insightCheck(result);
      if(result.month!==month)throw new Error("返回的预算月份与请求不一致。");
      if(destroyed||current!==requestNumber)return;
      monthData=result;
      root.querySelector(".insights-count").textContent="本次筛选 "+result.count+" 笔账目";
      const leading=result.categories[0];
      root.querySelector(".insights-leading").textContent=leading?"支出最多："+leading.category+" · "+insightMoney(leading.expenseCents)+" 元":"当前条件下暂无支出";
      renderTrend(result.trend);renderCategories(result);renderBudget();
      root.dataset.stale="false";feedback.textContent="洞察已与统计条件同步。";feedback.dataset.state="success";
    }catch(error){
      if(destroyed||current!==requestNumber||error.name==="AbortError")return;
      root.dataset.stale="true";feedback.textContent=(error instanceof TypeError?"网络连接失败。":error.message)+" 请重新查询统计或调整月份重试。";feedback.dataset.state="error";
      renderBudget();
    }
  }
  function onStatsUpdated(event){filters={...(event.detail?.filters??{})};void load();}
  function onStatsError(){root.dataset.stale="true";feedback.textContent="统计查询失败，当前保留上次洞察结果。";feedback.dataset.state="error";}
  function onMonthChanged(){
    monthData=null;
    const stored=readBudget();budgetInput.value=stored?insightMoney(stored):"";
    budgetFeedback.textContent="";renderBudget();void load();
  }
  budgetForm.addEventListener("submit",event=>{
    event.preventDefault();
    try{
      if(!/^\d{4}-\d{2}$/.test(monthInput.value)||monthInput.value.startsWith("0000"))throw new Error("请先选择有效的预算月份。");
      const budget=insightBudgetCents(budgetInput.value.trim());
      budgetFallback.set(budgetKey(),String(budget));
      let persistent=true;try{localStorage.setItem(budgetKey(),String(budget));}catch{persistent=false;}
      budgetInput.removeAttribute("aria-invalid");budgetInput.value=insightMoney(budget);
      budgetFeedback.textContent=persistent?"预算已保存。":"当前浏览器无法保存设置，本次预算仅临时有效。";budgetFeedback.dataset.state=persistent?"success":"error";renderBudget();
    }catch(error){budgetInput.setAttribute("aria-invalid","true");budgetFeedback.textContent=error.message;budgetFeedback.dataset.state="error";budgetInput.focus();}
  });
  root.querySelector(".insights-budget-clear").addEventListener("click",()=>{
    let persistent=true;try{localStorage.removeItem(budgetKey());}catch{persistent=false;}
    budgetFallback.set(budgetKey(),null);budgetInput.value="";budgetFeedback.textContent=persistent?"已取消所选月份的预算。":"本次已取消预算，但浏览器无法保存这一变更。";budgetFeedback.dataset.state=persistent?"success":"error";renderBudget();
  });
  monthInput.addEventListener("change",onMonthChanged);
  window.addEventListener("ledger:stats-updated",onStatsUpdated);
  window.addEventListener("ledger:stats-error",onStatsError);
  monthInput.value=insightLocalMonth();const stored=readBudget();if(stored)budgetInput.value=insightMoney(stored);
  const controller={refresh:load,destroy(){destroyed=true;++requestNumber;request?.abort();window.removeEventListener("ledger:stats-updated",onStatsUpdated);window.removeEventListener("ledger:stats-error",onStatsError);root.remove();insightInstances.delete(container);}};
  insightInstances.set(container,controller);void load();return controller;
}
