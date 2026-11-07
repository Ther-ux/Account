// Recent records, presented as a readable list. API order is preserved.
const recentInstances=new WeakMap();
function recentMoney(cents){const n=BigInt(cents);return(n/100n)+"."+(n%100n).toString().padStart(2,"0");}
function recentValidate(records){
 if(!Array.isArray(records))throw new Error("记录格式不正确，请重试。");
 for(const record of records)if(!record||typeof record.date!=="string"||!["income","expense"].includes(record.type)||!Number.isSafeInteger(record.amountCents)||record.amountCents<=0||typeof record.category!=="string"||(record.note!==undefined&&typeof record.note!=="string"))throw new Error("记录金额或格式无效，暂时无法显示。");
 return records;
}
export function initRecent(container){
 if(!container||typeof container.replaceChildren!=="function")throw new TypeError("最近记录容器无效");
 if(recentInstances.has(container))return recentInstances.get(container);
 const root=document.createElement("div");root.className="recent-view";
 root.innerHTML=`<div class="recent-toolbar"><label class="recent-search-label"><span class="sr-only">搜索已加载的记录</span><input class="recent-search" type="search" placeholder="搜索这批记录的备注、类别或日期" aria-label="搜索已加载的记录"></label>
 <label class="recent-limit-label">最近<select class="recent-limit" aria-label="显示最近多少笔记录"><option value="10" selected>10 笔</option><option value="20">20 笔</option><option value="100">100 笔</option></select></label>
 <button type="button" class="recent-refresh">刷新</button></div>
 <p class="recent-feedback" role="status" aria-live="polite"></p><div class="recent-content"></div>
 <div class="recent-footer"><a href="#recent" class="recent-more">查看更多记录 <span aria-hidden="true">→</span></a><span class="recent-scope"></span></div>`;
 container.replaceChildren(root);
 const content=root.querySelector(".recent-content"),feedback=root.querySelector(".recent-feedback"),limit=root.querySelector(".recent-limit"),search=root.querySelector(".recent-search"),refresh=root.querySelector(".recent-refresh"),scope=root.querySelector(".recent-scope");
 let records=[],hasResult=false,sequence=0,request=null,destroyed=false;
 function render(){
  content.replaceChildren();const term=search.value.trim().toLowerCase();
  const matching=records.filter(record=>[record.date,record.category,record.note||"",record.type==="income"?"收入":"支出",recentMoney(record.amountCents)].join(" ").toLowerCase().includes(term));
  if(!matching.length){
   const empty=document.createElement("div");empty.className="recent-empty";
   const mark=document.createElement("span");mark.className="recent-empty-mark";mark.setAttribute("aria-hidden","true");mark.textContent="＋";
   const title=document.createElement("strong");title.textContent=records.length?"没有找到匹配记录":"还没有记录";
   const text=document.createElement("p");text.textContent=records.length?"试试类别、日期或其他备注。":"记下第一笔收支，账本就从这里开始。";
   empty.append(mark,title,text);content.append(empty);
  }else{
   const list=document.createElement("ul");list.className="recent-list";list.setAttribute("aria-label","最近收支记录");
   for(const record of matching){
    const item=document.createElement("li");item.className="recent-row recent-"+record.type;
    const icon=document.createElement("span");icon.className="recent-record-icon";icon.setAttribute("aria-hidden","true");icon.textContent=record.type==="income"?"↙":"↗";
    const info=document.createElement("div");info.className="recent-record-info";
    const title=document.createElement("strong");title.className="recent-note";title.textContent=record.note?.trim()||record.category;
    const meta=document.createElement("div");meta.className="recent-record-meta";
    const category=document.createElement("span");category.className="recent-category";category.textContent=record.category;
    const date=document.createElement("time");date.className="recent-date";date.dateTime=record.date;date.textContent=record.date;
    const type=document.createElement("span");type.className="recent-type";type.textContent=record.type==="income"?"收入":"支出";
    meta.append(category,date,type);info.append(title,meta);
    const amount=document.createElement("strong");amount.className="recent-amount";amount.textContent=(record.type==="income"?"+":"−")+recentMoney(record.amountCents);
    item.append(icon,info,amount);list.append(item);
   }content.append(list);
  }
  scope.textContent="已加载最近 "+records.length+" 笔"+(term?" · 找到 "+matching.length+" 笔":"");
 }
 async function load(){
  if(destroyed)return;const current=++sequence;request?.abort();request=new AbortController();
  refresh.disabled=true;root.setAttribute("aria-busy","true");feedback.textContent="正在更新记录…";feedback.dataset.state="loading";
  try{
   const response=await fetch("/api/records/recent?limit="+limit.value,{signal:request.signal});let result;
   try{result=await response.json();}catch{throw new Error("暂时无法读取记录，请重试。");}
   if(!response.ok)throw new Error(typeof result?.error==="string"?result.error:"读取记录失败。");
   result=recentValidate(result);if(destroyed||current!==sequence)return;records=result;hasResult=true;render();
   feedback.textContent="账本已更新";feedback.dataset.state="success";
  }catch(error){if(destroyed||current!==sequence||error.name==="AbortError")return;feedback.textContent=(error instanceof TypeError?"网络连接失败。":error.message)+(hasResult?" 当前保留上次结果。":"")+" 点击刷新可重试。";feedback.dataset.state="error";}
  finally{if(!destroyed&&current===sequence){refresh.disabled=false;root.removeAttribute("aria-busy");}}
 }
 const refreshHandler=()=>void load();
 refresh.addEventListener("click",refreshHandler);limit.addEventListener("change",refreshHandler);search.addEventListener("input",render);window.addEventListener("ledger:changed",refreshHandler);
 const controller={refresh:load,destroy(){destroyed=true;++sequence;request?.abort();window.removeEventListener("ledger:changed",refreshHandler);root.remove();recentInstances.delete(container);}};
 recentInstances.set(container,controller);void load();return controller;
}
