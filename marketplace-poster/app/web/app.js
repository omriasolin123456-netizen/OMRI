const $ = s => document.querySelector(s);
const $$ = s => [...document.querySelectorAll(s)];
let ads = [], selected = new Set(), settings = null, accounts = [], templates = [], imageItems = [], licenseStatus = null, categories = [], detectedHardware = null;
let lastQueueRunning = false, lastQueueMessage = '', decisionKey = '';
const statusLabels = {ready:'מוכן',publishing:'מפרסם...',published:'פורסם',error:'שגיאה',skipped:'דולג',manual:'ידני',needs_images:'חסרות תמונות',preview:'בדיקה'};

async function api(url, opts={}) {
  const res = await fetch(url, opts);
  const ct = res.headers.get('content-type') || '';
  if (!res.ok) {
    let body;
    if (ct.includes('application/json')) { try { body = await res.json(); } catch(_) {} }
    if (!body) body = (await res.text()).trim();
    const err = new Error(typeof body === 'string' ? body : (body.error || `HTTP ${res.status}`));
    err.status = res.status; err.body = body; throw err;
  }
  if (res.status === 204) return null;
  return ct.includes('application/json') ? res.json() : res.text();
}

function esc(v){return String(v??'').replace(/[&<>'"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));}
function showMsg(sel,text,cls=''){const el=$(sel);if(!el)return;el.textContent=text;el.className='message '+cls;}
function fmtTime(v){if(!v)return '-';try{return new Date(v).toLocaleString('he-IL')}catch(_){return v}}

async function loadCategories(){
  try{categories=await api('/api/catalog/categories');if(!Array.isArray(categories))categories=[];const dl=$('#categoryOptions');if(dl)dl.innerHTML=categories.map(c=>`<option value="${esc(c.label)}">${esc((c.aliases||[]).slice(0,4).join(' · '))}</option>`).join('');syncCategoryFromInput();}catch(_){categories=[]}
}
function findCategoryByInput(v){const q=String(v||'').trim().toLowerCase();if(!q)return null;return categories.find(c=>c.id===q||String(c.label).toLowerCase()===q||(c.aliases||[]).some(a=>String(a).toLowerCase()===q))||categories.find(c=>String(c.label).toLowerCase().includes(q)||(c.aliases||[]).some(a=>String(a).toLowerCase().includes(q)||q.includes(String(a).toLowerCase())))||null}
function syncCategoryFromInput(){const c=findCategoryByInput($('#category')?.value);if(c){$('#categoryId').value=c.id;renderCategoryFields(c)}else{if($('#categoryId'))$('#categoryId').value='';renderCategoryFields(null)}}
function currentCategoryFields(){const out={};$$('#categoryFields [data-field-key]').forEach(el=>{if(String(el.value||'').trim())out[el.dataset.fieldKey]=String(el.value).trim()});return out}
function renderCategoryFields(c,values={}){const box=$('#categoryFields');if(!box)return;if(!c||!(c.fields||[]).length){box.innerHTML='';return}box.innerHTML=(c.fields||[]).map(f=>{const req=f.required?' required':'';if(f.type==='select')return `<label>${esc(f.label)}<select data-field-key="${esc(f.key)}"${req}><option value=""></option>${(f.options||[]).map(o=>`<option ${values[f.key]===o?'selected':''}>${esc(o)}</option>`).join('')}</select></label>`;return `<label>${esc(f.label)}<input data-field-key="${esc(f.key)}" type="${f.type==='number'?'number':'text'}" value="${esc(values[f.key]||'')}"${req}></label>`}).join('')}
function updateSignaturePreview(){const box=$('#signaturePreview');if(!box)return;const use=$('#useSignature')?.checked;if(!use){box.textContent='החתימה כבויה למודעה הזו.';return}let sig=$('#signatureText')?.value||settings?.default_signature_text||'';let phone=settings?.default_phone||'';if(settings?.phone_emoji)phone=[...phone].map(ch=>({'0':'0️⃣','1':'1️⃣','2':'2️⃣','3':'3️⃣','4':'4️⃣','5':'5️⃣','6':'6️⃣','7':'7️⃣','8':'8️⃣','9':'9️⃣'}[ch]||'')).join('');if(sig.includes('{{PHONE}}'))sig=sig.replaceAll('{{PHONE}}',phone),phone='';box.textContent=[sig,phone].filter(Boolean).join(' · ')||'חתימה פעילה — הגדר טקסט/טלפון בהגדרות.'}

function setTab(name){$$('.tab-panel').forEach(x=>x.classList.remove('active'));$$('#tabs button').forEach(x=>x.classList.toggle('active',x.dataset.tab===name));$('#tab-'+name).classList.add('active');if(name==='history')loadHistory();if(name==='templates')loadTemplates();if(name==='accounts')loadAccounts();if(name==='settings')loadSettings();if(name==='ads')loadAds();}
$$('#tabs button').forEach(b=>b.addEventListener('click',()=>setTab(b.dataset.tab)));
$('#category')?.addEventListener('input',syncCategoryFromInput);$('#category')?.addEventListener('blur',syncCategoryFromInput);$('#useSignature')?.addEventListener('change',updateSignaturePreview);$('#signatureText')?.addEventListener('input',updateSignaturePreview);

// ---------- Images ----------
function addFiles(files){
  for(const f of files){ if(imageItems.length>=10) break; if(!f.type.startsWith('image/') && !/\.(heic|heif)$/i.test(f.name)) continue; imageItems.push({file:f,url:URL.createObjectURL(f),rotation:0,coverBlocked:false,id:crypto.randomUUID?.()||Math.random().toString(36)}); }
  renderImagePreview();
}
function renderImagePreview(){
  const box=$('#imagePreview');box.innerHTML='';
  imageItems.forEach((it,i)=>{
    const tile=document.createElement('div');tile.className='image-tile'+(i===0&&!it.coverBlocked?' primary':'')+(it.coverBlocked?' cover-blocked':'');tile.draggable=true;
    tile.innerHTML=`<img src="${it.url}" style="transform:rotate(${it.rotation}deg)" data-viewnew="${i}"><div class="image-actions"><button type="button" data-primary="${i}" ${it.coverBlocked?'disabled':''}>ראשית</button><button type="button" data-rotate="${i}">סובב</button><button type="button" data-remove="${i}">מחק</button><button type="button" data-newblock="${i}">${it.coverBlocked?'אפשר כתמונה ראשית':'לא להשתמש כראשית'}</button></div>${it.coverBlocked?'<span class="cover-note">לא תיבחר כתמונה ראשית</span>':''}`;
    tile.querySelector('[data-primary]').onclick=()=>{if(it.coverBlocked)return;const [x]=imageItems.splice(i,1);imageItems.unshift(x);renderImagePreview();};
    tile.querySelector('[data-rotate]').onclick=()=>{it.rotation=(it.rotation+90)%360;renderImagePreview();};
    tile.querySelector('[data-remove]').onclick=()=>{URL.revokeObjectURL(it.url);imageItems.splice(i,1);renderImagePreview();};
    tile.querySelector('[data-newblock]').onclick=()=>{it.coverBlocked=!it.coverBlocked;renderImagePreview();};
    tile.querySelector('img').onclick=()=>openLightbox(it.url);
    tile.addEventListener('dragstart',e=>e.dataTransfer.setData('text/plain',String(i)));tile.addEventListener('dragover',e=>e.preventDefault());
    tile.addEventListener('drop',e=>{e.preventDefault();const from=+e.dataTransfer.getData('text/plain');const to=i;if(Number.isNaN(from)||from===to)return;const [x]=imageItems.splice(from,1);imageItems.splice(to,0,x);renderImagePreview();});box.appendChild(tile);
  });
}
function openLightbox(url,rotation=0){$('#lightboxImg').src=url;$('#lightboxImg').style.transform=`rotate(${rotation}deg)`;$('#lightbox').classList.remove('hidden');}
async function preparedFile(it,idx){
  if(!it.rotation) return {blob:it.file,name:it.file.name};
  try{
    const bmp=await createImageBitmap(it.file);const c=document.createElement('canvas');const rot=it.rotation%180!==0;c.width=rot?bmp.height:bmp.width;c.height=rot?bmp.width:bmp.height;const ctx=c.getContext('2d');ctx.translate(c.width/2,c.height/2);ctx.rotate(it.rotation*Math.PI/180);ctx.drawImage(bmp,-bmp.width/2,-bmp.height/2);const blob=await new Promise(r=>c.toBlob(r,'image/jpeg',.92));return {blob,name:`rotated-${idx+1}.jpg`};
  }catch(_){return {blob:it.file,name:it.file.name}}
}
const dz=$('#dropZone');dz.addEventListener('click',()=>$('#imagesInput').click());$('#imagesInput').addEventListener('change',e=>addFiles([...e.target.files]));['dragenter','dragover'].forEach(ev=>dz.addEventListener(ev,e=>{e.preventDefault();dz.classList.add('drag')}));['dragleave','drop'].forEach(ev=>dz.addEventListener(ev,e=>{e.preventDefault();dz.classList.remove('drag')}));dz.addEventListener('drop',e=>addFiles([...e.dataTransfer.files]));

// ---------- Ads ----------
async function loadAds(){
  const q=$('#showArchived')?.checked?'?archived=1':'';ads=await api('/api/ads'+q);const valid=new Set(ads.map(a=>a.id));selected=new Set([...selected].filter(id=>valid.has(id)));renderAds();
}
function filteredAds(){
  let out=[...ads];const s=($('#searchAds')?.value||'').trim().toLowerCase();const st=$('#statusFilter')?.value||'';if(s)out=out.filter(a=>`${a.title} ${a.category} ${a.description}`.toLowerCase().includes(s));if(st)out=out.filter(a=>a.status===st);const sort=$('#sortAds')?.value||'new';out.sort((a,b)=>sort==='old'?a.created_at.localeCompare(b.created_at):sort==='priceHigh'?(+b.price||0)-(+a.price||0):sort==='priceLow'?(+a.price||0)-(+b.price||0):b.created_at.localeCompare(a.created_at));return out;
}
function renderAds(){
  if(!$('#adsList'))return;const list=filteredAds();$('#adsCount').textContent=`${list.length} מודעות · ${selected.size} נבחרו`;$('#selectedCount').textContent=`${selected.size} מודעות נבחרו`;
  if(!list.length){$('#adsList').innerHTML='<div class="empty">אין מודעות להצגה.</div>';updateQueueLimit();return}
  $('#adsList').innerHTML=list.map(a=>`<div class="ad-row ${a.archived?'archived':''}"><input class="ad-check" type="checkbox" data-id="${a.id}" ${selected.has(a.id)?'checked':''}><img class="thumb" src="/api/ads/${a.id}/image/0" alt=""><div><div class="ad-title">${esc(a.title)}</div><div class="ad-sub">${esc(a.category)} · ${a.images?.length||0} תמונות · ${a.publish_count||0} פרסומים</div>${a.error?`<div class="error-text">${esc(a.error)}</div>`:''}</div><div class="ad-price">₪${esc(a.price)}</div><div class="ad-condition">${esc(a.condition)}</div><div><span class="status ${a.status}">${statusLabels[a.status]||esc(a.status)}</span><div class="row-actions"><button class="edit" data-edit="${a.id}">ערוך</button><button class="more" data-dup="${a.id}">שכפל</button><button class="more" data-variants="${a.id}">צור גרסאות</button><button class="more" data-pubvariants="${a.id}">פרסם X גרסאות</button><button class="more" data-retry="${a.id}">פרסם</button><button class="more" data-archive="${a.id}">${a.archived?'החזר':'ארכיון'}</button>${a.published_url?`<a class="more" href="${esc(a.published_url)}" target="_blank">פתח</a>`:''}<button class="del" data-del="${a.id}">מחק</button></div></div></div>`).join('');
  $$('.ad-check').forEach(el=>el.onchange=e=>{e.target.checked?selected.add(e.target.dataset.id):selected.delete(e.target.dataset.id);renderAds();});
  $$('[data-edit]').forEach(el=>el.onclick=()=>editAd(el.dataset.edit));
  $$('[data-dup]').forEach(el=>el.onclick=async()=>{try{await api(`/api/ads/${el.dataset.dup}/duplicate`,{method:'POST'});await loadAds()}catch(e){alert(e.message)}});
  $$('[data-variants]').forEach(el=>el.onclick=async()=>{const n=Math.max(1,Math.min(10,+prompt('כמה גרסאות ליצור?','3')||0));if(!n)return;const useAI=!!settings?.image_ai_enabled&&confirm('לנסות גם וריאציית AI לתמונה הראשית אם מנוע התמונה מוכן?');try{const r=await api(`/api/ads/${el.dataset.variants}/variants`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({count:n,use_ai_images:useAI,auto_publish:false})});alert(`נוצרו ${r.ids?.length||0} גרסאות.`);await loadAds()}catch(e){alert(e.message)}});
  $$('[data-pubvariants]').forEach(el=>el.onclick=async()=>{const n=Math.max(1,Math.min(10,+prompt('כמה גרסאות ליצור ולשלוח לתור הפרסום?','3')||0));if(!n)return;const useAI=!!settings?.image_ai_enabled&&confirm('לנסות גם וריאציות AI לתמונות אם המחשב והמנוע מתאימים?');if(!confirm(`ליצור ${n} גרסאות שונות ולהכניס אותן לתור? המגבלות וההשהיות הרגילות נשארות פעילות.`))return;try{const r=await api(`/api/ads/${el.dataset.pubvariants}/variants`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({count:n,use_ai_images:useAI,auto_publish:true,preview:!!settings?.preview_before_publish})});selected=new Set(r.ids||[]);renderAds();setTab('queue')}catch(e){alert(e.message)}});
  $$('[data-retry]').forEach(el=>el.onclick=()=>{selected=new Set([el.dataset.retry]);renderAds();setTab('queue')});
  $$('[data-archive]').forEach(el=>el.onclick=async()=>{const a=ads.find(x=>x.id===el.dataset.archive);await api(`/api/ads/${a.id}`,{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({archived:!a.archived})});await loadAds()});
  $$('[data-del]').forEach(el=>el.onclick=async()=>{if(!confirm('למחוק את המודעה והתמונות המקומיות?'))return;try{await api('/api/ads/'+el.dataset.del,{method:'DELETE'});selected.delete(el.dataset.del);await loadAds()}catch(e){alert(e.message)}});
  updateQueueLimit();
}
function updateQueueLimit(){const n=selected.size;$('#limit').max=Math.max(1,n);if(n&&(+$('#limit').value>n||+$('#limit').value<1))$('#limit').value=n;$('#selectedCount').textContent=`${n} מודעות נבחרו`;}
['searchAds','statusFilter','sortAds'].forEach(id=>$('#'+id)?.addEventListener('input',renderAds));$('#showArchived')?.addEventListener('change',loadAds);$('#selectAllBtn').onclick=()=>{const list=filteredAds();const all=list.length&&list.every(a=>selected.has(a.id));list.forEach(a=>all?selected.delete(a.id):selected.add(a.id));renderAds()};

function resetAdForm(){
  $('#editAdId').value='';$('#adFormTitle').textContent='מודעה חדשה';$('#saveAdBtn').textContent='שמור מודעה';$('#cancelEditBtn').classList.add('hidden');$('#existingImages').innerHTML='';imageItems.forEach(x=>URL.revokeObjectURL(x.url));imageItems=[];renderImagePreview();$('#adForm').reset();applySettingsDefaults();showMsg('#formMsg','');
}
$('#cancelEditBtn').onclick=resetAdForm;
$('#adForm').addEventListener('reset',()=>setTimeout(()=>{if(!$('#editAdId').value){imageItems.forEach(x=>URL.revokeObjectURL(x.url));imageItems=[];renderImagePreview();applySettingsDefaults()}},0));

async function editAd(id){
  const a=await api('/api/ads/'+id);setTab('newad');$('#editAdId').value=id;$('#adFormTitle').textContent='עריכת מודעה';$('#saveAdBtn').textContent='שמור שינויים';$('#cancelEditBtn').classList.remove('hidden');
  $('#title').value=a.title||'';$('#price').value=a.price||'';$('#category').value=a.category||'';$('#categoryId').value=a.category_id||'';renderCategoryFields(categories.find(c=>c.id===(a.category_id||''))||findCategoryByInput(a.category),a.fields||{});$('#condition').value=a.condition||'';$('#description').value=a.description||'';$('#tags').value=(a.tags||[]).join(', ');$('#groupsField').value=(a.groups||[]).join(', ');$('#titleVariants').value=(a.title_variants||[]).join('\n---\n');$('#descriptionVariants').value=(a.description_variants||[]).join('\n---\n');$('#variantMode').value=a.variant_mode||'sequence';const hasVars=(a.title_variants||[]).length||(a.description_variants||[]).length;setVariantSource(hasVars?'manual':'none');$('#imageLimit').value=a.image_limit||10;$('#adAccount').value=a.account_id||'';$('#useSignature').checked=a.use_signature==null?!!settings?.default_signature_enabled:!!a.use_signature;$('#signatureText').value=a.signature_text||'';updateSignaturePreview();
  imageItems.forEach(x=>URL.revokeObjectURL(x.url));imageItems=[];renderImagePreview();renderExistingImages(a);
}
function renderExistingImages(a){
  const blocked=new Set(a.cover_blocked||[]);const box=$('#existingImages');box.innerHTML=(a.images||[]).map((_,i)=>`<div class="image-tile ${a.primary_image===i&&!blocked.has(i)?'primary':''} ${blocked.has(i)?'cover-blocked':''}"><img src="/api/ads/${a.id}/image/${i}" data-view="${i}"><div class="image-actions"><button type="button" data-setprimary="${i}" ${blocked.has(i)?'disabled':''}>ראשית</button><button type="button" data-delimg="${i}">מחק</button><button type="button" data-blockcover="${i}">${blocked.has(i)?'אפשר כתמונה ראשית':'לא להשתמש כראשית'}</button></div>${blocked.has(i)?'<span class="cover-note">לא תיבחר כתמונה ראשית</span>':''}</div>`).join('');
  $$('[data-view]').forEach(x=>x.onclick=()=>openLightbox(`/api/ads/${a.id}/image/${x.dataset.view}`));
  $$('[data-setprimary]').forEach(x=>x.onclick=async()=>{await api(`/api/ads/${a.id}`,{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({primary_image:+x.dataset.setprimary})});renderExistingImages(await api('/api/ads/'+a.id))});
  $$('[data-blockcover]').forEach(x=>x.onclick=async()=>{const idx=+x.dataset.blockcover;const set=new Set(a.cover_blocked||[]);if(set.has(idx))set.delete(idx);else set.add(idx);await api(`/api/ads/${a.id}`,{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({cover_blocked:[...set].sort((m,n)=>m-n)})});renderExistingImages(await api('/api/ads/'+a.id))});
  $$('[data-delimg]').forEach(x=>x.onclick=async()=>{if(!confirm('למחוק את התמונה?'))return;await api(`/api/ads/${a.id}/image/${x.dataset.delimg}`,{method:'DELETE'});renderExistingImages(await api('/api/ads/'+a.id))});
}

$('#adForm').addEventListener('submit',async e=>{
  e.preventDefault();showMsg('#formMsg','שומר...');const editId=$('#editAdId').value;
  try{
    if(editId){
      const patch={title:$('#title').value,price:$('#price').value,category_id:$('#categoryId').value,category:$('#category').value,fields:currentCategoryFields(),condition:$('#condition').value,description:$('#description').value,use_signature:$('#useSignature').checked,signature_text:$('#signatureText').value,tags:splitComma($('#tags').value),groups:splitComma($('#groupsField').value),title_variants:splitVariants($('#titleVariants').value),description_variants:splitVariants($('#descriptionVariants').value),variant_mode:$('#variantMode').value,image_limit:+$('#imageLimit').value||10,account_id:$('#adAccount').value};
      await api('/api/ads/'+editId,{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify(patch)});
      if(imageItems.length){const before=(await api('/api/ads/'+editId)).images?.length||0;const fd=new FormData();for(let i=0;i<imageItems.length;i++){const p=await preparedFile(imageItems[i],i);fd.append('images',p.blob,p.name)}await api(`/api/ads/${editId}/images`,{method:'POST',body:fd});const fresh=await api('/api/ads/'+editId);const blocked=new Set(fresh.cover_blocked||[]);imageItems.forEach((it,i)=>{if(it.coverBlocked)blocked.add(before+i)});await api(`/api/ads/${editId}`,{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({cover_blocked:[...blocked]})});}
      showMsg('#formMsg','השינויים נשמרו.','ok');await loadAds();setTimeout(resetAdForm,500);return;
    }
    const fd=new FormData();['title','price','category','condition','description','tags','groups'].forEach(k=>fd.append(k,$(`[name="${k}"]`).value));fd.append('category_id',$('#categoryId').value);fd.append('fields_json',JSON.stringify(currentCategoryFields()));fd.append('use_signature',$('#useSignature').checked?'1':'0');fd.append('signature_text',$('#signatureText').value);fd.append('title_variants',$('#titleVariants').value);fd.append('description_variants',$('#descriptionVariants').value);fd.append('variant_mode',$('#variantMode').value);fd.append('image_limit',$('#imageLimit').value);fd.append('primary_image','0');fd.append('cover_blocked',imageItems.map((it,i)=>it.coverBlocked?i:null).filter(i=>i!==null).join(','));fd.append('account_id',$('#adAccount').value);
    for(let i=0;i<imageItems.length;i++){const p=await preparedFile(imageItems[i],i);fd.append('images',p.blob,p.name)}
    if(!imageItems.length)throw new Error('בחר לפחות תמונה אחת.');
    try{await api('/api/ads',{method:'POST',body:fd})}catch(err){if(err.status===409&&err.body?.error==='duplicate'&&confirm('נמצאה מודעה זהה. לפרסם/לשמור אותה בכל זאת?')){fd.set('force','1');await api('/api/ads',{method:'POST',body:fd})}else throw err}
    showMsg('#formMsg','המודעה נשמרה.','ok');resetAdForm();await loadAds();
  }catch(err){showMsg('#formMsg',err.message,'error')}
});
function splitComma(s){return s.split(',').map(x=>x.trim()).filter(Boolean)}function splitVariants(s){return s.replace(/\r\n/g,'\n').split(/\n---\n/).map(x=>x.trim()).filter(Boolean)}

// ---------- Bulk ----------
$('#bulkBtn').onclick=()=>{if(!selected.size){alert('בחר מודעות קודם.');return}$('#bulkOverlay').classList.remove('hidden')};$('#applyBulk').onclick=async()=>{const patch={};if($('#bulkPrice').value)patch.price=$('#bulkPrice').value;if($('#bulkCategory').value)patch.category=$('#bulkCategory').value;if($('#bulkCondition').value)patch.condition=$('#bulkCondition').value;if(!Object.keys(patch).length){alert('לא הוזן שינוי.');return}await api('/api/bulk',{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({ids:[...selected],patch})});$('#bulkOverlay').classList.add('hidden');await loadAds()};
$$('[data-close-overlay]').forEach(x=>x.onclick=()=>$('#'+x.dataset.closeOverlay).classList.add('hidden'));
$('#selectAllSavedGroupsBtn').onclick=()=>{const saved=(settings?.favorite_groups||[]).filter(Boolean);if(!saved.length){alert('אין עדיין קבוצות שמורות. הוסף קבוצות תחת הגדרות > משתמש.');return}$('#groupsField').value=saved.join(', ');};

// ---------- Queue ----------
$('#connectBtn').onclick=$('#dashConnectBtn').onclick=async()=>{try{await api('/api/connect',{method:'POST'})}catch(e){alert(e.message)}};
$('#exitBtn').onclick=async()=>{if(!confirm('לסגור את Marketplace Poster?'))return;try{await api('/api/exit',{method:'POST'});document.body.innerHTML='<div style=\"padding:40px;font-family:Arial\">Marketplace Poster נסגר. אפשר לסגור את הטאב.</div>'}catch(e){alert(e.message)}};
$('#publishBtn').onclick=async()=>{if(!selected.size){alert('בחר לפחות מודעה אחת במסך המודעות.');return}const body={ids:[...selected],limit:+$('#limit').value||selected.size,min_delay:+$('#minDelay').value||0,max_delay:+$('#maxDelay').value||0,groups:$('#groups').checked,preview:$('#previewBefore').checked,dry_run:$('#dryRun').checked,account_id:$('#queueAccount').value};try{await api('/api/publish',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})}catch(e){alert(e.message)}};
$('#stopBtn').onclick=async()=>{try{await api('/api/stop',{method:'POST'})}catch(e){alert(e.message)}};$('#resumeBtn').onclick=$('#queueResumeBtn').onclick=async()=>{try{await api('/api/resume',{method:'POST'});setTab('queue')}catch(e){alert(e.message)}};
async function sendDecision(action){try{await api('/api/decision',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({action})});$('#decisionOverlay').classList.add('hidden')}catch(e){alert(e.message)}}
$('#decisionRetry').onclick=()=>sendDecision('retry');$('#decisionPublish').onclick=()=>sendDecision('publish');$('#decisionSkip').onclick=()=>sendDecision('skip');$('#decisionManual').onclick=()=>sendDecision('manual');
function renderDecision(d){
  if(!d){$('#decisionOverlay').classList.add('hidden');decisionKey='';return}const key=d.type+':'+d.ad_id+':'+(d.error||'');if(key!==decisionKey){decisionKey=key;$('#decisionOverlay').classList.remove('hidden')}
  const preview=d.type==='preview';$('#decisionTitle').textContent=preview?'Preview מוכן ב-Chrome':'שגיאה בפרסום';$('#decisionText').textContent=preview?`המודעה "${d.title}" מולאה ב-Chrome. בדוק אותה ובחר מה לעשות.`:`${d.title}: ${d.error}`;$('#decisionPublish').classList.toggle('hidden',!preview);$('#decisionRetry').classList.toggle('hidden',preview);
  if(d.screenshot){$('#decisionScreenshot').src='/api/screenshots/'+encodeURIComponent(d.screenshot);$('#decisionScreenshot').classList.remove('hidden');$('#decisionScreenshot').onclick=()=>openLightbox($('#decisionScreenshot').src)}else{$('#decisionScreenshot').classList.add('hidden')}
  if(d.expires_at){const left=Math.max(0,d.expires_at-Math.floor(Date.now()/1000));$('#decisionTimer').textContent=`דילוג אוטומטי בעוד ${left} שניות`}else $('#decisionTimer').textContent='';
}

// ---------- Templates ----------
async function loadTemplates(){const r=await api('/api/templates');templates=Array.isArray(r)?r:[];renderTemplates()}
function renderTemplates(){const box=$('#templatesList');if(!templates.length){box.innerHTML='<div class="empty">אין תבניות.</div>';return}box.innerHTML=templates.map(t=>`<div class="template-row"><div><b>${esc(t.name)}</b><span class="muted">${esc(t.title||'')}</span></div><div class="actions"><button class="btn ghost small" data-usetpl="${t.id}">השתמש</button><button class="btn danger small" data-deltpl="${t.id}">מחק</button></div></div>`).join('');$$('[data-usetpl]').forEach(x=>x.onclick=()=>useTemplate(x.dataset.usetpl));$$('[data-deltpl]').forEach(x=>x.onclick=async()=>{await api('/api/templates/'+x.dataset.deltpl,{method:'DELETE'});await loadTemplates()})}
$('#templateForm').onsubmit=async e=>{e.preventDefault();const t={name:$('#tplName').value,title:$('#tplTitle').value,description:$('#tplDesc').value,category:$('#tplCategory').value,condition:$('#tplCondition').value,tags:splitComma($('#tplTags').value)};await api('/api/templates',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(t)});e.target.reset();await loadTemplates()};
function useTemplate(id){const t=templates.find(x=>x.id===id);if(!t)return;const texts=[t.title||'',t.description||''].join('\n');const vars=[...new Set([...texts.matchAll(/\{\{([A-Za-z0-9_]+)\}\}/g)].map(m=>m[1]))];const vals={};for(const v of vars){const x=prompt(`ערך עבור ${v}:`,'');if(x===null)return;vals[v]=x}const render=s=>{for(const [k,v] of Object.entries(vals))s=s.replaceAll(`{{${k}}}`,v);return s};resetAdForm();$('#title').value=render(t.title||'');$('#description').value=render(t.description||'');if(t.category)$('#category').value=t.category;if(t.condition)$('#condition').value=t.condition;$('#tags').value=(t.tags||[]).join(', ');setTab('newad')}

// ---------- History ----------
async function loadHistory(){const r=await api('/api/history?limit=500');const h=Array.isArray(r)?r:[];renderHistory(h,'#historyTable');renderHistory(h.slice(0,8),'#dashboardHistory')}
function renderHistory(h,sel){const box=$(sel);if(!box)return;if(!h.length){box.innerHTML='<div class="empty">אין היסטוריה.</div>';return}box.innerHTML=`<table class="table"><thead><tr><th>תאריך</th><th>מודעה</th><th>סטטוס</th><th>גרסת כותרת</th><th>משך</th><th>קישור</th></tr></thead><tbody>${h.map(x=>`<tr><td>${fmtTime(x.started_at)}</td><td>${esc(x.title)}</td><td><span class="status ${x.status}">${statusLabels[x.status]||esc(x.status)}</span>${x.error?`<div class="error-text">${esc(x.error)}</div>`:''}</td><td class="wrap">${esc(x.title_used||'')}</td><td>${x.duration_seconds||0}s</td><td>${x.published_url?`<a href="${esc(x.published_url)}" target="_blank">פתח</a>`:''}${x.screenshot?` <a href="/api/screenshots/${encodeURIComponent(x.screenshot)}" target="_blank">צילום</a>`:''}</td></tr>`).join('')}</tbody></table>`}
$('#refreshHistory').onclick=loadHistory;

// ---------- Accounts ----------
async function loadAccounts(){const data=await api('/api/accounts');accounts=data.accounts||[];renderAccounts(data.active);fillAccountSelects(data.active);renderLicenseStatus(licenseStatus)}
function fillAccountSelects(active){for(const sel of [$('#adAccount'),$('#queueAccount')]){if(!sel)continue;const current=sel.value;sel.innerHTML=accounts.map(a=>`<option value="${a.id}">${esc(a.name)}</option>`).join('');sel.value=current&&accounts.some(a=>a.id===current)?current:(active||accounts[0]?.id||'')}}
function renderAccounts(active){const box=$('#accountsList');if(!box)return;box.innerHTML=accounts.map(a=>`<div class="account-row"><div><b>${esc(a.name)}</b>${a.id===active?'<span class="active-pill">פעיל</span>':''}</div><div class="actions"><button class="btn secondary small" data-connectacc="${a.id}">חבר</button>${a.id!==active?`<button class="btn ghost small" data-activeacc="${a.id}">הפוך לפעיל</button>`:''}<button class="btn danger small" data-delacc="${a.id}">מחק</button></div></div>`).join('');$$('[data-connectacc]').forEach(x=>x.onclick=async()=>{await api(`/api/accounts/${x.dataset.connectacc}/connect`,{method:'POST'})});$$('[data-activeacc]').forEach(x=>x.onclick=async()=>{await api(`/api/accounts/${x.dataset.activeacc}/activate`,{method:'POST'});await loadAccounts();await loadSettings()});$$('[data-delacc]').forEach(x=>x.onclick=async()=>{if(!confirm('למחוק את החשבון המקומי ואת Chrome Profile שלו?'))return;try{await api('/api/accounts/'+x.dataset.delacc,{method:'DELETE'});await loadAccounts()}catch(e){alert(e.message)}})}
$('#accountForm').onsubmit=async e=>{e.preventDefault();try{await api('/api/accounts',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:$('#accountName').value})});e.target.reset();await loadAccounts()}catch(err){if(err.body?.error==='account_limit')alert(`התוכנית שלך מאפשרת עד ${err.body.max_accounts} חשבונות Facebook.`);else alert(err.message)}};

// ---------- Settings ----------
async function loadSettings(){settings=await api('/api/settings');$('#setDailyLimit').value=settings.daily_limit;$('#setRunLimit').value=settings.run_limit;$('#setMaxErrors').value=settings.max_errors;$('#setDecisionTimeout').value=settings.decision_timeout;$('#setMinDelay').value=settings.min_delay;$('#setMaxDelay').value=settings.max_delay;$('#setMaxGroups').value=settings.max_groups;$('#setImageLimit').value=settings.default_image_limit;$('#setAutoSkip').checked=!!settings.auto_skip_timeout;$('#setPreview').checked=!!settings.preview_before_publish;$('#setGroups').checked=!!settings.post_to_groups;$('#setFavoriteGroups').value=(settings.favorite_groups||[]).join(', ');$('#setCategory').value=settings.default_category||'';$('#setCondition').value=settings.default_condition||'חדש';$('#setVariantMode').value=settings.variant_mode||'sequence';$('#setLocalAIEnabled').checked=settings.local_ai_enabled!==false;$('#setLocalAIAutoStart').checked=settings.local_ai_auto_start!==false;$('#setLocalAIBaseURL').value=settings.local_ai_base_url||'http://127.0.0.1:12345';$('#setLocalAIModel').value=settings.local_ai_model||'Qwen3-4B-Q4_K_M.gguf';$('#setAICreativeMode').value=settings.ai_creative_mode||'creative';$('#setAIHebrewOnly').checked=settings.ai_hebrew_only!==false;$('#setAIVariantCount').value=settings.ai_variant_count||5;$('#setAutoCreativeText').checked=settings.auto_creative_text!==false;$('#setImageShuffle').checked=settings.image_shuffle!==false;$('#setImageRandomCover').checked=settings.image_random_cover!==false;$('#setImageRandomSubset').checked=settings.image_random_subset!==false;$('#setImageMinCount').value=settings.image_min_count||3;$('#setDefaultSignatureEnabled').checked=!!settings.default_signature_enabled;$('#setDefaultSignatureText').value=settings.default_signature_text||'';$('#setDefaultPhone').value=settings.default_phone||'';$('#setPhoneEmoji').checked=!!settings.phone_emoji;$('#setAIModelMode').value=settings.ai_model_mode||'auto';$('#setImageAIEnabled').checked=!!settings.image_ai_enabled;$('#setImageAIRuntime').value=settings.image_ai_runtime_path||'';$('#setImageAIModel').value=settings.image_ai_model_path||'';$('#setImageAIVAE').value=settings.image_ai_vae_path||'';$('#setImageAILLM').value=settings.image_ai_llm_path||'';$('#setImageAILLMVision').value=settings.image_ai_llm_vision_path||'';$('#setGeminiEnabled').checked=!!settings.gemini_enabled;$('#setGeminiKey').value=settings.gemini_api_key||'';$('#setGeminiModel').value=settings.gemini_model||'gemini-2.5-flash';$('#setUpdateURL').value=settings.update_manifest_url||'';$('#setCheckUpdatesOnStart').checked=settings.check_updates_on_start!==false;$('#setNotifications').checked=!!settings.notifications;$('#setDark').checked=!!settings.dark_mode;applyDark();applySettingsDefaults();setTimeout(()=>checkAIStatus(true),200);}
function applySettingsDefaults(){if(!settings)return;if(!$('#editAdId').value){$('#category').value=settings.default_category||$('#category').value;$('#condition').value=settings.default_condition||'חדש';$('#variantMode').value=settings.variant_mode||'sequence';$('#imageLimit').value=settings.default_image_limit||10;setVariantSource('none');$('#adAIVariantCount').value=settings.ai_variant_count||5;$('#useSignature').checked=!!settings.default_signature_enabled;$('#signatureText').value='';updateSignaturePreview()}$('#minDelay').value=settings.min_delay??45;$('#maxDelay').value=settings.max_delay??180;$('#previewBefore').checked=!!settings.preview_before_publish;$('#groups').checked=!!settings.post_to_groups;}
function applyDark(){document.documentElement.classList.toggle('dark',!!settings?.dark_mode)}
$('#settingsForm').onsubmit=async e=>{e.preventDefault();const s={...settings,daily_limit:+$('#setDailyLimit').value||15,run_limit:+$('#setRunLimit').value||5,max_errors:+$('#setMaxErrors').value||3,decision_timeout:+$('#setDecisionTimeout').value||60,min_delay:+$('#setMinDelay').value||0,max_delay:+$('#setMaxDelay').value||0,max_groups:+$('#setMaxGroups').value||0,default_image_limit:+$('#setImageLimit').value||10,auto_skip_timeout:$('#setAutoSkip').checked,preview_before_publish:$('#setPreview').checked,post_to_groups:$('#setGroups').checked,favorite_groups:splitComma($('#setFavoriteGroups').value),default_category:$('#setCategory').value,default_condition:$('#setCondition').value,variant_mode:$('#setVariantMode').value,local_ai_enabled:$('#setLocalAIEnabled').checked,local_ai_auto_start:$('#setLocalAIAutoStart').checked,local_ai_base_url:$('#setLocalAIBaseURL').value,local_ai_model:$('#setLocalAIModel').value,ai_creative_mode:$('#setAICreativeMode').value,ai_hebrew_only:$('#setAIHebrewOnly').checked,ai_variant_count:+$('#setAIVariantCount').value||5,auto_creative_text:$('#setAutoCreativeText').checked,image_shuffle:$('#setImageShuffle').checked,image_random_cover:$('#setImageRandomCover').checked,image_random_subset:$('#setImageRandomSubset').checked,image_min_count:+$('#setImageMinCount').value||1,default_signature_enabled:$('#setDefaultSignatureEnabled').checked,default_signature_text:$('#setDefaultSignatureText').value,default_phone:$('#setDefaultPhone').value,phone_emoji:$('#setPhoneEmoji').checked,ai_model_mode:$('#setAIModelMode').value,image_ai_enabled:$('#setImageAIEnabled').checked,image_ai_runtime_path:$('#setImageAIRuntime').value,image_ai_model_path:$('#setImageAIModel').value,image_ai_vae_path:$('#setImageAIVAE').value,image_ai_llm_path:$('#setImageAILLM').value,image_ai_llm_vision_path:$('#setImageAILLMVision').value,gemini_enabled:$('#setGeminiEnabled').checked,gemini_api_key:$('#setGeminiKey').value,gemini_model:$('#setGeminiModel').value,update_manifest_url:$('#setUpdateURL').value,check_updates_on_start:$('#setCheckUpdatesOnStart').checked,notifications:$('#setNotifications').checked,dark_mode:$('#setDark').checked};try{settings=await api('/api/settings',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(s)});showMsg('#settingsMsg','ההגדרות נשמרו.','ok');applyDark();applySettingsDefaults();await checkAIStatus(true)}catch(err){showMsg('#settingsMsg',err.message,'error')}};

// ---------- Local AI status ----------
function setAIProblem(mode,st={},detail=''){
  const title=$('#aiProblemTitle'), text=$('#aiProblemText'), details=$('#aiProblemDetails');
  const install=$('#aiProblemInstall'), repair=$('#aiProblemRepair'), retry=$('#aiProblemRetry'), support=$('#aiProblemSupport');
  [install,repair,retry,support].forEach(x=>x?.classList.add('hidden'));
  if(mode==='install'){title.textContent='Smart AI עדיין לא מותקן';text.textContent='אפשר להשתמש ב-Marketplace Poster גם בלי AI. כדי להשתמש בכפתורי ה-AI צריך להתקין את התוסף המקומי.';install?.classList.remove('hidden')}
  else if(mode==='installing'){title.textContent='Smart AI בהתקנה';text.textContent='ההתקנה מתבצעת ברקע. אפשר להמשיך לעבוד בתוכנה ולנסות שוב כשההתקנה תסתיים.'}
  else {title.textContent='נמצאה תקלה ב-Smart AI';text.textContent='ה-AI מותקן, אבל לא הצליח לעלות. נסה Repair AI. אם גם Repair נכשל, פנה לשירות.';repair?.classList.remove('hidden');retry?.classList.remove('hidden');support?.classList.remove('hidden')}
  details.textContent=detail||st.last_error||st.message||'';details.className='message '+(mode==='installing'?'ok':(mode==='install'?'':'error'));
  $('#aiProblemOverlay')?.classList.remove('hidden');
}
async function checkAIStatus(silent=false){
  try{
    const st=await api('/api/ai/status');
    const text=st.running?`AI מקומי פעיל · ${st.model}`:st.installing?'Smart AI מתקין/מתקן את עצמו ברקע — אפשר להמשיך לעבוד':st.state==='install_failed'?'התקנת Smart AI הקודמת נכשלה — מומלץ Repair AI':st.installed?`AI מותקן אך אינו פעיל · ${st.model}`:'Smart AI עדיין לא מותקן. אפשר להתקין עכשיו או מאוחר יותר מההגדרות.';
    showMsg('#aiStatusMsg',text,st.running?'ok':(st.installing?'ok':(st.installed?'':'error')));
    const pill=$('#aiStatePill');if(pill){pill.textContent=st.running?'מוכן':st.installing?'בהתקנה':st.installed?'דורש הפעלה':'לא מותקן';pill.className='status-pill '+(st.running?'ok':(st.installing||st.installed?'warn':'bad'))}
    if($('#repairAIButton'))$('#repairAIButton').classList.toggle('hidden',!(st.repair_available||st.state==='install_failed'));
    if($('#installRecommendedBtn'))$('#installRecommendedBtn').textContent=st.installed?'התקן / אמת Smart AI':'התקן Smart AI';
    return st;
  }catch(e){if(!silent)showMsg('#aiStatusMsg',e.message,'error');return null}
}
async function repairAI(source='#aiStatusMsg'){
  if(!confirm('להפעיל Repair AI? המערכת תתקין מחדש את מנוע ה-AI ותבדוק את המודל. התהליך ירוץ ברקע.'))return false;
  try{await api('/api/ai/repair',{method:'POST'});showMsg(source,'Repair AI התחיל ברקע. אפשר להמשיך לעבוד בתוכנה.','ok');$('#aiProblemOverlay')?.classList.add('hidden');setTimeout(()=>checkAIStatus(true),700);return true}catch(e){showMsg(source,`Repair AI לא הצליח להתחיל: ${e.message}`,'error');$('#aiProblemSupport')?.classList.remove('hidden');return false}
}
async function ensureAIReady(){
  let st=await api('/api/ai/status');
  if(st.running)return true;
  if(st.installing){setAIProblem('installing',st);return false}
  if(!st.installed){setAIProblem(st.state==='install_failed'?'repair':'install',st);return false}
  try{st=await api('/api/ai/start',{method:'POST'});if(st.running)return true}catch(e){setAIProblem('repair',st,e.message);return false}
  setAIProblem('repair',st);return false;
}
$('#checkAIStatusBtn').onclick=()=>checkAIStatus(false);
$('#startAIButton').onclick=async()=>{showMsg('#aiStatusMsg','בודק ומפעיל AI מקומי...');try{const ready=await ensureAIReady();if(ready){const st=await api('/api/ai/status');showMsg('#aiStatusMsg',`AI פעיל · ${st.model}`,'ok')}}catch(e){showMsg('#aiStatusMsg',e.message,'error')}};
$('#repairAIButton')?.addEventListener('click',()=>repairAI('#hardwareMsg'));
$('#aiProblemRepair')?.addEventListener('click',()=>repairAI('#aiProblemMsg'));
$('#aiProblemRetry')?.addEventListener('click',async()=>{showMsg('#aiProblemMsg','מנסה להפעיל את ה-AI...');try{const st=await api('/api/ai/start',{method:'POST'});if(st.running){showMsg('#aiProblemMsg','AI חזר לפעול.','ok');setTimeout(()=>$('#aiProblemOverlay')?.classList.add('hidden'),600)}else setAIProblem('repair',st)}catch(e){showMsg('#aiProblemMsg',e.message,'error');$('#aiProblemSupport')?.classList.remove('hidden')}});
$('#aiProblemInstall')?.addEventListener('click',async()=>{showMsg('#aiProblemMsg','מתחיל התקנת Smart AI ברקע...');try{await api('/api/ai/install',{method:'POST'});showMsg('#aiProblemMsg','ההתקנה התחילה ברקע. אפשר להמשיך לעבוד.','ok');setTimeout(()=>$('#aiProblemOverlay')?.classList.add('hidden'),700)}catch(e){showMsg('#aiProblemMsg',e.message,'error')}});


// ---------- AI Hardware / Model Manager ----------
async function detectAIHardware(silent=false){try{const r=await api('/api/ai/hardware');detectedHardware=r;const h=r.hardware||{};const eff=r.effective||{};if($('#modelManagerSelect'))$('#modelManagerSelect').innerHTML=(h.models||[]).map(m=>`<option value="${m.id}" ${m.id===eff.id?'selected':''}>${esc(m.name)} · ${m.size_gb}GB</option>`).join('');const installed=r.installed||{};if($('#modelManagerList'))$('#modelManagerList').innerHTML=(h.models||[]).map(m=>`<div class="model-row"><b>${esc(m.name)}</b><span>${m.size_gb}GB · RAM מומלץ ${m.recommended_ram_gb}GB · ${installed[m.id]?'מותקן':'לא מותקן'}</span></div>`).join('');showMsg('#hardwareMsg',`RAM ${h.ram_gb||'?'}GB · GPU ${h.gpu||'לא זוהה'} ${h.vram_gb?`(${h.vram_gb}GB VRAM)`:''} · מומלץ: ${(h.models||[]).find(m=>m.id===h.recommended_model)?.name||h.recommended_model}. ${h.reason||''}`,'ok');if($('#imageAIStatusMsg'))showMsg('#imageAIStatusMsg',r.image_ai?.message||'',r.image_ai?.ready?'ok':'');return r}catch(e){if(!silent)showMsg('#hardwareMsg',e.message,'error');return null}}
$('#detectHardwareBtn').onclick=()=>detectAIHardware(false);
$('#installRecommendedBtn').onclick=async()=>{const st=await checkAIStatus(true);if(st?.installed&&st.repair_available){return repairAI('#hardwareMsg')}if(!confirm('להתקין את Smart AI המומלץ למחשב הזה? ההורדה תרוץ ברקע ואפשר להמשיך לעבוד.'))return;try{await api('/api/ai/install',{method:'POST'});showMsg('#hardwareMsg','התקנת Smart AI התחילה ברקע. אפשר להמשיך לעבוד כרגיל.','ok');setTimeout(()=>checkAIStatus(true),500)}catch(e){showMsg('#hardwareMsg',e.message,'error')}};
$('#installSelectedModelBtn').onclick=async()=>{const id=$('#modelManagerSelect').value;if(!id)return;if(!confirm('להתקין את המודל הנבחר?'))return;try{const r=await api('/api/ai/models/install',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({id})});showMsg('#hardwareMsg',`הורדת ${r.model.name} התחילה ברקע.`,'ok')}catch(e){showMsg('#hardwareMsg',e.message,'error')}};
$('#installAllModelsBtn').onclick=async()=>{if(!confirm('להתקין את כל מודלי הטקסט? ההורדה הכוללת היא בערך 35GB.'))return;try{await api('/api/ai/models/install-all',{method:'POST'});showMsg('#hardwareMsg','התקנת כל המודלים התחילה ברקע. הם יורדו אחד אחרי השני.','ok')}catch(e){showMsg('#hardwareMsg',e.message,'error')}};
async function refreshImageAIStatus(){try{const st=await api('/api/ai/image/status');showMsg('#imageAIStatusMsg',st.message||'',st.ready?'ok':(st.eligible?'':'error'));return st}catch(e){showMsg('#imageAIStatusMsg',e.message,'error')}}
$('#refreshImageAIStatusBtn').onclick=refreshImageAIStatus;
$('#installImageAIBtn').onclick=async()=>{const hw=detectedHardware||await detectAIHardware(true);if(hw?.hardware&&!hw.hardware.image_ai_eligible){alert('החומרה שזוהתה אינה מומלצת לחבילת AI תמונה.');return}if(!confirm('להתקין Image AI Pack מקומי? ההורדה היא בערך 13GB ותתבצע ברקע.'))return;try{const st=await api('/api/ai/image/install',{method:'POST'});settings=await api('/api/settings');$('#setImageAIEnabled').checked=!!settings.image_ai_enabled;$('#setImageAIRuntime').value=settings.image_ai_runtime_path||'';$('#setImageAIModel').value=settings.image_ai_model_path||'';$('#setImageAIVAE').value=settings.image_ai_vae_path||'';$('#setImageAILLM').value=settings.image_ai_llm_path||'';$('#setImageAILLMVision').value=settings.image_ai_llm_vision_path||'';showMsg('#imageAIStatusMsg',st.message||'התקנת Image AI Pack התחילה.','ok')}catch(e){showMsg('#imageAIStatusMsg',e.message,'error')}};
$('#modelManagerSelect').onchange=async()=>{if($('#setAIModelMode').value!=='manual')return;try{await api('/api/ai/models/select',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({mode:'manual',id:$('#modelManagerSelect').value})});settings=await api('/api/settings');$('#setLocalAIModel').value=settings.local_ai_model||''}catch(e){alert(e.message)}};
$('#setAIModelMode').onchange=async()=>{if($('#setAIModelMode').value==='auto'){try{await api('/api/ai/models/select',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({mode:'auto'})});settings=await api('/api/settings');await detectAIHardware(true)}catch(e){alert(e.message)}}};

// ---------- Updates ----------
let updateAvailable=false;
async function checkForUpdates(silent=false){if(!silent)showMsg('#updateMsg','בודק עדכונים...');try{const r=await api('/api/update/check');updateAvailable=!!r.update_available;$('#downloadUpdateBtn').disabled=!updateAvailable;if(updateAvailable){showMsg('#updateMsg',`נמצאה גרסה ${r.latest}. ${r.notes||''}`,'ok');if(silent)notify('Marketplace Poster',`עדכון ${r.latest} זמין להתקנה`)}else if(!silent){showMsg('#updateMsg',`אין עדכון חדש. הגרסה הנוכחית היא ${r.current}.`,'')}}catch(e){if(!silent)showMsg('#updateMsg',e.message,'error')}}
$('#checkUpdateBtn').onclick=()=>checkForUpdates(false);
$('#downloadUpdateBtn').onclick=async()=>{if(!updateAvailable)return;if(!confirm('להוריד את העדכון, לאמת SHA-256, לסגור את התוכנה ולהפעיל את הגרסה החדשה?'))return;showMsg('#updateMsg','מוריד ומאמת את העדכון...');try{const r=await api('/api/update/install',{method:'POST'});showMsg('#updateMsg',`גרסה ${r.version} הותקנה. התוכנה מופעלת מחדש...`,'ok');$('#downloadUpdateBtn').disabled=true}catch(e){showMsg('#updateMsg',e.message,'error')}};

// ---------- Local PIN ----------
$('#savePINBtn').onclick=async()=>{const pin=$('#setPIN').value;if(pin.length<4){alert('PIN חייב להכיל לפחות 4 תווים.');return}try{await api('/api/auth/pin',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({pin})});$('#setPIN').value='';alert('ה-PIN נשמר.')}catch(e){alert(e.message)}};
$('#clearPINBtn').onclick=async()=>{if(!confirm('לבטל את נעילת ה-PIN?'))return;try{await api('/api/auth/pin',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({clear:true})});$('#setPIN').value='';alert('ה-PIN בוטל.')}catch(e){alert(e.message)}};

// ---------- AI ----------
async function runAIRequest(mode,text){if(!(await ensureAIReady()))return null;try{return await api('/api/ai',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({mode,text,creativity:settings?.ai_creative_mode||'creative',language:settings?.ai_hebrew_only===false?'auto':'he',count:settings?.ai_variant_count||5})})}catch(e){let st=null;try{st=await api('/api/ai/status')}catch(_){ }setAIProblem('repair',st||{},e.message);return null}}
$$('.ai-btn').forEach(b=>b.onclick=async()=>{const mode=b.dataset.ai;const text=$('#description').value||$('#title').value;if(!text){alert('אין טקסט לעבודה.');return}b.disabled=true;try{const out=await runAIRequest(mode,text);if(!out)return;if(mode==='titles')$('#titleVariants').value=out.text.split('\n').filter(Boolean).join('\n---\n');else if(mode==='tags')$('#tags').value=out.text;else $('#description').value=out.text}finally{b.disabled=false}});
// Clear, explicit AI actions
$('#aiTitlesBtn')?.addEventListener('click',async()=>{const text=($('#title').value+'\n'+$('#description').value).trim();if(!text)return;const b=$('#aiTitlesBtn');b.disabled=true;try{const out=await runAIRequest('titles',text);if(!out)return;$('#titleVariants').value=out.text.split('\n').filter(Boolean).join('\n---\n');setVariantSource('manual')}finally{b.disabled=false}});
$('#aiTagsBtn')?.addEventListener('click',async()=>{const text=($('#title').value+'\n'+$('#description').value+'\nתגיות קיימות: '+$('#tags').value).trim();if(!text)return;const b=$('#aiTagsBtn');b.disabled=true;try{const out=await runAIRequest('tags',text);if(out)$('#tags').value=out.text}finally{b.disabled=false}});

// ---------- Text variants UX ----------
function setVariantSource(mode){
  mode=mode||'none';$('#variantSource').value=mode;
  $$('input[name="variant_source"]').forEach(r=>r.checked=r.value===mode);
  $('#manualVariantsBox')?.classList.toggle('hidden',mode!=='manual');$('#aiVariantsBox')?.classList.toggle('hidden',mode!=='ai');
  if(mode==='none'){ $('#titleVariants').value='';$('#descriptionVariants').value='' }
}
$$('input[name="variant_source"]').forEach(r=>r.addEventListener('change',()=>setVariantSource(r.value)));
$('#generateAdVariantsBtn')?.addEventListener('click',async()=>{
  const text=($('#title').value+'\n'+$('#description').value).trim();if(!text){showMsg('#adVariantMsg','כתוב קודם כותרת או תיאור.','error');return}
  const count=Math.max(2,Math.min(10,+$('#adAIVariantCount').value||5));const mode=$('#aiVariantStyle').value||'variants';const b=$('#generateAdVariantsBtn');b.disabled=true;showMsg('#adVariantMsg','יוצר גרסאות...');
  try{const out=await api('/api/ai',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({mode,text,creativity:settings?.ai_creative_mode||'creative',language:'he',count})});$('#descriptionVariants').value=out.text;$('#manualVariantsBox')?.classList.remove('hidden');showMsg('#adVariantMsg',`נוצרו עד ${count} גרסאות. אפשר לערוך אותן לפני השמירה.`,'ok')}catch(e){let st=null;try{st=await api('/api/ai/status')}catch(_){};setAIProblem(st?.installed?'repair':'install',st||{},e.message);showMsg('#adVariantMsg','Smart AI אינו מוכן כרגע.','error')}finally{b.disabled=false}
});

// ---------- Settings categories ----------
$$('.settings-nav-btn').forEach(btn=>btn.addEventListener('click',()=>{$$('.settings-nav-btn').forEach(x=>x.classList.toggle('active',x===btn));$$('.settings-section').forEach(p=>p.classList.toggle('active',p.dataset.settingsPanel===btn.dataset.settingsSection))}));
// ---------- Smart AI first-run offer ----------
async function startSmartAIBackground(){
  showMsg('#aiInstallMsg','מתחיל התקנה ברקע...');
  try{
    await api('/api/ai/install',{method:'POST'});
    localStorage.setItem('mp-smart-ai-offer-v1','started');
    showMsg('#aiInstallMsg','ההתקנה התחילה ברקע. אפשר להמשיך לעבוד בתוכנה.','ok');
    setTimeout(()=>{$('#aiInstallOverlay')?.classList.add('hidden')},900);
  }catch(e){
    showMsg('#aiInstallMsg',e.message,'error');
  }
}
async function maybeOfferSmartAI(){
  try{
    const st=await checkAIStatus(true);
    if(!st||st.installed||st.installing)return;
    if(localStorage.getItem('mp-smart-ai-offer-v1'))return;
    let hw=null;try{hw=await api('/api/ai/hardware')}catch(_){}
    const h=hw?.hardware||{};const models=h.models||[];const rec=models.find(m=>m.id===h.recommended_model);
    const txt=rec?`המחשב זוהה: RAM ${h.ram_gb||'?'}GB${h.gpu?` · ${h.gpu}`:''}. המודל המומלץ: ${rec.name} (~${rec.size_gb}GB).`:'המערכת תזהה אוטומטית את המודל החזק ביותר שמתאים למחשב.';
    showMsg('#aiInstallHardware',txt,'ok');
    $('#aiInstallOverlay')?.classList.remove('hidden');
  }catch(_){ }
}
$('#aiInstallNowBtn')?.addEventListener('click',startSmartAIBackground);
$('#aiInstallLaterBtn')?.addEventListener('click',()=>{localStorage.setItem('mp-smart-ai-offer-v1','later');$('#aiInstallOverlay')?.classList.add('hidden')});

// ---------- Imports / backup ----------
$('#csvImport').onchange=async e=>{const f=e.target.files[0];if(!f)return;const fd=new FormData();fd.append('file',f);try{const r=await api('/api/csv/import',{method:'POST',body:fd});alert(`יובאו ${r.imported} מודעות`);await loadAds()}catch(err){alert(err.message)}e.target.value=''};
$('#folderImport').onchange=async e=>{const files=[...e.target.files];if(!files.length)return;showMsg('#folderImportMsg',`מעלה ${files.length} קבצים...`);const fd=new FormData();for(const f of files){fd.append('files',f,f.name);fd.append('paths',f.webkitRelativePath||f.name)}try{const r=await api('/api/folder/import',{method:'POST',body:fd});showMsg('#folderImportMsg',`נוצרו ${r.created} מודעות.`, 'ok');await loadAds()}catch(err){showMsg('#folderImportMsg',err.message,'error')}e.target.value=''};
$('#backupBtn').onclick=async()=>{try{const r=await api('/api/backup/create',{method:'POST'});const a=document.createElement('a');a.href=r.download;a.download=r.name;a.click()}catch(e){alert(e.message)}};
$('#restoreBackup').onchange=async e=>{const f=e.target.files[0];if(!f)return;if(!confirm('שחזור יחליף את המודעות וההגדרות הנוכחיות. להמשיך?'))return;const fd=new FormData();fd.append('file',f);try{await api('/api/backup/restore',{method:'POST',body:fd});alert('השחזור הושלם.');location.reload()}catch(err){alert(err.message)}};
$('#openDataBtn').onclick=()=>api('/api/open-data',{method:'POST'}).catch(e=>alert(e.message));

// ---------- State / notifications ----------
async function pollState(){
  try{
    const s=await api('/api/state');$('#versionLabel').textContent='V'+s.version;if(s.license){licenseStatus=s.license;renderLicenseStatus(licenseStatus)};const connected=s.facebook==='connected';for(const dot of [$('#fbDot'),$('#dashFbDot')])dot.className='dot '+s.facebook;$('#fbStatus').textContent=connected?'Facebook מחובר':s.facebook==='connecting'?'ממתין להתחברות ב-Chrome...':s.facebook==='error'?`שגיאה: ${s.facebook_error}`:'Facebook לא מחובר';$('#dashFbText').textContent=$('#fbStatus').textContent;
    const st=s.stats;$('#stTotal').textContent=st.total_ads;$('#stWaiting').textContent=st.waiting;$('#stPublished').textContent=st.published_today;$('#stFailed').textContent=st.failed_today;$('#stSkipped').textContent=st.skipped_today;$('#stAvg').textContent=Math.round(st.avg_seconds||0)+'s';
    const cp=s.checkpoint;const resumable=cp?.active&&cp.next_index<(cp.ids?.length||0)&&!s.queue.running;$('#resumeBtn').disabled=!resumable;$('#queueResumeBtn').disabled=!resumable;$('#resumeInfo').textContent=resumable?`נשמרה הרצה: ${cp.next_index} מתוך ${cp.ids.length} טופלו. אפשר להמשיך.`:'אין הרצה שנקטעה.';
    const q=s.queue;const done=q.completed+q.skipped;const pct=q.total?Math.min(100,Math.round(done/q.total*100)):0;$('#progressBar').style.width=pct+'%';$('#queueMessage').textContent=q.message||'מוכן';$('#queueNumbers').textContent=q.total?`הושלמו ${q.completed} · דולגו ${q.skipped} · שגיאות ${q.failed} · מתוך ${q.total}`:'';$('#currentAd').textContent=q.current?`נוכחית: ${q.current}`:'';$('#publishBtn').disabled=!!q.running;$('#stopBtn').disabled=!q.running;renderDecision(q.decision);
    if(lastQueueRunning&&!q.running){notify('Marketplace Poster',q.message||'ההרצה הסתיימה');await loadAds();await loadHistory()}if(q.message!==lastQueueMessage&&/שגיאה|מגבלה/.test(q.message||'')){notify('Marketplace Poster',q.message)}lastQueueRunning=q.running;lastQueueMessage=q.message||'';
  }catch(_){ }
}
function notify(title,body){if(!settings?.notifications)return;if('Notification'in window){if(Notification.permission==='granted')new Notification(title,{body});else if(Notification.permission==='default')Notification.requestPermission().then(p=>{if(p==='granted')new Notification(title,{body})})}}

// ---------- Misc ----------
$('#lightbox').addEventListener('click',e=>{if(e.target===$('#lightbox'))$('#lightbox').classList.add('hidden')});

function renderLicenseStatus(st){
  if(!st)return;licenseStatus=st;const badge=$('#licenseBadge');if(badge){badge.textContent=st.licensed?String(st.plan||'LICENSE').toUpperCase():'ללא רישיון';badge.classList.toggle('license-ok',!!st.licensed)}
  const rem=st.publish_limit>0?` · נותרו ${st.publish_remaining} פעולות`:' ';const exp=st.expires_at?` · עד ${fmtTime(st.expires_at)}`:'';const online=st.online?' · אונליין':(st.licensed?' · Grace '+(st.grace_remaining_hours||0)+'h':'');
  const accLimit=st.max_accounts>0?st.max_accounts:'∞';const txt=st.licensed?`תוכנית ${String(st.plan||'').toUpperCase()} · עד ${accLimit} חשבונות Facebook${exp}${rem}${online}`:`נדרש רישיון או Trial.`;
  showMsg('#licenseSettingsStatus',txt,st.licensed?'ok':'error');showMsg('#accountLimitInfo',st.licensed?`התוכנית מאפשרת ${accLimit} חשבונות Facebook. בשימוש: ${accounts.length}.`:'נדרש רישיון.');
}
async function loadLicense(force=false){try{licenseStatus=await api('/api/license/status'+(force?'?force=1':''));renderLicenseStatus(licenseStatus);return licenseStatus}catch(e){showMsg('#licenseMsg',e.message,'error');return {licensed:false}}}
async function continueAfterLicense(){const st=await api('/api/auth/status');if(st.locked){$('#authOverlay').classList.remove('hidden');return}await initUnlocked()}
async function initUnlocked(){
  await Promise.all([loadSettings(),loadAccounts(),loadAds(),loadTemplates(),loadHistory(),loadCategories()]);applySettingsDefaults();renderLicenseStatus(licenseStatus);setTimeout(()=>detectAIHardware(true),500);setTimeout(()=>maybeOfferSmartAI(),800);if(settings?.check_updates_on_start!==false)setTimeout(()=>checkForUpdates(true),1200);pollState();setInterval(pollState,1000);setInterval(()=>{if($('#tab-ads').classList.contains('active'))loadAds()},8000);
}
async function bootstrap(){
  try{const lic=await loadLicense(true);if(!lic.licensed){$('#licenseOverlay').classList.remove('hidden');return}await continueAfterLicense()}catch(e){showMsg('#licenseMsg','שגיאת אתחול: '+e.message,'error');$('#licenseOverlay').classList.remove('hidden')}
}
$('#licenseForm').onsubmit=async e=>{e.preventDefault();showMsg('#licenseMsg','מפעיל רישיון...');try{licenseStatus=await api('/api/license/activate',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({key:$('#licenseKey').value})});renderLicenseStatus(licenseStatus);$('#licenseOverlay').classList.add('hidden');showMsg('#licenseMsg','');await continueAfterLicense()}catch(err){showMsg('#licenseMsg',err.message,'error')}};
$('#startTrialBtn').onclick=async()=>{showMsg('#licenseMsg','מפעיל ניסיון...');try{licenseStatus=await api('/api/license/trial',{method:'POST'});renderLicenseStatus(licenseStatus);$('#licenseOverlay').classList.add('hidden');showMsg('#licenseMsg','');await continueAfterLicense()}catch(err){showMsg('#licenseMsg',err.message,'error')}};
$('#refreshLicenseBtn').onclick=async()=>{const st=await loadLicense(true);renderLicenseStatus(st)};
$('#deactivateLicenseBtn').onclick=async()=>{if(!confirm('לנתק את הרישיון מהמחשב הזה?'))return;try{await api('/api/license/deactivate',{method:'POST'});location.reload()}catch(e){alert(e.message)}};
$('#authForm').onsubmit=async e=>{e.preventDefault();showMsg('#authMsg','בודק...');try{await api('/api/auth/unlock',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({pin:$('#authPIN').value})});$('#authOverlay').classList.add('hidden');showMsg('#authMsg','');await initUnlocked()}catch(err){showMsg('#authMsg',err.message,'error')}};
bootstrap();
