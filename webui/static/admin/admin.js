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
let systemStatus=null;
let sambaStatus=null;

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
 const headers=options.body instanceof FormData?{...(options.headers||{})}:{'Content-Type':'application/json',...(options.headers||{})};const r=await fetch(path,{...options,headers});
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
 if(settings){renderNavigation();if(systemStatus)renderSystemStatus()}
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
 v.wifi=v.wifi||{primary_interface:'',disabled_interfaces:[],ipv4:{mode:'dhcp'}};
 v.wifi.ipv4=v.wifi.ipv4||{mode:'dhcp'};
 field('ipv4_mode').value=v.wifi.ipv4.mode||'dhcp';field('ipv4_interface').value=v.wifi.ipv4.interface||'';field('ipv4_address').value=v.wifi.ipv4.address||'';field('ipv4_gateway').value=v.wifi.ipv4.gateway||'';field('ipv4_dns').value=(v.wifi.ipv4.dns||[]).join(', ');toggleStaticIP();
 const spotify=v.providers?.spotify||{},amazon=v.providers?.amazon_music||{};
 field('spotify_enabled').checked=!!spotify.enabled;field('spotify_client_id').value=spotify.client_id||'';field('spotify_client_secret').value=spotify.client_secret||'';field('spotify_country').value=spotify.country||'DE';
 field('amazon_enabled').checked=!!amazon.enabled;field('amazon_client_id').value=amazon.client_id||'';field('amazon_client_secret').value=amazon.client_secret||'';field('amazon_country').value=amazon.country||'DE';
 const samba=v.samba||{};field('samba_enabled').checked=!!samba.enabled;field('samba_mode').value=samba.mode||'guest';field('samba_share_name').value=samba.share_name||'MuPiBox';field('samba_workgroup').value=samba.workgroup||'WORKGROUP';field('samba_password').value='';toggleSamba();renderSambaStatus();
 v.mupihat=v.mupihat||{enabled:false,selected_battery:'USB-C mode (no battery)',current_limit_ma:1790,battery_profiles:[]};field('mupihat_enabled').checked=!!v.mupihat.enabled;field('mupihat_current').value=String(v.mupihat.current_limit_ma||1790);const battery=field('mupihat_battery');battery.replaceChildren(...(v.mupihat.battery_profiles||[]).map(profile=>new Option(profile.name,profile.name)));battery.value=v.mupihat.selected_battery||'';fillBatteryProfile();
 const system=v.system||{};field('swap_policy').value=system.swap_policy||'keep';field('wait_online_policy').value=system.wait_online_policy||'keep';field('performance_mode').value=system.performance_mode||'balanced';field('initial_turbo_seconds').value=Number(system.initial_turbo_seconds||0);
 if(!$('content-language').dataset.userSelected)$('content-language').value=v.language||v.tts.language||'de'
}
function fillBatteryProfile(){const profile=settings?.mupihat?.battery_profiles?.find(item=>item.name===field('mupihat_battery').value);if(!profile)return;field('battery_v100').value=profile.v_100;field('battery_v75').value=profile.v_75;field('battery_v50').value=profile.v_50;field('battery_v25').value=profile.v_25;field('battery_v0').value=profile.v_0;field('battery_warning').value=profile.warning;field('battery_shutdown').value=profile.shutdown}
function readBatteryProfiles(){const profiles=[...(settings?.mupihat?.battery_profiles||[])];const selected=field('mupihat_battery').value;const index=profiles.findIndex(item=>item.name===selected);const updated={name:selected,v_100:Number(field('battery_v100').value),v_75:Number(field('battery_v75').value),v_50:Number(field('battery_v50').value),v_25:Number(field('battery_v25').value),v_0:Number(field('battery_v0').value),warning:Number(field('battery_warning').value),shutdown:Number(field('battery_shutdown').value)};if(index>=0)profiles[index]=updated;else if(selected)profiles.push(updated);return profiles}
function toggleStaticIP(){const active=field('ipv4_mode').value==='static';$('static-ip-fields').hidden=!active;$('static-ip-notice').hidden=!active}
function toggleSamba(){const enabled=field('samba_enabled').checked;field('samba_mode').disabled=!enabled;field('samba_share_name').disabled=!enabled;field('samba_workgroup').disabled=!enabled;field('samba_password').disabled=!enabled||field('samba_mode').value!=='password'}
function renderSambaStatus(){if(!$('samba-state'))return;const state=sambaStatus||{};$('samba-state').textContent=!state.installed?t('samba_not_installed','Samba ist noch nicht installiert. Bitte den DietPi-Installer erneut ausführen.'):state.active?t('samba_active','Freigabe ist aktiv.'):t('samba_inactive','Freigabe ist deaktiviert und startet nicht mit dem System.')}
async function saveSamba(){const button=$('save-samba');button.disabled=true;try{const result=await request('/api/admin/samba',{method:'PUT',body:JSON.stringify({enabled:field('samba_enabled').checked,mode:field('samba_mode').value,share_name:field('samba_share_name').value.trim(),workgroup:field('samba_workgroup').value.trim(),password:field('samba_password').value})});settings.samba=result.settings;sambaStatus=result.runtime;field('samba_password').value='';fillSettings(settings);message(t('samba_saved','Dateifreigabe gespeichert.'))}catch(error){message(error.message,true)}finally{button.disabled=false}}
function readSettings(){const ipv4={mode:field('ipv4_mode').value,interface:field('ipv4_interface').value.trim(),address:field('ipv4_address').value.trim(),gateway:field('ipv4_gateway').value.trim(),dns:field('ipv4_dns').value.split(',').map(value=>value.trim()).filter(Boolean)};return{
 language:field('language').value.trim(),
 admin_language:field('admin_language').value,
 theme:field('theme').value,
 audio:{...(settings?.audio||{}),startup_volume:Number(field('startup_volume').value),max_volume:Number(field('max_volume').value),start_sound_enabled:field('start_sound_enabled').checked,shutdown_sound_enabled:field('shutdown_sound_enabled').checked},
 display:{brightness:Number(field('brightness').value),ui_size:field('ui_size').value,idle_off_minutes:Number(field('display_idle').value)},
 power:{idle_shutdown_minutes:Number(field('shutdown_idle').value)},
 wifi:{...(settings?.wifi||{}),ipv4},
 bluetooth:{enabled:field('bluetooth_enabled').checked},
 tts:{...(settings?.tts||{}),enabled:field('tts_enabled').checked,language:field('tts_language').value.trim(),provider:field('tts_provider').value.trim()},
 providers:{spotify:{enabled:field('spotify_enabled').checked,client_id:field('spotify_client_id').value.trim(),client_secret:field('spotify_client_secret').value,country:field('spotify_country').value.trim().toUpperCase()},amazon_music:{enabled:field('amazon_enabled').checked,client_id:field('amazon_client_id').value.trim(),client_secret:field('amazon_client_secret').value,country:field('amazon_country').value.trim().toUpperCase()}},
 samba:{enabled:field('samba_enabled').checked,mode:field('samba_mode').value,share_name:field('samba_share_name').value.trim(),workgroup:field('samba_workgroup').value.trim()},
 mupihat:{...(settings?.mupihat||{}),enabled:field('mupihat_enabled').checked,selected_battery:field('mupihat_battery').value,current_limit_ma:Number(field('mupihat_current').value),battery_profiles:readBatteryProfiles()},
 system:{swap_policy:field('swap_policy').value,wait_online_policy:field('wait_online_policy').value,performance_mode:field('performance_mode').value,initial_turbo_seconds:Number(field('initial_turbo_seconds').value)}
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
  const [s,n,d,w,sys,samba]=await Promise.all([request('/api/admin/settings'),request('/api/admin/navigation'),request('/api/admin/local-directories'),request('/api/connectivity/wifi/adapters').catch(()=>({adapters:[]})),request('/api/admin/system').catch(()=>null),request('/api/admin/samba').catch(()=>null)]);
  settings=s;systemStatus=sys;sambaStatus=samba?.runtime||null;localDirectories=d.directories||[];localRoot=d.root||localRoot;wifiAdapters=w.adapters||[];wifiPrimary=w.primary_interface||s.wifi?.primary_interface||'';wifiSelected=w.selected_interface||'';await setLocale(s.admin_language||'de');fillSettings(s);navigation=n;renderNavigation();renderWifiAdapters();renderPasswordState();renderSystemStatus();$('state').textContent=t('sqlite_connected','SQLite verbunden')
 }catch(error){$('state').textContent=t('error','Fehler');message(error.message,true)}
}
function showView(name){document.querySelectorAll('#admin-nav button').forEach(button=>button.classList.toggle('active',button.dataset.view===name));document.querySelectorAll('[data-admin-view]').forEach(view=>view.hidden=view.dataset.adminView!==name);if(name==='system'&&!systemStatus)refreshSystem();if(name!=='system')stopScreenshotLive();if(name==='tts')enterTTSView();else leaveTTSView();if(name==='hardware')loadAudioStatus()}
document.querySelectorAll('#admin-nav button').forEach(button=>button.onclick=()=>showView(button.dataset.view));
field('admin_language').addEventListener('change',()=>setLocale(field('admin_language').value));
field('ui_size').addEventListener('change',()=>$('settings-form').requestSubmit());
field('theme').addEventListener('change',()=>$('settings-form').requestSubmit());
field('ipv4_mode').addEventListener('change',toggleStaticIP);
field('samba_enabled').addEventListener('change',toggleSamba);
field('samba_mode').addEventListener('change',toggleSamba);
field('mupihat_battery').addEventListener('change',fillBatteryProfile);
function formatDuration(ms){if(!ms)return '—';return ms>=1000?(ms/1000).toFixed(2)+' s':ms+' ms'}
function statusItem(label,value){const item=document.createElement('div');const title=document.createElement('small');title.textContent=label;const content=document.createElement('strong');content.textContent=value;item.append(title,content);return item}
function renderSystemStatus(){if(!systemStatus)return;const turbo=Number(systemStatus.initial_turbo_seconds||0);const summary=$('system-summary');summary.replaceChildren(statusItem(t('device_model','Gerät'),systemStatus.model||'—'),statusItem(t('network_backend','Netzwerk-Backend'),systemStatus.network_backend||'—'),statusItem(t('swap_active','Swap aktiv'),systemStatus.swap_active?t('yes','Ja'):t('no','Nein')),statusItem(t('wait_online','Wait-online'),systemStatus.wait_online||'—'),statusItem(t('cpu_governor','CPU-Governor'),(systemStatus.cpu_governors||[]).join(', ')||'—'),statusItem(t('initial_turbo','Initial Turbo'),turbo?t('seconds_value','{value} Sekunden').replace('{value}',turbo):t('disabled','Deaktiviert')));const runtime=$('network-runtime');runtime.replaceChildren(statusItem(t('network_backend','Netzwerk-Backend'),systemStatus.network_backend||'—'));(systemStatus.interfaces||[]).forEach(item=>runtime.append(statusItem(item.name+(item.up?' · up':' · down'),(item.addresses||[]).join(', ')||'—')));$('static-ip-notice').textContent=t('static_ip_'+(systemStatus.static_apply_notice||'unknown'),t('static_ip_notice','Die Konfiguration wird gespeichert, aber noch nicht automatisch angewendet.'));$('boot-total').textContent=t('boot_total','Gesamte Bootzeit: {duration}').replace('{duration}',formatDuration(systemStatus.boot?.total_ms));const units=$('boot-units');units.replaceChildren();(systemStatus.boot?.top_units||[]).forEach(item=>{const row=document.createElement('div');const name=document.createElement('span');name.textContent=item.unit;const duration=document.createElement('strong');duration.textContent=formatDuration(item.duration_ms);row.append(name,duration);units.append(row)})}
let screenshotTimer=null;
function stopScreenshotLive(){if(screenshotTimer){clearInterval(screenshotTimer);screenshotTimer=null}$('screenshot-live').checked=false}
function captureScreenshot(){const image=$('screenshot-image');const error=$('screenshot-error');image.onerror=()=>{image.hidden=true;error.hidden=false;stopScreenshotLive()};image.onload=()=>{image.hidden=false;error.hidden=true};image.src='/api/admin/screenshot?_='+Date.now()}
function setScreenshotLive(enabled){if(screenshotTimer)clearInterval(screenshotTimer);screenshotTimer=null;if(enabled){captureScreenshot();screenshotTimer=setInterval(captureScreenshot,2000)}}
async function refreshSystem(){const button=$('refresh-system');button.disabled=true;try{systemStatus=await request('/api/admin/system');renderSystemStatus();message(t('analysis_updated','Systemanalyse aktualisiert.'))}catch(error){message(error.message,true)}finally{button.disabled=false}}
async function powerAction(action){const key=action==='reboot'?'reboot_confirm':'shutdown_confirm';const fallback=action==='reboot'?'MuPiBox jetzt neu starten?':'MuPiBox jetzt sicher herunterfahren?';if(!window.confirm(t(key,fallback)))return;const button=$(action==='reboot'?'reboot-box':'shutdown-box');button.disabled=true;try{await request('/api/admin/system/power',{method:'POST',body:JSON.stringify({action})});message(t(action==='reboot'?'reboot_scheduled':'shutdown_scheduled',action==='reboot'?'Neustart wurde ausgelöst.':'Herunterfahren wurde ausgelöst.'));}catch(error){button.disabled=false;message(error.message,true)}}
function wifiBars(percent){const level=percent>=75?4:percent>=50?3:percent>=25?2:1;return '▂▄▆█'.slice(0,level)}
function renderWifiAdapters(){
 const select=$('wifi-adapter');const selected=select.value||'auto';select.replaceChildren(new Option(t('wifi_auto_adapter','Automatisch (bevorzugt/aktiv)'),'auto'));
 const root=$('wifi-adapters');root.replaceChildren();
 wifiAdapters.forEach(adapter=>{
  const row=document.createElement('div');row.className='device-row adapter-choice';row.dataset.usable=String(!!adapter.usable);
  const primary=document.createElement('label');const primaryInput=document.createElement('input');primaryInput.type='radio';primaryInput.name='wifi-primary';primaryInput.value=adapter.interface;primaryInput.checked=!!adapter.preferred;const primaryText=document.createElement('span');primaryText.textContent=t('wifi_primary','Diesen Adapter verwenden');primary.append(primaryInput,primaryText);
  const details=document.createElement('small');const state=adapter.usable?t('wifi_ready','bereit'):t('wifi_not_ready','nicht bereit');const active=adapter.selected?' · '+t('wifi_currently_selected','aktuell gewählt'):'';details.textContent=[adapter.interface,adapter.driver,adapter.mac,adapter.state,state].filter(Boolean).join(' · ')+active;
  row.append(primary,details);root.append(row)
 });
 const onboard=wifiAdapters.find(adapter=>String(adapter.driver).toLowerCase()==='brcmfmac');const external=wifiAdapters.some(adapter=>adapter!==onboard);$('onboard-wifi-settings').hidden=!(onboard&&external);$('disable-onboard-wifi').checked=!!settings?.wifi?.disable_onboard;
 renderWifiScanOptions(selected)
}
function renderWifiScanOptions(selected){const select=$('wifi-adapter');const current=selected||select.value||'auto';select.replaceChildren(new Option(t('wifi_auto_adapter','Aktiver Adapter'),'auto'));wifiAdapters.forEach(adapter=>{if(!adapter.usable||!adapter.selected)return;const details=[adapter.interface,adapter.driver,adapter.state].filter(Boolean).join(' · ');select.append(new Option(details,adapter.interface))});select.value=[...select.options].some(option=>option.value===current)?current:'auto'}
async function saveWifiAdapters(){const primary=document.querySelector('input[name="wifi-primary"]:checked')?.value||'';if(!primary){message(t('wifi_choose_adapter','Bitte einen WLAN-Adapter auswählen.'),true);return}if(!window.confirm(t('wifi_switch_warning','Der gewählte Adapter wird aktiviert und alle anderen WLAN-Adapter werden deaktiviert. Die Verbindung kann kurz abbrechen. Fortfahren?')))return;const disableOnboard=!$('onboard-wifi-settings').hidden&&$('disable-onboard-wifi').checked;const button=$('save-wifi-adapters');button.disabled=true;try{const data=await request('/api/connectivity/wifi/preferences',{method:'PUT',body:JSON.stringify({primary_interface:primary,disable_onboard:disableOnboard})});wifiPrimary=data.primary_interface||primary;settings.wifi={...(settings.wifi||{}),primary_interface:wifiPrimary,primary_mac:data.primary_mac||settings.wifi?.primary_mac,disable_onboard:disableOnboard};message(t(data.restart_required?'wifi_switch_restart':'wifi_switch_scheduled',data.restart_required?'Adapterwechsel gespeichert. Für die Onboard-Einstellung bitte neu starten.':'Adapterwechsel gespeichert. Die Verbindung kann kurz abbrechen; danach die Seite neu laden.'))}catch(error){message(error.message,true)}finally{button.disabled=false}}
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
async function restoreBackup(){const file=$('restore-file').files[0];if(!file){message(t('restore_choose_file','Bitte zuerst eine Backup-Datei auswählen.'),true);return}if(!window.confirm(t('restore_confirm','Einstellungen und Fortschritt werden durch das Backup ersetzt. Wirklich fortfahren?')))return;const button=$('restore-backup');button.disabled=true;const form=new FormData();form.append('backup',file);const includeMedia=$('restore-media').checked;try{const result=await request('/api/admin/restore?include_media='+String(includeMedia),{method:'POST',body:form});$('restore-file').value='';showLogin();message(t('restore_done','Backup wiederhergestellt. Bitte mit dem Passwort aus dem Backup anmelden.').replace('{count}',result.media_files||0))}catch(error){message(error.message,true)}finally{button.disabled=false}}
async function loadReleases(){const button=$('load-releases');button.disabled=true;try{const data=await request('/api/admin/releases');const select=$('release-select');select.replaceChildren(new Option('—',''));(data.releases||[]).forEach(release=>select.append(new Option((release.name||release.tag_name)+(release.prerelease?' · '+t('prerelease','Vorabversion'):''),release.tag_name)));$('rollback-release').disabled=!data.rollback_available;$('release-state').textContent=t('current_version','Installierte Version: {version}').replace('{version}',data.current_version||'?');if(!(data.releases||[]).length)message(t('no_releases','Noch keine GitHub-Releases vorhanden.'),true)}catch(error){message(error.message,true)}finally{button.disabled=false}}
async function switchRelease(target){if(!target){message(t('choose_release','Bitte ein Release auswählen.'),true);return}if(!window.confirm(t('update_confirm','Die Box startet ihre Dienste während des Versionswechsels neu. Fortfahren?')))return;try{await request('/api/admin/releases/switch',{method:'POST',body:JSON.stringify({target})});message(t('update_started','Update wurde gestartet. Die Oberfläche verbindet sich nach einigen Minuten neu.'));setTimeout(()=>location.reload(),120000)}catch(error){message(error.message,true)}}
async function bootstrap(){await setLocale('de');try{authState=await request('/api/admin/auth');if(authState.protected&&!authState.authenticated){showLogin();return}showAdmin();await load()}catch(error){$('state').textContent=t('error','Fehler');message(error.message,true)}}

$('content-language').addEventListener('change',()=>{$('content-language').dataset.userSelected='true';renderNavigation()});
$('wifi-adapter').addEventListener('change',()=>{$('wifi-network').replaceChildren(new Option('—',''))});
$('save-wifi-adapters').onclick=saveWifiAdapters;
$('save-samba').onclick=saveSamba;
$('scan-wifi').onclick=scanWifi;
$('connect-wifi').onclick=connectWifi;
$('scan-bluetooth').onclick=scanBluetooth;
$('settings-form').addEventListener('submit',async event=>{event.preventDefault();const next=readSettings();const systemChanged=JSON.stringify(next.system)!==JSON.stringify(settings?.system||{});if(systemChanged&&!window.confirm(t('system_change_confirm','Systemoptionen werden jetzt angewendet. Netzwerk und Dienste können kurz unterbrochen werden. Fortfahren?')))return;try{const result=await request('/api/admin/settings',{method:'PUT',body:JSON.stringify(next)});fillSettings(result.settings);await setLocale(result.settings.admin_language);if(systemChanged)await refreshSystem();message(t('settings_saved','Einstellungen gespeichert.'))}catch(error){message(error.message,true)}});
$('add-category').onclick=()=>{const language=activeContentLanguage();idCounter++;navigation.categories.push({id:'category-'+Date.now()+'-'+idCounter,labels:{[language]:t('new_category','Neue Kategorie')},rows:[]});renderNavigation()};
$('save-navigation').onclick=async()=>{try{navigation=await request('/api/admin/navigation',{method:'PUT',body:JSON.stringify(navigation)});renderNavigation();message(t('content_saved','Inhalte gespeichert und Player aktualisiert.'))}catch(error){message(error.message,true)}};
$('rescan-library').onclick=async()=>{try{const result=await request('/api/admin/library/rescan',{method:'POST'});localDirectories=result.directories||[];renderNavigation();message(t('library_rescanned','Medienordner neu eingelesen.'))}catch(error){message(error.message,true)}};
$('restart-ui').onclick=async()=>{try{await request('/api/admin/ui/restart',{method:'POST'});message(t('ui_restarting','Touch-Oberfläche wird neu gestartet.'))}catch(error){message(error.message,true)}};
$('restore-backup').onclick=restoreBackup;
$('load-releases').onclick=loadReleases;
$('install-release').onclick=()=>switchRelease($('release-select').value);
$('rollback-release').onclick=()=>switchRelease('rollback');
$('refresh-system').onclick=refreshSystem;
$('screenshot-refresh').onclick=captureScreenshot;
$('screenshot-live').addEventListener('change',event=>setScreenshotLive(event.target.checked));
$('reboot-box').onclick=()=>powerAction('reboot');
$('shutdown-box').onclick=()=>powerAction('poweroff');
$('login-form').addEventListener('submit',login);
$('save-password').onclick=savePassword;
$('logout').onclick=logout;
const ttsLanguageNames={'de-DE':'Deutsch','en-GB':'English (UK)','en-US':'English (US)','fr-FR':'Français','es-ES':'Español','it-IT':'Italiano','nl-NL':'Nederlands','sv-SE':'Svenska','da-DK':'Dansk','nb-NO':'Norsk (Bokmål)','fi-FI':'Suomi','pl-PL':'Polski','cs-CZ':'Čeština','pt-PT':'Português','ru-RU':'Русский','uk-UA':'Українська','tr-TR':'Türkçe'};
let ttsVoices=[],ttsConfig=null,ttsStatus=null,ttsPollTimer=null;
function ttsLanguageLabel(code){return (ttsLanguageNames[code]||code)+' · '+code}
function formatBytes(n){if(!n)return '0 B';const units=['B','KB','MB','GB'];let i=0,v=n;while(v>=1024&&i<units.length-1){v/=1024;i++}return v.toFixed(v<10&&i>0?1:0)+' '+units[i]}
async function loadTTSVoices(){try{const data=await request('/api/admin/tts/voices');ttsVoices=data.voices||[]}catch(error){ttsVoices=[]}}
function ttsLanguagesAvailable(){return [...new Set(ttsVoices.map(v=>v.language))].sort()}
function ttsFamiliesFor(language){return [...new Set(ttsVoices.filter(v=>v.language===language).map(v=>v.voice_family))]}
function ttsVoicesFor(language,family){return ttsVoices.filter(v=>v.language===language&&v.voice_family===family)}
function ttsFamilyDisplayName(language,family){const v=ttsVoicesFor(language,family)[0];return v?v.display_name.replace(/\s*\([^)]*\)\s*$/,''):family}
function renderTTSLanguageSelect(selected){const select=$('tts2-language');const codes=ttsLanguagesAvailable();select.replaceChildren(...codes.map(c=>new Option(ttsLanguageLabel(c),c)));if(codes.includes(selected))select.value=selected;else if(codes.length)select.value=codes[0]}
function renderTTSVoiceSelect(language,selectedFamily){const select=$('tts2-voice');const families=ttsFamiliesFor(language);select.replaceChildren(...families.map(f=>new Option(ttsFamilyDisplayName(language,f),f)));$('tts2-no-voice').hidden=families.length>0;select.disabled=families.length===0;if(families.includes(selectedFamily))select.value=selectedFamily;else if(families.length)select.value=families[0]}
function renderTTSQualitySelect(language,family,selectedQuality){const select=$('tts2-quality');const qualities=[...new Set(ttsVoicesFor(language,family).map(v=>v.quality))];select.replaceChildren(...qualities.map(q=>new Option(q,q)));select.disabled=qualities.length===0;if(qualities.includes(selectedQuality))select.value=selectedQuality;else if(qualities.length)select.value=qualities[0]}
function ttsSelectedVoiceID(){const language=$('tts2-language').value,family=$('tts2-voice').value,quality=$('tts2-quality').value;const match=ttsVoices.find(v=>v.language===language&&v.voice_family===family&&v.quality===quality);return match?match.id:''}
function fillTTSConfig(cfg){
 ttsConfig=cfg;
 $('tts2-enabled').checked=!!cfg.enabled;
 $('tts2-cpu').value=cfg.background_cpu_percent||30;
 $('tts2-prerender').checked=!!cfg.pre_rendering_enabled;
 const language=cfg.language||ttsLanguagesAvailable()[0]||'';
 renderTTSLanguageSelect(language);
 const currentLanguage=$('tts2-language').value;
 const selectedVoice=ttsVoices.find(v=>v.id===cfg.voice_id);
 renderTTSVoiceSelect(currentLanguage,selectedVoice?.voice_family);
 renderTTSQualitySelect(currentLanguage,$('tts2-voice').value,selectedVoice?.quality||cfg.quality)
}
function priorityRow(label,value){const div=document.createElement('div');const l=document.createElement('span');l.textContent=label;const v=document.createElement('strong');v.textContent=value;div.append(l,v);return div}
function renderTTSStatus(){
 if(!ttsStatus)return;
 $('tts-unavailable').hidden=ttsStatus.available!==false;
 const gen=ttsStatus.building_generation||ttsStatus.active_generation;
 const bar=$('tts2-progress-bar');
 if(!ttsStatus.enabled){$('tts2-status-line').textContent=t('tts_status_disabled','Text-to-Speech ist deaktiviert.');bar.style.width='0%'}
 else if(!gen){$('tts2-status-line').textContent=t('tts_status_idle','Sprach-Cache ist vollständig.');bar.style.width='100%'}
 else if(ttsStatus.building_generation){$('tts2-status-line').textContent=t('tts_status_building','Neuer TTS-Cache wird vorbereitet')+' — '+ttsLanguageLabel(gen.language);bar.style.width=gen.progress_percent+'%'}
 else{$('tts2-status-line').textContent=t('tts_status_active','Hintergrundverarbeitung läuft');bar.style.width='100%'}
 $('tts2-counts').replaceChildren(
  statusItem(t('tts_created','Erstellt'),String(gen?.completed??0)),
  statusItem(t('tts_pending','Ausstehend'),String((gen?.pending_high||0)+(gen?.pending_normal||0)+(gen?.pending_low||0))),
  statusItem(t('tts_failed','Fehler'),String(gen?.failed??0))
 );
 $('tts2-priorities').replaceChildren(
  priorityRow(t('tts_priority_high','HIGH'),String(gen?.pending_high??0)),
  priorityRow(t('tts_priority_normal','NORMAL'),String(gen?.pending_normal??0)),
  priorityRow(t('tts_priority_low','LOW'),String(gen?.pending_low??0))
 );
 $('tts2-cache-stats').replaceChildren(
  statusItem(t('tts_cache_files','Dateien'),String(ttsStatus.cache_entries||0)),
  statusItem(t('tts_cache_size','Cache-Größe'),formatBytes(ttsStatus.cache_size_bytes||0)),
  statusItem(t('tts_cpu_limit_label','CPU-Limit'),(ttsStatus.cpu_limit_percent||0)+' %'),
  statusItem(t('tts_worker_running','Worker'),ttsStatus.worker_running?t('tts_worker_running','Hintergrund-Worker aktiv'):t('tts_worker_stopped','Hintergrund-Worker gestoppt'))
 );
 $('tts2-last-error').hidden=!ttsStatus.last_error;
 $('tts2-last-error').textContent=ttsStatus.last_error?(t('tts_last_error','Letzter Fehler')+': '+ttsStatus.last_error):'';
 $('tts2-diagnostics').replaceChildren(
  statusItem(t('tts_generation_id','Generation'),gen?String(gen.id):'—'),
  statusItem(t('tts_generation_status','Status'),gen?gen.status:'—')
 )
}
function ttsPollingInterval(){return ttsStatus&&ttsStatus.building_generation?2000:8000}
async function loadTTSStatus(){try{ttsStatus=await request('/api/admin/tts/status');renderTTSStatus()}catch(error){}}
function scheduleTTSPoll(){clearTimeout(ttsPollTimer);ttsPollTimer=setTimeout(async()=>{await loadTTSStatus();scheduleTTSPoll()},ttsPollingInterval())}
function stopTTSPoll(){clearTimeout(ttsPollTimer);ttsPollTimer=null}
async function enterTTSView(){
 try{
  await loadTTSVoices();
  if(!ttsConfig)ttsConfig=await request('/api/admin/tts/config');
  fillTTSConfig(ttsConfig);
  await loadTTSStatus()
 }catch(error){message(error.message,true)}
 scheduleTTSPoll()
}
function leaveTTSView(){stopTTSPoll()}
async function saveTTSConfig(){
 const enabled=$('tts2-enabled').checked;
 const language=$('tts2-language').value;
 const voiceID=ttsSelectedVoiceID();
 if(enabled&&(!language||!voiceID)){message(t('tts_no_voice_for_language','Für diese Sprache ist noch keine Stimme installiert.'),true);return}
 const languageOrVoiceChanged=enabled&&ttsConfig&&(ttsConfig.language!==language||ttsConfig.voice_id!==voiceID);
 const firstTimeEnable=enabled&&(!ttsConfig||!ttsConfig.enabled);
 if((languageOrVoiceChanged||firstTimeEnable)&&!window.confirm(t('tts_change_confirm_title','TTS-Sprache ändern?')+'\n\n'+t('tts_change_confirm_text','Die vorhandenen Sprachdateien werden im Hintergrund neu erstellt.')))return;
 const body={enabled,language,voice_id:voiceID,quality:$('tts2-quality').value,background_cpu_percent:Number($('tts2-cpu').value),pre_rendering_enabled:$('tts2-prerender').checked};
 const button=$('tts2-save');button.disabled=true;
 try{
  const result=await request('/api/admin/tts/config',{method:'PUT',body:JSON.stringify(body)});
  ttsConfig=result.settings;ttsStatus=result.status;renderTTSStatus();
  message(t('tts_config_saved','TTS-Konfiguration gespeichert.'))
 }catch(error){message(error.message,true)}finally{button.disabled=false}
}
async function testTTSVoice(){
 const text=$('tts2-test-text').value.trim();if(!text)return;
 const button=$('tts2-test');button.disabled=true;
 try{
  await request('/api/admin/tts/test',{method:'POST',body:JSON.stringify({text,voice_id:ttsSelectedVoiceID(),language:$('tts2-language').value})});
  message(t('tts_test_playing','Testwiedergabe gestartet.'))
 }catch(error){message(error.message,true)}finally{button.disabled=false}
}
async function fillMissingTTS(){
 const button=$('tts2-fill-missing');button.disabled=true;
 try{await request('/api/admin/tts/cache/fill-missing',{method:'POST'});message(t('tts_fill_missing_done','Fehlende Sprachdateien werden im Hintergrund erzeugt.'));await loadTTSStatus()}catch(error){message(error.message,true)}finally{button.disabled=false}
}
async function rebuildTTSCache(){
 if(!window.confirm(t('tts_rebuild_confirm','Der TTS-Cache wird für die aktuelle Sprache/Stimme neu aufgebaut. Fortfahren?')))return;
 const button=$('tts2-rebuild');button.disabled=true;
 try{await request('/api/admin/tts/cache/rebuild',{method:'POST'});message(t('tts_rebuild_done','Cache-Neuaufbau gestartet.'));await loadTTSStatus()}catch(error){message(error.message,true)}finally{button.disabled=false}
}
async function cleanupTTSCache(){
 const button=$('tts2-cleanup');button.disabled=true;
 try{await request('/api/admin/tts/cache/cleanup',{method:'POST'});message(t('tts_cleanup_done','Bereinigung ausgeführt.'))}catch(error){message(error.message,true)}finally{button.disabled=false}
}
$('tts2-language').addEventListener('change',()=>{renderTTSVoiceSelect($('tts2-language').value);renderTTSQualitySelect($('tts2-language').value,$('tts2-voice').value)});
$('tts2-voice').addEventListener('change',()=>renderTTSQualitySelect($('tts2-language').value,$('tts2-voice').value));
$('tts2-save').onclick=saveTTSConfig;
$('tts2-test').onclick=testTTSVoice;
$('tts2-fill-missing').onclick=fillMissingTTS;
$('tts2-rebuild').onclick=rebuildTTSCache;
$('tts2-cleanup').onclick=cleanupTTSCache;

let audioStatus=null;
function renderAudioStatus(){
 if(!audioStatus)return;
 $('audio-status').replaceChildren(
  statusItem(t('audio_aplay','aplay'),audioStatus.aplay_installed?t('installed','installiert'):t('not_installed','nicht installiert')),
  statusItem(t('audio_mpv','mpv'),audioStatus.mpv_installed?t('installed','installiert'):t('not_installed','nicht installiert')),
  statusItem(t('audio_mupihat_detected','MuPiHAT erkannt'),audioStatus.mupihat_detected?t('yes','Ja'):t('no','Nein')),
  statusItem(t('audio_overlay_configured','MuPiHAT-Overlay konfiguriert'),audioStatus.mupihat_overlay_configured?t('yes','Ja'):t('no','Nein'))
 );
 $('audio-reboot-notice').hidden=!audioStatus.reboot_required;
 const select=$('audio-device');const current=audioStatus.configured_device||'';
 select.replaceChildren(new Option(t('audio_device_auto','Automatisch (mpv-Standard)'),''));
 (audioStatus.mpv_devices||[]).forEach(device=>select.append(new Option(device.name+' — '+device.id,device.id)));
 if([...select.options].some(option=>option.value===current))select.value=current;
 else if(current){select.append(new Option(current,current));select.value=current}
 const recommendation=$('audio-device-recommendation');
 if(audioStatus.recommended_device&&audioStatus.recommended_device!==current){
  recommendation.hidden=false;recommendation.dataset.device=audioStatus.recommended_device;
  recommendation.querySelector('span').textContent=t('audio_device_recommended','Empfohlen für gleichzeitige Ansagen/Musik:')+' '+audioStatus.recommended_device
 }else recommendation.hidden=true
}
async function loadAudioStatus(){
 try{
  audioStatus=await request('/api/admin/audio/status');
  $('audio-mupihat-enabled').checked=!!settings?.mupihat?.audio_enabled;
  $('audio-mupihat-revision').value=settings?.mupihat?.revision||'auto';
  renderAudioStatus()
 }catch(error){message(error.message,true)}
}
async function saveMuPiHATAudio(){
 const button=$('audio-save-mupihat');button.disabled=true;
 try{
  const result=await request('/api/admin/audio/mupihat',{method:'PUT',body:JSON.stringify({enabled:$('audio-mupihat-enabled').checked})});
  if(settings)settings.mupihat={...(settings.mupihat||{}),audio_enabled:result.enabled};
  message(t('audio_mupihat_saved','MuPiHAT-Audio gespeichert.'));
  await loadAudioStatus()
 }catch(error){message(error.message,true)}finally{button.disabled=false}
}
async function saveAudioDevice(){
 const button=$('audio-save-device');button.disabled=true;
 try{
  await request('/api/admin/audio/device',{method:'PUT',body:JSON.stringify({device:$('audio-device').value})});
  if(settings)settings.audio={...(settings.audio||{}),device:$('audio-device').value};
  message(t('audio_device_saved','Wiedergabegerät gespeichert. Neustart erforderlich.'))
 }catch(error){message(error.message,true)}finally{button.disabled=false}
}
$('audio-refresh').onclick=loadAudioStatus;
$('audio-save-mupihat').onclick=saveMuPiHATAudio;
$('audio-save-device').onclick=saveAudioDevice;
$('audio-device-recommendation').querySelector('button').onclick=()=>{$('audio-device').value=$('audio-device-recommendation').dataset.device;$('audio-device-recommendation').hidden=true};

bootstrap();
