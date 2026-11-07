// Page composition: small everyday flow first, statistics loaded when requested.
const appFeatures=[
 {id:"record-panel",name:"记账",load:()=>import("./record.js"),init:"initRecord"},
 {id:"recent-panel",name:"最近记录",load:()=>import("./recent.js"),init:"initRecent"},
 {id:"stats-panel",name:"收支统计",load:()=>import("./stats.js"),init:"initStats"},
 {id:"insights-panel",name:"账本洞察",load:()=>import("./insights.js"),init:"initInsights"},
];
const appLoads=new Map();
async function appStartFeature(feature){
 if(appLoads.has(feature.id))return appLoads.get(feature.id);
 const task=(async()=>{const container=document.getElementById(feature.id);if(!container)return;
  try{const module=await feature.load();if(typeof module[feature.init]!=="function")throw new Error("Initializer unavailable");return module[feature.init](container);}
  catch(error){const p=document.createElement("p");p.className="app-module-error";p.setAttribute("role","alert");p.textContent=feature.name+"暂时无法加载，请刷新页面重试。";container.replaceChildren(p);console.error("Unable to initialize "+feature.id,error);}
 })();appLoads.set(feature.id,task);return task;
}
function appUpdateNavigation(){
 const current=window.location.hash.slice(1),view=["record","recent","stats"].includes(current)?current:"record";
 document.body.dataset.view=view;
 for(const link of document.querySelectorAll(".app-nav a")){if(link.getAttribute("href")==="#"+view)link.setAttribute("aria-current","page");else link.removeAttribute("aria-current");}
 document.getElementById("record").hidden=view!=="record";
 document.querySelector(".app-workspace").hidden=view==="stats";
 document.getElementById("recent").hidden=view==="stats";
 document.getElementById("stats").hidden=view!=="stats";
 document.getElementById("insights").hidden=view!=="stats";
 document.querySelector(".app-overview").hidden=view==="stats";
 const copy={record:["轻松记好每一笔。","输入金额，选个类别，一键保存就好。"],recent:["每一笔，都有迹可循。","看看最近的收支，也可以搜索已加载的记录。"],stats:["收支变化，心中有数。","一键选时间，复杂筛选按需展开。"]}[view];
 document.getElementById("app-page-title").textContent=copy[0];document.getElementById("app-page-description").textContent=copy[1];
 document.getElementById("app-recent-hint").textContent=view==="record"?"最近 5 笔，按录入顺序显示":"按录入顺序显示，补记旧账也会排在前面";
 if(view==="record"){const search=document.querySelector(".recent-search");if(search?.value){search.value="";search.dispatchEvent(new Event("input"));}}
 if(view==="stats")void(async()=>{await appStartFeature(appFeatures[3]);await appStartFeature(appFeatures[2]);})();
 return view;
}
function appMoney(cents){
 if(!Number.isSafeInteger(cents))throw new Error("账本金额超出安全显示范围。");
 const value=BigInt(cents),n=value<0n?-value:value;
 return(value<0n?"-":"")+(n/100n).toString().replace(/\B(?=(\d{3})+(?!\d))/g,",")+"."+(n%100n).toString().padStart(2,"0");
}
function appSetupOverview(){
 const root=document.querySelector(".app-overview"),feedback=root.querySelector(".app-overview-feedback");
 let sequence=0,request=null,hasResult=false;
 async function refresh(){
  const current=++sequence;request?.abort();request=new AbortController();
  try{const response=await fetch("/api/stats",{signal:request.signal});let result;try{result=await response.json();}catch{throw new Error("概览暂时无法更新。");}
   if(!response.ok)throw new Error(typeof result?.error==="string"?result.error:"概览暂时无法更新。");
   if(!result||!["incomeCents","expenseCents","balanceCents"].every(key=>Number.isSafeInteger(result[key]))||result.incomeCents<0||result.expenseCents<0||BigInt(result.incomeCents)-BigInt(result.expenseCents)!==BigInt(result.balanceCents))throw new Error("概览金额无效或超出安全范围。");
   if(current!==sequence)return;
   for(const key of ["income","expense","balance"])root.querySelector("[data-overview="+key+"]").textContent=appMoney(result[key+"Cents"]);
   root.dataset.stale="false";hasResult=true;feedback.textContent="全部账目 · 已自动更新";feedback.dataset.state="success";
  }catch(error){if(current!==sequence||error.name==="AbortError")return;root.dataset.stale="true";feedback.textContent=(error instanceof TypeError?"网络连接失败。":error.message)+(hasResult?" 当前保留上次概览。":"");feedback.dataset.state="error";}
 }
 window.addEventListener("ledger:changed",()=>void refresh());void refresh();
}
function appStart(){
 document.querySelector(".app-skip-link").addEventListener("click",event=>{event.preventDefault();window.scrollTo({top:0,behavior:"auto"});document.getElementById("main-content").focus({preventScroll:true});});
 document.getElementById("app-today").textContent=new Intl.DateTimeFormat("zh-CN",{month:"long",day:"numeric",weekday:"long"}).format(new Date());
 const help=document.querySelector(".app-help");
 document.getElementById("app-help-open").addEventListener("click",()=>{document.querySelector(".app-more").open=false;help.showModal();});
 help.addEventListener("close",()=>document.querySelector(".app-more>summary").focus());
 appSetupExports();void appReadMode();appSetupOverview();
 void Promise.allSettled(appFeatures.slice(0,2).map(appStartFeature));
 appUpdateNavigation();
 window.addEventListener("hashchange",()=>{if(!["#record","#recent","#stats"].includes(location.hash))return;const view=appUpdateNavigation();window.scrollTo({top:0,behavior:"auto"});document.getElementById("app-page-title").focus({preventScroll:true});if(view==="record")document.querySelector(".record-amount-input")?.focus({preventScroll:true});});
 document.addEventListener("click",event=>{const more=document.querySelector(".app-more");if(more.open&&!more.contains(event.target))more.open=false;});
}
if(document.readyState==="loading")document.addEventListener("DOMContentLoaded",appStart,{once:true});else appStart();

async function appReadMode() {
  try {
    const response = await fetch("/api/meta");
    if (!response.ok) return;
    const meta = await response.json();
    const label = document.getElementById("app-mode");
    if (label) label.textContent = meta.demo ? "演示账本" : "本机账本";
    const banner = document.getElementById("app-demo-notice");
    if (banner) banner.hidden = !meta.demo;
  } catch { /* Feature APIs report their own errors if the service is unavailable. */ }
}

function appSetupExports() {
  const feedback = document.querySelector(".app-export-feedback");
  for (const button of document.querySelectorAll("[data-export]")) {
    button.addEventListener("click", async () => {
      const format = button.dataset.export;
      if (!["json", "csv"].includes(format)) return;
      const label = button.textContent;
      const more = button.closest("details"); if (more) more.open = false;
      button.disabled = true;
      button.textContent = "正在导出…";
      if (feedback) feedback.textContent = "正在准备备份…";
      try {
        const response = await fetch("/api/export?format=" + format);
        if (!response.ok) {
          let message = "导出失败，请稍后重试。";
          try { const error = await response.json(); if (typeof error.error === "string") message = error.error; } catch {}
          throw new Error(message);
        }
        const mime = response.headers.get("Content-Type") ?? "";
        if (!mime.includes(format === "json" ? "application/json" : "text/csv")) throw new Error("导出响应格式无效。");
        const blob = await response.blob();
        const link = document.createElement("a");
        const filename = response.headers.get("Content-Disposition")?.match(/filename="(ledger-[0-9]+\.(?:json|csv))"/)?.[1];
        link.href = URL.createObjectURL(blob);
        link.download = filename ?? "ledger-backup." + format;
        document.body.append(link); link.click(); link.remove();
        setTimeout(() => URL.revokeObjectURL(link.href), 1000);
        if (feedback) { feedback.textContent = format.toUpperCase() + " 文件已下载，请妥善保存。"; feedback.dataset.state = "success"; }
      } catch (error) {
        if (feedback) { feedback.textContent = error instanceof TypeError ? "网络连接失败，暂时无法导出。" : error.message; feedback.dataset.state = "error"; }
      } finally { button.disabled = false; button.textContent = label; }
    });
  }
}
