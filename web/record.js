// Quick entry view. The public initRecord(container) and POST contract are unchanged.
const recordInstances = new WeakMap();
const recordCategories = {expense:["饮食","交通","购物","其他支出"],income:["工资","奖金","其他收入"]};
const recordPaths = {
 "饮食":"M8 3v6m4-6v6M6 3v6a4 4 0 0 0 8 0V3M10 13v8m8-18v18m0-18c4 2 4 9 0 9",
 "交通":"M5 16V6a3 3 0 0 1 3-3h8a3 3 0 0 1 3 3v10M5 16h14M7 19v2m10-2v2M5 10h14M8 14h.01M16 14h.01",
 "购物":"M5 8h14l1 13H4L5 8Zm4 0V6a3 3 0 0 1 6 0v2",
 "其他支出":"M5 12h.01M12 12h.01M19 12h.01",
 "工资":"M4 6h15v14H4a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h13v2m-2 6h6v5h-6v-5Z",
 "奖金":"m12 3 2.5 5 5.5.8-4 3.9.9 5.5-4.9-2.6-4.9 2.6.9-5.5-4-3.9 5.5-.8L12 3Z",
 "其他收入":"M12 5v14M5 12h14M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0Z"
};
let recordSequence=0;
function recordLocalDate(){const d=new Date();return [d.getFullYear(),String(d.getMonth()+1).padStart(2,"0"),String(d.getDate()).padStart(2,"0")].join("-");}
function recordValidDate(value){
 if(!/^\d{4}-\d{2}-\d{2}$/.test(value)||value.startsWith("0000"))return false;
 const [y,m,d]=value.split("-").map(Number),date=new Date(0);date.setHours(0,0,0,0);date.setFullYear(y,m-1,d);
 return date.getFullYear()===y&&date.getMonth()===m-1&&date.getDate()===d;
}
function recordParseAmount(value){
 if(!/^\d+(?:\.\d{1,2})?$/.test(value))throw new Error("请输入大于 0 的金额，最多保留两位小数。");
 const [yuan,fraction=""]=value.split("."),cents=BigInt(yuan)*100n+BigInt(fraction.padEnd(2,"0"));
 if(cents<=0n||cents>BigInt(Number.MAX_SAFE_INTEGER))throw new Error("请输入可安全计算的正金额。");
 return Number(cents);
}
function recordMoney(cents){const n=BigInt(cents);return (n/100n)+"."+(n%100n).toString().padStart(2,"0");}
function recordIcon(category){
 const svg=document.createElementNS("http://www.w3.org/2000/svg","svg");
 for(const [key,value]of Object.entries({viewBox:"0 0 24 24",fill:"none",stroke:"currentColor","stroke-width":"1.6","stroke-linecap":"round","stroke-linejoin":"round","aria-hidden":"true"}))svg.setAttribute(key,value);
 const path=document.createElementNS(svg.namespaceURI,"path");path.setAttribute("d",recordPaths[category]);svg.append(path);return svg;
}
export function initRecord(container){
 if(!container||typeof container.replaceChildren!=="function")throw new TypeError("记账容器无效");
 if(recordInstances.has(container))return recordInstances.get(container);
 const prefix="record-"+(++recordSequence),form=document.createElement("form");
 form.className="record-form";form.noValidate=true;
 form.innerHTML=`
  <fieldset class="record-fields"><legend class="sr-only">记下一笔收支</legend>
   <div class="record-type-switch" role="group" aria-label="收支类型">
    <button type="button" data-type="expense" aria-pressed="true">支出</button>
    <button type="button" data-type="income" aria-pressed="false">收入</button>
   </div><input type="hidden" name="type" value="expense">
   <div class="record-amount-field"><label for="${prefix}-amount">金额 <span>元</span></label>
    <div class="record-amount-wrap"><span aria-hidden="true">¥</span><input id="${prefix}-amount" class="record-amount-input" name="amount" type="text" inputmode="decimal" autocomplete="off" placeholder="0.00" aria-describedby="${prefix}-feedback" required></div>
   </div>
   <fieldset class="record-category-field"><legend>选择类别</legend><div class="record-category-options"></div></fieldset>
   <details class="record-extras"><summary><span>日期与备注</span><span class="record-date-summary">今天</span></summary>
    <div class="record-extras-fields"><div class="record-field"><label for="${prefix}-date">记账日期</label><input id="${prefix}-date" name="date" type="date" required></div>
    <div class="record-field"><label for="${prefix}-note">备注 <span>选填</span></label><textarea id="${prefix}-note" name="note" rows="2" placeholder="例如：和朋友一起吃午饭"></textarea></div></div>
   </details>
   <button type="submit" class="record-submit"><span class="record-submit-label">保存这笔支出</span><span aria-hidden="true">↗</span></button>
   <p class="record-small-print">默认记在今天，保存后自动更新账本</p>
  </fieldset><p id="${prefix}-feedback" class="record-feedback" role="status" aria-live="polite"></p>`;
 container.replaceChildren(form);
 const fields=form.querySelector(".record-fields"),typeInput=form.elements.namedItem("type"),amount=form.elements.namedItem("amount"),date=form.elements.namedItem("date"),note=form.elements.namedItem("note");
 const options=form.querySelector(".record-category-options"),extras=form.querySelector(".record-extras"),feedback=form.querySelector(".record-feedback"),submitLabel=form.querySelector(".record-submit-label");
 const selections={expense:"饮食",income:"工资"};
 let saving=false,destroyed=false,request=null;
 try{const saved=JSON.parse(localStorage.getItem("ledger.record.preferences.v1")||"null");if(saved){for(const type of ["expense","income"])if(recordCategories[type].includes(saved.categories?.[type]))selections[type]=saved.categories[type];if(["expense","income"].includes(saved.type))typeInput.value=saved.type;}}catch{}
 function show(message,state){feedback.textContent=message;feedback.dataset.state=state;}
 function updateDate(){form.querySelector(".record-date-summary").textContent=(date.value===recordLocalDate()?"今天":date.value||"请选择日期")+(note.value.trim()?" · 有备注":"");}
 function renderCategories(){
  const type=typeInput.value;options.replaceChildren();
  recordCategories[type].forEach((category,index)=>{
   const label=document.createElement("label");label.className="record-category-option";
   const input=document.createElement("input");input.type="radio";input.name="category";input.value=category;input.id=prefix+"-category-"+index;input.checked=selections[type]===category;
   const visual=document.createElement("span");visual.className="record-category-visual";
   const text=document.createElement("span");text.textContent=category;visual.append(recordIcon(category),text);label.append(input,visual);options.append(label);
   input.addEventListener("change",()=>{if(input.checked)selections[type]=category;});
  });
  for(const button of form.querySelectorAll("[data-type]"))button.setAttribute("aria-pressed",String(button.dataset.type===type));
  form.dataset.type=type;submitLabel.textContent=type==="income"?"保存这笔收入":"保存这笔支出";
 }
 for(const button of form.querySelectorAll("[data-type]"))button.addEventListener("click",()=>{typeInput.value=button.dataset.type;renderCategories();});
 date.value=recordLocalDate();renderCategories();updateDate();
 date.addEventListener("change",updateDate);note.addEventListener("input",updateDate);
 amount.addEventListener("blur",()=>{if(!amount.value.trim())return;try{recordParseAmount(amount.value.trim());amount.removeAttribute("aria-invalid");}catch(error){amount.setAttribute("aria-invalid","true");show(error.message,"error");}});
 form.addEventListener("submit",async event=>{
  event.preventDefault();if(saving||destroyed)return;
  let cents;
  try{cents=recordParseAmount(amount.value.trim());amount.removeAttribute("aria-invalid");}catch(error){amount.setAttribute("aria-invalid","true");show(error.message,"error");amount.focus();return;}
  if(!recordValidDate(date.value)){extras.open=true;date.setAttribute("aria-invalid","true");show("请选择真实有效的日期。","error");date.focus();return;}
  date.removeAttribute("aria-invalid");
  const category=form.elements.namedItem("category")?.value,type=typeInput.value;
  if(!recordCategories[type]?.includes(category)){show("请先选择一个类别。","error");return;}
  const record={date:date.value,type,amountCents:cents,category,note:note.value};
  saving=true;fields.disabled=true;form.setAttribute("aria-busy","true");submitLabel.textContent="正在保存…";show("正在保存，请稍候。","loading");request=new AbortController();
  let saved=false;
  try{
   const response=await fetch("/api/records",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(record),signal:request.signal});
   let result;try{result=await response.json();}catch{throw new Error("未能确认保存结果，请先核对记录，避免重复记账。");}
   if(response.status!==201)throw new Error(typeof result?.error==="string"?result.error:"保存失败，请重试。");
   if(!result||result.date!==record.date||result.type!==record.type||result.amountCents!==record.amountCents||result.category!==record.category||(result.note??"")!==record.note)throw new Error("未能确认保存结果，请先核对记录。");
   if(destroyed)return;
   amount.value="";note.value="";updateDate();
   try{localStorage.setItem("ledger.record.preferences.v1",JSON.stringify({type,categories:selections}));}catch{}
   show("已记下 "+(type==="income"?"收入 ":"支出 ")+recordMoney(cents)+" 元 · "+category,"success");saved=true;
  }catch(error){if(!destroyed)show(error.name==="AbortError"?"请求已取消，请核对记录后重试。":error instanceof TypeError?"网络连接失败，填写的内容已保留。":error.message,"error");}
  finally{saving=false;if(!destroyed){fields.disabled=false;form.removeAttribute("aria-busy");submitLabel.textContent=typeInput.value==="income"?"保存这笔收入":"保存这笔支出";}}
  if(saved&&!destroyed){window.dispatchEvent(new Event("ledger:changed"));amount.focus();}
 });
 const controller={destroy(){destroyed=true;request?.abort();form.remove();recordInstances.delete(container);}};
 recordInstances.set(container,controller);return controller;
}
