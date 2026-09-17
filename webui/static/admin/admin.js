'use strict';
const $=id=>document.getElementById(id);
let settings=null;
let navigation={categories:[]};
let locale='de';
let translations={};
let idCounter=0;
let localDirectories=[];
let localRoot='/srv/mupibox/music';
let wifiAdapters=[];
let wifiPrimary='';
let wifiSelected='';
let authState={protected:false,authenticated:false};

const providerCatalog={
 'local-library':{label:'source_local',types:['library','path']},
 'resume-list':{label:'source_resume',types:['limit']},
 spotify:{label:'source_spotify',types:['artist','playlist','album','track']},
 'amazon-music':{label:'source_amazon',types:['artist','playlist','album','track']},
 stream:{label:'source_stream',types:['url']},
 podcast:{label:'source_podcast',types:['feed']}
};
const typeKeys={library:'type_library',path:'type_path',limit:'type_limit',artist:'type_artist',playlist:'type_playlist',album:'type_album',track:'type_track',url:'type_url',feed:'type_feed'};

async function request(path,options={}){
 const r=await fetch(path,{...options,headers:{'Content-Type':'application/json',...(options.headers||{})}});
 const data=await r.json().catch(()=>({}));
 if(r.status===401&&path!=='/api/admin/login'){showLogin()}
 if(!r.ok)throw new Error(data.error||('HTTP '+r.status));
 return data
}
function t(key,fallback=''){return translations[key]||fallback||key}
async function setLocale(language){
 const requested=(language||'de').toLowerCase().split('-')[0];
 let selected=requested;
 let response=await fetch('/admin/locales/'+selected+'.json',{cache:'no-store'});
 if(!response.ok){selected='de';response=await fetch('/admin/locales/de.json',{cache:'no-store'})}
 translations=await response.json();
 locale=selected;
 document.documentElement.lang=selected;
 document.querySelectorAll('[data-i18n]').forEach(el=>{el.textContent=t(el.dataset.i18n,el.textContent)});
 document.querySelectorAll('[data-i18n-placeholder]').forEach(el=>{el.placeholder=t(el.dataset.i18nPlaceholder,el.placeholder)});
 if(settings)renderNavigation()
}
function message(text,error=false){const el=$('message');el.textContent=text;el.style.background=error?'#682f45':'#263348';el.hidden=false;setTimeout(()=>el.hidden=true,4000)}
function field(name){return document.querySelector('[name="'+name+'"]')}
function fillSettings(v){
 settings=v;
 field('admin_language').value=v.admin_language||'de';
 field('language').value=v.language;
 field('theme').value=v.theme;
 document.documentElement.dataset.theme=v.theme||'modern-dark';
 field('startup_volume').value=v.audio.startup_volume;
 field('max_volume').value=v.audio.max_volume;
 field('start_sound_enabled').checked=v.audio.start_sound_enabled;
 field('shutdown_sound_enabled').checked=v.audio.shutdown_sound_enabled;
 field('brightness').value=v.display.brightness;
 field('ui_size').value=v.display.ui_size||'normal';
 field('display_idle').value=v.display.idle_off_minutes;
 field('shutdown_idle').value=v.power.idle_shutdown_minutes;
 field('tts_enabled').checked=v.tts.enabled;
 field('tts_language').value=v.tts.language;
 field('tts_provider').value=v.tts.provider;
 field('bluetooth_enabled').checked=!!v.bluetooth?.enabled;
 v.wifi=v.wifi||{primary_interface:'',disabled_interfaces:[]};
 if(!$('content-language').dataset.userSelected)$('content-language').value=v.language||v.tts.language||'de'
}
function readSettings(){return{
 language:field('language').value.trim(),
 admin_language:field('admin_language').value,
 theme:field('theme').value,
 audio:{startup_volume:Number(field('startup_volume').value),max_volume:Number(field('max_volume').value),start_sound_enabled:field('start_sound_enabled').checked,shutdown_sound_enabled:field('shutdown_sound_enabled').checked},
 display:{brightness:Number(field('brightness').value),ui_size:field('ui_size').value,idle_off_minutes:Number(field('display_idle').value)},
 power:{idle_shutdown_minutes:Number(field('shutdown_idle').value)},
 wifi:settings?.wifi||{primary_interface:'',disabled_interfaces:[]},
 bluetooth:{enabled:field('bluetooth_enabled').checked},
 tts:{enabled:field('tts_enabled').checked,language:field('tts_language').value.trim(),provider:field('tts_provider').value.trim()}
}}
function activeContentLanguage(){return($('content-language').value||settings?.language||'de').trim().toLowerCase()}
function labelFor(labels,id){const language=activeContentLanguage();return labels?.[language]||labels?.[language.split('-')[0]]||labels?.de||labels?.en||Object.values(labels||{})[0]||id}
function ensureLabels(node){if(!node.labels)node.labels={};return node.labels}
function labeledInput(value,onchange,label,options={}){
 const l=document.createElement('label');
 const span=document.createElement('span');span.textContent=label;
 const i=document.createElement('input');i.value=value||'';
 if(options.type)i.type=options.type;
 if(options.min!==undefined)i.min=options.min;
 if(options.max!==undefined)i.max=options.max;
 if(options.placeholder)i.placeholder=options.placeholder;
 if(options.maxLength)i.maxLength=options.maxLength;
 i.addEventListener('input',()=>onchange(i.value));
 l.append(span,i);return l
}
function labeledSelect(value,onchange,label,choices){
 const l=document.createElement('label');
 const span=document.createElement('span');span.textContent=label;
 const s=document.createElement('select');
 choices.forEach(choice=>{const o=document.createElement('option');o.value=choice.value;o.textContent=choice.label;s.append(o)});
 s.value=value;
 if(!s.value&&choices.length)s.value=choices[0].value;
 s.addEventListener('change',()=>onchange(s.value));
 l.append(span,s);return l
}
function actions(list,index,remove){
 const a=document.createElement('div');a.className='actions';
 [[t('move_up','↑'),-1],[t('move_down','↓'),1]].forEach(([title,d])=>{const b=document.createElement('button');b.type='button';b.textContent=d<0?'↑':'↓';b.title=title;b.disabled=index+d<0||index+d>=list.length;b.onclick=()=>{[list[index],list[index+d]]=[list[index+d],list[index]];renderNavigation()};a.append(b)});
 const del=document.createElement('button');del.type='button';del.textContent=t('delete','Löschen');del.className='danger';del.onclick=remove;a.append(del);return a
}
function sourceReferenceLabel(type){
 if(type==='path')return t('source_path','Pfad');
 if(type==='url')return t('source_url','Stream-URL');
 if(type==='feed')return t('source_feed','Feed-URL');
 if(type==='limit')return t('resume_limit','Anzahl');
 return t('source_reference','Quelle / ID / Link')
}
function renderMediaSource(row,category,index){
 const box=document.createElement('div');box.className='row';
 const head=document.createElement('div');head.className='row-head';
 const title=document.createElement('strong');title.textContent=labelFor(row.labels,row.id);
 head.append(title,actions(category.rows,index,()=>{category.rows.splice(index,1);renderNavigation()}));
 const grid=document.createElement('div');grid.className='row-grid';
 const language=activeContentLanguage();const labels=ensureLabels(row);
 grid.append(
  labeledInput(labels[language]||'',v=>{labels[language]=v;title.textContent=v||labelFor(labels,row.id)},t('name','Name'),{maxLength:160}),
  labeledInput(row.id,v=>row.id=v,t('technical_id','Technische ID'),{maxLength:64})
 );
 const providerChoices=Object.entries(providerCatalog).map(([value,p])=>({value,label:t(p.label,value)}));
 const providerSelect=labeledSelect(row.provider||'local-library',v=>{row.provider=v;row.source_type=providerCatalog[v].types[0];row.source_ref=v==='resume-list'?'10':'';renderNavigation()},t('media_source','Medienquelle'),providerChoices);
 grid.append(providerSelect);
 const provider=providerCatalog[row.provider]||providerCatalog['local-library'];
 if(!providerCatalog[row.provider])providerChoices.push({value:row.provider,label:row.provider});
 const types=provider.types.map(value=>({value,label:t(typeKeys[value],value)}));
 if(!row.source_type||!provider.types.includes(row.source_type))row.source_type=provider.types[0];
 grid.append(labeledSelect(row.source_type,v=>{row.source_type=v;if(v==='library')row.source_ref='';renderNavigation()},t('source_type','Quelltyp'),types));
 if(row.provider==='local-library'&&row.source_type==='path'){
  const choices=localDirectories.map(directory=>({value:directory.path,label:directory.path}));
  if(row.source_ref&&!choices.some(choice=>choice.value===row.source_ref))choices.unshift({value:row.source_ref,label:row.source_ref});
  if(!row.source_ref&&choices.length)row.source_ref=choices[0].value;
  if(choices.length){grid.append(labeledSelect(row.source_ref||'',v=>row.source_ref=v,t('choose_directory','Ordner auswählen'),choices))}
  else{const info=document.createElement('p');info.className='source-info';info.textContent=t('no_local_directories','Noch keine Unterordner im Medienverzeichnis gefunden.');grid.append(info)}
  const hint=document.createElement('p');hint.className='directory-hint';hint.textContent=localRoot+(row.source_ref?'/'+row.source_ref:'');grid.append(hint)
 }else if(row.provider==='resume-list'){
  if(!/^\d+$/.test(String(row.source_ref||'')))row.source_ref='10';
  grid.append(labeledInput(row.source_ref,v=>row.source_ref=v,t('resume_limit','Anzahl der Einträge'),{type:'number',min:1,max:100}));
  const info=document.createElement('p');info.className='source-info';info.textContent=t('resume_hint','Zeigt die zuletzt begonnenen, noch nicht beendeten Medien.');grid.append(info)
 }else if(row.source_type!=='library'){
  grid.append(labeledInput(row.source_ref||'',v=>row.source_ref=v,sourceReferenceLabel(row.source_type),{maxLength:2048,placeholder:t('source_reference_placeholder','URL, URI, ID oder Pfad')}));
 }else{
  const info=document.createElement('p');info.className='source-info';info.textContent=t('local_library_hint','Verwendet die komplette lokale Medienbibliothek.');grid.append(info)
 }
 box.append(head,grid);return box
}
function renderNavigation(){
 const root=$('categories');root.replaceChildren();
 const language=activeContentLanguage();
 navigation.categories.forEach((category,ci)=>{
  category.rows=category.rows||[];
  const card=document.createElement('section');card.className='category';
  const head=document.createElement('div');head.className='category-head';
  const title=document.createElement('strong');title.textContent=labelFor(category.labels,category.id);
  head.append(title,actions(navigation.categories,ci,()=>{navigation.categories.splice(ci,1);renderNavigation()}));
  const grid=document.createElement('div');grid.className='category-grid';
  const labels=ensureLabels(category);
  grid.append(
   labeledInput(labels[language]||'',v=>{labels[language]=v;title.textContent=v||labelFor(labels,category.id)},t('name','Name'),{maxLength:160}),
   labeledInput(category.id,v=>category.id=v,t('technical_id','Technische ID'),{maxLength:64})
  );
  const media=document.createElement('div');media.className='rows';
  const mediaTitle=document.createElement('h2');mediaTitle.textContent=t('assigned_media','Zugeordnete Medien');media.append(mediaTitle);
  category.rows.forEach((row,ri)=>media.append(renderMediaSource(row,category,ri)));
  const add=document.createElement('button');add.type='button';add.className='add-media';add.textContent=t('add_media','+ Medium hinzufügen');
  add.onclick=()=>{idCounter++;category.rows.push({id:'media-'+Date.now()+'-'+idCounter,labels:{[language]:t('new_media','Neues Medium')},provider:'local-library',source_type:'library',source_ref:''});renderNavigation()};
  media.append(add);card.append(head,grid,media);root.append(card)
 });
}
async function load(){
 try{
  const [s,n,d,w]=await Promise.all([request('/api/admin/settings'),request('/api/admin/navigation'),request('/api/admin/local-directories'),request('/api/connectivity/wifi/adapters').catch(()=>({adapters:[]}))]);
  settings=s;localDirectories=d.directories||[];localRoot=d.root||localRoot;wifiAdapters=w.adapters||[];wifiPrimary=w.primary_interface||s.wifi?.primary_interface||'';wifiSelected=w.selected_interface||'';await setLocale(s.admin_language||'de');fillSettings(s);navigation=n;renderNavigation();renderWifiAdapters();renderPasswordState();$('state').textContent=t('sqlite_connected','SQLite verbunden')
 }catch(error){$('state').textContent=t('error','Fehler');message(error.message,true)}
}
document.querySelectorAll('nav button').forEach(button=>button.onclick=()=>{document.querySelectorAll('nav button').forEach(x=>x.classList.toggle('active',x===button));$('settings-view').hidden=button.dataset.view!=='settings';$('navigation-view').hidden=button.dataset.view!=='navigation'});
field('admin_language').addEventListener('change',()=>setLocale(field('admin_language').value));
field('ui_size').addEventListener('change',()=>$('settings-form').requestSubmit());
field('theme').addEventListener('change',()=>$('settings-form').requestSubmit());
function wifiBars(percent){const level=percent>=75?4:percent>=50?3:percent>=25?2:1;return '▂▄▆█'.slice(0,level)}
function renderWifiAdapters(){
 const select=$('wifi-adapter');const selected=select.value||'auto';select.replaceChildren(new Option(t('wifi_auto_adapter','Automatisch (bevorzugt/aktiv)'),'auto'));
 const root=$('wifi-adapters');root.replaceChildren();
 wifiAdapters.forEach(adapter=>{
  const row=document.createElement('div');row.className='device-row adapter-choice';row.dataset.usable=String(!!adapter.usable);
  const enabled=document.createElement('label');const enabledInput=document.createElement('input');enabledInput.type='checkbox';enabledInput.className='wifi-enabled';enabledInput.dataset.interface=adapter.interface;enabledInput.checked=!!adapter.enabled;const enabledText=document.createElement('span');enabledText.textContent=t('wifi_use_adapter','Verwenden');enabled.append(enabledInput,enabledText);
  const primary=document.createElement('label');const primaryInput=document.createElement('input');primaryInput.type='radio';primaryInput.name='wifi-primary';primaryInput.value=adapter.interface;primaryInput.checked=!!adapter.preferred;primaryInput.disabled=!adapter.enabled||!adapter.usable;const primaryText=document.createElement('span');primaryText.textContent=t('wifi_primary','Bevorzugt');primary.append(primaryInput,primaryText);
  const details=document.createElement('small');const state=adapter.usable?t('wifi_ready','bereit'):t('wifi_not_ready','nicht bereit');const active=adapter.selected?' · '+t('wifi_currently_selected','aktuell gewählt'):'';details.textContent=[adapter.interface,adapter.driver,adapter.mac,adapter.state,state].filter(Boolean).join(' · ')+active;
  enabledInput.onchange=()=>{primaryInput.disabled=!enabledInput.checked||!adapter.usable;if(!enabledInput.checked&&primaryInput.checked){primaryInput.checked=false;const replacement=[...document.querySelectorAll('input[name="wifi-primary"]')].find(input=>!input.disabled);if(replacement)replacement.checked=true}renderWifiScanOptions()};
  row.append(enabled,primary,details);root.append(row)
 });
 renderWifiScanOptions(selected)
}
function renderWifiScanOptions(selected){const select=$('wifi-adapter');const current=selected||select.value||'auto';select.replaceChildren(new Option(t('wifi_auto_adapter','Automatisch (bevorzugt/aktiv)'),'auto'));wifiAdapters.forEach(adapter=>{const enabled=$(`.wifi-enabled[data-interface="${CSS.escape(adapter.interface)}"]`)?.checked??adapter.enabled;if(!enabled||!adapter.usable)return;const details=[adapter.interface,adapter.driver,adapter.state].filter(Boolean).join(' · ');select.append(new Option(details,adapter.interface))});select.value=[...select.options].some(option=>option.value===current)?current:'auto'}
async function saveWifiAdapters(){const disabled=[...document.querySelectorAll('.wifi-enabled')].filter(input=>!input.checked).map(input=>input.dataset.interface);const primary=document.querySelector('input[name="wifi-primary"]:checked')?.value||'';const button=$('save-wifi-adapters');button.disabled=true;try{const data=await request('/api/connectivity/wifi/preferences',{method:'PUT',body:JSON.stringify({primary_interface:primary,disabled_interfaces:disabled})});wifiAdapters=data.adapters||[];wifiPrimary=data.primary_interface||'';wifiSelected=data.selected_interface||'';settings.wifi={primary_interface:wifiPrimary,disabled_interfaces:disabled};renderWifiAdapters();message(t('wifi_preferences_saved','WLAN-Adapterauswahl gespeichert.'))}catch(error){message(error.message,true)}finally{button.disabled=false}}
async function scanWifi(){
 const button=$('scan-wifi');button.disabled=true;const original=button.textContent;button.textContent=t('wifi_scanning','WLAN-Suche läuft …');const adapter=$('wifi-adapter').value||'auto';
 try{const data=await request('/api/connectivity/wifi?interface='+encodeURIComponent(adapter));const select=$('wifi-network');const networks=data.networks||[];select.replaceChildren(new Option('—',''));networks.forEach(network=>{const suffix=network.interface?' · '+network.interface:'';const option=new Option((network.connected?'✓ ':'')+wifiBars(network.signal_percent)+'  '+network.ssid+(network.security?' · '+network.security:'')+suffix,network.ssid);option.dataset.security=network.security||'';option.dataset.interface=network.interface||adapter;select.append(option)});if(networks.length){message(t('wifi_scan_done','WLAN-Suche abgeschlossen.').replace('{count}',networks.length))}else{const empty=new Option(t('wifi_no_networks','Keine WLAN-Netze gefunden.'),'');empty.disabled=true;select.append(empty);message(t('wifi_no_networks','Keine WLAN-Netze gefunden.'),true)}}catch(error){message(error.message,true)}finally{button.disabled=false;button.textContent=original}
}
async function connectWifi(){
 const ssid=$('wifi-network').value;if(!ssid){message(t('choose_wifi','Bitte ein WLAN auswählen.'),true);return}
 const selected=$('wifi-network').selectedOptions[0];const security=selected?.dataset.security||'';const password=$('wifi-password').value;if(security&&security!=='--'&&security.toLowerCase()!=='open'&&password.length<8){message(t('wifi_password_short','Das WLAN-Passwort muss mindestens 8 Zeichen haben.'),true);return}
 const button=$('connect-wifi');button.disabled=true;
 const interfaceName=selected?.dataset.interface||$('wifi-adapter').value||'auto';try{await request('/api/connectivity/wifi/connect',{method:'POST',body:JSON.stringify({ssid,password,interface:interfaceName})});$('wifi-password').value='';message(t('wifi_connected','WLAN-Verbindung wurde eingerichtet.'));setTimeout(scanWifi,1500)}catch(error){message(error.message,true)}finally{button.disabled=false}
}
function renderBluetooth(devices){
 const root=$('bluetooth-devices');root.replaceChildren();
 if(!devices.length){const empty=document.createElement('p');empty.className='hint';empty.textContent=t('no_bluetooth_devices','Keine Geräte gefunden.');root.append(empty);return}
 devices.forEach(device=>{const row=document.createElement('div');row.className='device-row';const info=document.createElement('span');info.textContent=(device.connected?'● ':'')+(device.name||device.address);const actions=document.createElement('div');actions.className='toolbar';
  const action=device.paired?(device.connected?'disconnect':'connect'):'pair';const button=document.createElement('button');button.type='button';button.textContent=t('bluetooth_'+action,action);button.onclick=()=>bluetoothAction(action,device.address);actions.append(button);
  if(device.paired){const remove=document.createElement('button');remove.type='button';remove.className='danger';remove.textContent=t('bluetooth_remove','Entfernen');remove.onclick=()=>bluetoothAction('remove',device.address);actions.append(remove)}
  row.append(info,actions);root.append(row)
 })
}
async function scanBluetooth(){const button=$('scan-bluetooth');button.disabled=true;try{const data=await request('/api/connectivity/bluetooth');renderBluetooth(data.devices||[])}catch(error){message(error.message,true)}finally{button.disabled=false}}
async function bluetoothAction(action,address){try{await request('/api/connectivity/bluetooth/command',{method:'POST',body:JSON.stringify({action,address})});message(t('bluetooth_action_done','Bluetooth-Aktion abgeschlossen.'));setTimeout(scanBluetooth,800)}catch(error){message(error.message,true)}}

function showLogin(){authState.authenticated=false;$('login-view').hidden=false;$('admin-nav').hidden=true;$('admin-main').hidden=true;$('state').textContent=t('login_required','Anmeldung erforderlich');$('login-password').focus()}
function showAdmin(){authState.authenticated=true;$('login-view').hidden=true;$('admin-nav').hidden=false;$('admin-main').hidden=false}
function renderPasswordState(){if(!settings)return;const protectedMode=!!authState.protected;$('password-warning').hidden=protectedMode;$('current-password-label').hidden=!protectedMode;$('logout').hidden=!protectedMode;$('password-state').textContent=protectedMode?t('password_active','Passwortschutz ist aktiv.'):t('password_inactive','Noch kein Admin-Passwort gesetzt.');$('save-password').textContent=t(protectedMode?'change_password':'save_password',protectedMode?'Passwort ändern':'Passwort setzen')}
async function login(event){event.preventDefault();const button=$('login-form').querySelector('button');button.disabled=true;try{await request('/api/admin/login',{method:'POST',body:JSON.stringify({password:$('login-password').value})});$('login-password').value='';authState={protected:true,authenticated:true};showAdmin();await load()}catch(error){message(t('login_failed','Anmeldung fehlgeschlagen.'),true)}finally{button.disabled=false}}
async function savePassword(){const current=$('current-password').value;const next=$('new-password').value;const confirm=$('confirm-password').value;if(next.length<10){message(t('password_too_short','Das Passwort muss mindestens 10 Zeichen enthalten.'),true);return}if(next!==confirm){message(t('password_mismatch','Die neuen Passwörter stimmen nicht überein.'),true);return}const button=$('save-password');button.disabled=true;try{await request('/api/admin/password',{method:'PUT',body:JSON.stringify({current_password:current,new_password:next})});$('current-password').value='';$('new-password').value='';$('confirm-password').value='';authState={protected:true,authenticated:true};renderPasswordState();message(t('password_saved','Admin-Passwort gespeichert.'))}catch(error){message(error.message,true)}finally{button.disabled=false}}
async function logout(){try{await request('/api/admin/logout',{method:'POST'});showLogin()}catch(error){message(error.message,true)}}
async function bootstrap(){await setLocale('de');try{authState=await request('/api/admin/auth');if(authState.protected&&!authState.authenticated){showLogin();return}showAdmin();await load()}catch(error){$('state').textContent=t('error','Fehler');message(error.message,true)}}

$('content-language').addEventListener('change',()=>{$('content-language').dataset.userSelected='true';renderNavigation()});
$('wifi-adapter').addEventListener('change',()=>{$('wifi-network').replaceChildren(new Option('—',''))});
$('save-wifi-adapters').onclick=saveWifiAdapters;
$('scan-wifi').onclick=scanWifi;
$('connect-wifi').onclick=connectWifi;
$('scan-bluetooth').onclick=scanBluetooth;
$('settings-form').addEventListener('submit',async event=>{event.preventDefault();try{const result=await request('/api/admin/settings',{method:'PUT',body:JSON.stringify(readSettings())});fillSettings(result.settings);await setLocale(result.settings.admin_language);message(t('settings_saved','Einstellungen gespeichert.'))}catch(error){message(error.message,true)}});
$('add-category').onclick=()=>{const language=activeContentLanguage();idCounter++;navigation.categories.push({id:'category-'+Date.now()+'-'+idCounter,labels:{[language]:t('new_category','Neue Kategorie')},rows:[]});renderNavigation()};
$('save-navigation').onclick=async()=>{try{navigation=await request('/api/admin/navigation',{method:'PUT',body:JSON.stringify(navigation)});renderNavigation();message(t('content_saved','Inhalte gespeichert und Player aktualisiert.'))}catch(error){message(error.message,true)}};
$('rescan-library').onclick=async()=>{try{const result=await request('/api/admin/library/rescan',{method:'POST'});localDirectories=result.directories||[];renderNavigation();message(t('library_rescanned','Medienordner neu eingelesen.'))}catch(error){message(error.message,true)}};
$('restart-ui').onclick=async()=>{try{await request('/api/admin/ui/restart',{method:'POST'});message(t('ui_restarting','Touch-Oberfläche wird neu gestartet.'))}catch(error){message(error.message,true)}};
$('login-form').addEventListener('submit',login);
$('save-password').onclick=savePassword;
$('logout').onclick=logout;
bootstrap();
