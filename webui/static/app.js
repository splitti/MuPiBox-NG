'use strict';
const $ = id => document.getElementById(id);
let state, connected=false, pending=false, homeLoaded=false, boxInfo=null, homeSignature='', latestHome=null, networkOnline=true;
let messageTimer;
const uiLocale=(document.documentElement.lang||navigator.language||'de').toLowerCase();
const baseLocale=uiLocale.split('-')[0];
function textFor(values,fallback=''){if(!values)return fallback;return values[uiLocale]||values[baseLocale]||values.de||values.en||Object.values(values)[0]||fallback}
function textForLocale(values,locale,fallback=''){if(!values)return fallback;const base=locale.split('-')[0];return values[locale]||values[base]||values.de||values.en||Object.values(values)[0]||fallback}
function showError(message){$('message').textContent=message;$('message').hidden=false;clearTimeout(messageTimer);messageTimer=setTimeout(()=>{$('message').hidden=true},8000)}
async function request(path,body){const r=await fetch(path,body?{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}:{cache:'no-store'});const data=await r.json();if(!r.ok)throw Error(data.error||'Anfrage fehlgeschlagen');return data}
const clock=s=>`${Math.floor((s||0)/60)}:${String(Math.floor((s||0)%60)).padStart(2,'0')}`;
function updateClock(){const d=new Date();$('clock').textContent=d.toLocaleTimeString(uiLocale,{hour:'2-digit',minute:'2-digit'})}
function setupAdminHold(){
 const target=$('clock');let timer=null;let triggered=false;
 const cancel=()=>{if(timer)clearTimeout(timer);timer=null;target.classList.remove('admin-hold')};
 const start=event=>{
  if(event.type==='pointerdown'&&event.button!==0)return;
  if(timer||triggered)return;
  event.preventDefault();target.classList.add('admin-hold');
  timer=setTimeout(()=>{triggered=true;target.classList.remove('admin-hold');window.location.assign('/admin/')},2000)
 };
 target.addEventListener('pointerdown',start);
 ['pointerup','pointercancel','pointerleave'].forEach(name=>target.addEventListener(name,cancel));
 target.addEventListener('contextmenu',event=>event.preventDefault());
 target.addEventListener('keydown',event=>{if(event.key==='Enter'||event.key===' '){if(!event.repeat)start(event)}});
 target.addEventListener('keyup',event=>{if(event.key==='Enter'||event.key===' ')cancel()});
 target.addEventListener('blur',cancel)
}
function speakCategory(category){
 const tts=boxInfo?.tts;
 if(!tts?.enabled)return;
 const locale=(tts.language||baseLocale||'de').toLowerCase();
 const text=textForLocale(category.labels,locale,category.id);
 if(!text)return;
 if(tts.provider!=='browser-dev'){showError('Der konfigurierte TTS-Provider ist im Backend noch nicht implementiert.');return}
 if(!('speechSynthesis' in window)){showError('Browser-TTS ist in diesem Browser nicht verfügbar.');return}
 window.speechSynthesis.cancel();const utterance=new SpeechSynthesisUtterance(text);utterance.lang=locale;window.speechSynthesis.speak(utterance)
}
function controls(){document.querySelectorAll('[data-action]').forEach(b=>b.disabled=!connected||pending||!state?.queue.length);$('volume').disabled=!connected||pending;$('seek').disabled=!connected||pending||!state?.duration||!['playing','paused'].includes(state?.state);document.querySelectorAll('.media-item').forEach(b=>b.disabled=!connected||pending)}
function render(s){state=s;$('title').textContent=s.queue[s.index]?.title||'Such dir etwas aus';$('album').textContent=s.folder||'Deine Medien warten auf dich.';$('track').textContent=s.queue.length?`${s.index+1} / ${s.queue.length} · ${{playing:'Wiedergabe',paused:'Pausiert',stopped:'Gestoppt',error:'Wiedergabefehler'}[s.state]}`:'';$('position').textContent=clock(s.position);$('duration').textContent=clock(s.duration);if(document.activeElement!==$('seek')){$('seek').max=s.duration||100;$('seek').value=s.position||0}if(document.activeElement!==$('volume')){$('volume').max=s.max_volume;$('volume').value=s.volume}$('volume-label').textContent=s.volume;$('toggle').textContent=s.state==='playing'?'Ⅱ':'▶';$('toggle').setAttribute('aria-label',s.state==='playing'?'Pausieren':'Abspielen');const art=$('art');if(art.dataset.cover!==(s.cover||'')){art.dataset.cover=s.cover||'';art.replaceChildren();if(s.cover){const img=document.createElement('img');img.src=s.cover;img.alt='';art.append(img)}else art.textContent='♫'}document.querySelectorAll('.media-item').forEach(b=>b.setAttribute('aria-pressed',String(b.dataset.folderId===s.folder_id)));if(s.error){$('message').textContent='Wiedergabe fehlgeschlagen: '+s.error;$('message').hidden=false}controls()}
async function command(cmd){if(pending)return;pending=true;controls();try{render(await request('/api/command',cmd))}catch(e){showError(e.message)}finally{pending=false;controls()}}
function mediaItem(item){const b=document.createElement('button');b.className='media-item';b.dataset.id=item.id;b.dataset.kind=item.kind||'';b.setAttribute('aria-pressed','false');if(item.command?.folder_id)b.dataset.folderId=item.command.folder_id;const cover=document.createElement('div');cover.className='cover';if(item.cover){const img=document.createElement('img');img.src=item.cover;img.alt='';img.loading='lazy';cover.append(img)}else cover.textContent='♫';const title=document.createElement('strong');title.textContent=item.title||'Ohne Titel';const detail=document.createElement('small');detail.textContent=item.subtitle||item.kind||'';b.append(cover,title,detail);b.addEventListener('click',()=>{if(item.command)command(item.command)});return b}
function categoryHeading(category){
 const label=textFor(category.labels,category.id);
 if(!boxInfo?.tts?.enabled){const h=document.createElement('h1');h.className='category-title';h.textContent=label;return h}
 const b=document.createElement('button');b.className='category-title category-speak';b.type='button';b.textContent=label;b.setAttribute('aria-label',`${label} vorlesen`);b.addEventListener('click',()=>speakCategory(category));return b
}
function renderHome(home){const root=$('content');root.replaceChildren();let shown=0;for(const category of home?.categories||[]){const items=(category.rows||[]).flatMap(row=>row.items||[]).filter(item=>networkOnline||item.offline_available);if(!items.length)continue;shown++;const section=document.createElement('section');section.className='category';section.dataset.categoryId=category.id;section.append(categoryHeading(category));const media=document.createElement('div');media.className='media-row';media.setAttribute('aria-label',textFor(category.labels,category.id));items.forEach(item=>media.append(mediaItem(item)));section.append(media);root.append(section)}if(!shown){const p=document.createElement('p');p.className='empty-state';p.textContent=networkOnline?'Noch keine Inhalte eingerichtet.':'Offline sind keine lokalen Medien verfügbar.';root.append(p)}}
function applyInfo(info){boxInfo=info;document.documentElement.dataset.uiSize=info?.display?.ui_size||'normal';document.documentElement.dataset.theme=info?.theme||'modern-dark'}
async function refreshHomeAndInfo(){const [home,info]=await Promise.all([request('/api/home'),request('/api/info')]);applyInfo(info);latestHome=home;const signature=JSON.stringify(home);if(signature!==homeSignature){homeSignature=signature;renderHome(home)}homeLoaded=true}
function signalLevel(percent){return percent>=75?4:percent>=50?3:percent>=25?2:percent>0?1:0}
function batteryColor(percent){return percent<=15?'#ef4b5f':percent<=25?'#f2cf4a':percent<=50?'#b7c94b':percent<=75?'#8bd66a':'#43c86a'}
async function updateSystem(){
 try{
  const system=await request('/api/system');
  const wasOnline=networkOnline;networkOnline=system.online!==false;if(wasOnline!==networkOnline&&latestHome)renderHome(latestHome);
  const wifi=system.wifi||{};const wifiEl=$('wifi');const quality=Math.max(0,Math.min(100,Number(wifi.quality_percent)||0));
  wifiEl.dataset.level=wifi.connected?String(signalLevel(quality)):'0';
  wifiEl.dataset.tone=quality>=50?'good':quality>=25?'medium':'weak';
  wifiEl.title=wifi.connected?`${wifi.interface||'WLAN'}: ${quality}% / ${wifi.signal_dbm} dBm`:'Keine WLAN-Verbindung';
  wifiEl.setAttribute('aria-label',wifi.connected?`WLAN-Empfang ${quality} Prozent`:'Keine WLAN-Verbindung');
  const battery=system.battery||{};const batteryEl=$('battery');const percent=Math.max(0,Math.min(100,Number(battery.percent)||0));
  batteryEl.dataset.available=battery.available?'true':'false';batteryEl.dataset.charging=battery.charging?'true':'false';
  batteryEl.style.setProperty('--battery-level',`${percent}%`);batteryEl.style.setProperty('--battery-color',batteryColor(percent));
  batteryEl.title=battery.available?`Akku: ${percent}%${battery.charging?' · lädt':''}`:'Kein Akku erkannt';
  batteryEl.setAttribute('aria-label',batteryEl.title);
 }catch(e){$('wifi').dataset.level='0';$('battery').dataset.available='false'}
}
async function poll(){try{if(!homeLoaded)await refreshHomeAndInfo();const s=await request('/api/status');connected=true;$('connection').textContent=s.backend==='simulated'?'Simulation':'Verbunden';if(!pending)render(s)}catch(e){connected=false;$('connection').textContent='Offline';controls()}finally{setTimeout(poll,750)}}
document.querySelectorAll('[data-action]').forEach(b=>b.addEventListener('click',()=>command({action:b.dataset.action})));
$('volume').addEventListener('change',()=>command({action:'volume',value:Number($('volume').value)}));$('volume').addEventListener('input',()=>{$('volume-label').textContent=$('volume').value});$('seek').addEventListener('change',()=>command({action:'seek',value:Number($('seek').value)}));
updateClock();setInterval(updateClock,30000);setupAdminHold();controls();updateSystem();setInterval(updateSystem,5000);setInterval(()=>refreshHomeAndInfo().catch(()=>{}),2000);poll();
