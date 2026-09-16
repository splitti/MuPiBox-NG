'use strict';
const $=id=>document.getElementById(id);
let settings=null;
let navigation={categories:[]};
let locale='de';
let translations={};
let idCounter=0;
let localDirectories=[];
let localRoot='/srv/mupibox/music';

const providerCatalog={
 'local-library':{label:'source_local',types:['library','path']},
 spotify:{label:'source_spotify',types:['artist','playlist','album','track']},
 'amazon-music':{label:'source_amazon',types:['artist','playlist','album','track']},
 stream:{label:'source_stream',types:['url']},
 podcast:{label:'source_podcast',types:['feed']}
};
const typeKeys={library:'type_library',path:'type_path',artist:'type_artist',playlist:'type_playlist',album:'type_album',track:'type_track',url:'type_url',feed:'type_feed'};

async function request(path,options={}){
 const r=await fetch(path,{...options,headers:{'Content-Type':'application/json',...(options.headers||{})}});
 const data=await r.json().catch(()=>({}));
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
 if(!$('content-language').dataset.userSelected)$('content-language').value=v.language||v.tts.language||'de'
}
function readSettings(){return{
 language:field('language').value.trim(),
 admin_language:field('admin_language').value,
 theme:field('theme').value,
 audio:{startup_volume:Number(field('startup_volume').value),max_volume:Number(field('max_volume').value),start_sound_enabled:field('start_sound_enabled').checked,shutdown_sound_enabled:field('shutdown_sound_enabled').checked},
 display:{brightness:Number(field('brightness').value),ui_size:field('ui_size').value,idle_off_minutes:Number(field('display_idle').value)},
 power:{idle_shutdown_minutes:Number(field('shutdown_idle').value)},
 tts:{enabled:field('tts_enabled').checked,language:field('tts_language').value.trim(),provider:field('tts_provider').value.trim()}
}}
function activeContentLanguage(){return($('content-language').value||settings?.language||'de').trim().toLowerCase()}
function labelFor(labels,id){const language=activeContentLanguage();return labels?.[language]||labels?.[language.split('-')[0]]||labels?.de||labels?.en||Object.values(labels||{})[0]||id}
function ensureLabels(node){if(!node.labels)node.labels={};return node.labels}
function labeledInput(value,onchange,label,options={}){
 const l=document.createElement('label');
 const span=document.createElement('span');span.textContent=label;
 const i=document.createElement('input');i.value=value||'';
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
 const providerSelect=labeledSelect(row.provider||'local-library',v=>{row.provider=v;row.source_type=providerCatalog[v].types[0];row.source_ref='';renderNavigation()},t('media_source','Medienquelle'),providerChoices);
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
  const [s,n,d]=await Promise.all([request('/api/admin/settings'),request('/api/admin/navigation'),request('/api/admin/local-directories')]);
  settings=s;localDirectories=d.directories||[];localRoot=d.root||localRoot;await setLocale(s.admin_language||'de');fillSettings(s);navigation=n;renderNavigation();$('state').textContent=t('sqlite_connected','SQLite verbunden')
 }catch(error){$('state').textContent=t('error','Fehler');message(error.message,true)}
}
document.querySelectorAll('nav button').forEach(button=>button.onclick=()=>{document.querySelectorAll('nav button').forEach(x=>x.classList.toggle('active',x===button));$('settings-view').hidden=button.dataset.view!=='settings';$('navigation-view').hidden=button.dataset.view!=='navigation'});
field('admin_language').addEventListener('change',()=>setLocale(field('admin_language').value));
field('ui_size').addEventListener('change',()=>$('settings-form').requestSubmit());
$('content-language').addEventListener('change',()=>{$('content-language').dataset.userSelected='true';renderNavigation()});
$('settings-form').addEventListener('submit',async event=>{event.preventDefault();try{const result=await request('/api/admin/settings',{method:'PUT',body:JSON.stringify(readSettings())});fillSettings(result.settings);await setLocale(result.settings.admin_language);message(t('settings_saved','Einstellungen gespeichert.'))}catch(error){message(error.message,true)}});
$('add-category').onclick=()=>{const language=activeContentLanguage();idCounter++;navigation.categories.push({id:'category-'+Date.now()+'-'+idCounter,labels:{[language]:t('new_category','Neue Kategorie')},rows:[]});renderNavigation()};
$('save-navigation').onclick=async()=>{try{navigation=await request('/api/admin/navigation',{method:'PUT',body:JSON.stringify(navigation)});renderNavigation();message(t('content_saved','Inhalte gespeichert und Player aktualisiert.'))}catch(error){message(error.message,true)}};
$('rescan-library').onclick=async()=>{try{const result=await request('/api/admin/library/rescan',{method:'POST'});localDirectories=result.directories||[];renderNavigation();message(t('library_rescanned','Medienordner neu eingelesen.'))}catch(error){message(error.message,true)}};
$('restart-ui').onclick=async()=>{try{await request('/api/admin/ui/restart',{method:'POST'});message(t('ui_restarting','Touch-Oberfläche wird neu gestartet.'))}catch(error){message(error.message,true)}};
setLocale('de').then(load);
