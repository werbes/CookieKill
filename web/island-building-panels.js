// Island construction and chest menus; shared and personal storage stay distinct.
export function createIslandBuildingPanels(deps) {
  const {state,action,toast,button,shopRow,openModal,closeModal,renderModal,escapeHTML:esc,pretty}=deps;
  const me=()=>state.snapshot?.me||{}, island=()=>state.snapshot?.island||null;
  const catalog=()=>state.snapshot?.buildCatalog||[], count=item=>Number(me().inventory?.[item]||0);
  const owner=()=>island()?.owner===me().id;
  const canBuild=()=>!!island()&&(owner()||!!island().builders?.[me().id]);
  const materialCount=item=>owner()?count(item):Number(island()?.chest?.[item]||0);
  const afford=cost=>Object.entries(cost||{}).every(([item,n])=>materialCount(item)>=n);
  const name=kind=>catalog().find(r=>r.id===kind)?.name||pretty(kind);
  const itemName=item=>item.startsWith('build_')?name(item.slice(6).split(':')[0])+(item.includes(':')?' · '+pretty(item.split(':')[1]):''):deps.itemName?.(item)||pretty(item);
  const colors=['teal','blue','red','purple','sand','black','white'];
  const icons={wall:'▥',floor:'▱',roof:'⌂',stairs:'▟',window:'▦',door:'▯',chair:'♧',table:'▰',bed:'▱',plant:'❧',mixer:'◴',oven:'▣',garden_bed:'❧',tree:'♠',flowers:'✿',pond:'≈',chest:'▣',storage_chest:'▣',fire:'♨'};
  const costText=cost=>Object.entries(cost||{}).map(([item,n])=>`${n} ${pretty(item)}`).join(' · ')||'No materials';
  let category='building';
  function travel(){return `<div class="item-actions">${me().homeIsland&&me().island!==me().id?button('Go to my island','teleport_home',{},false,true):''}${me().island?button('Return to main island','teleport_main'):''}</div>`;}
  function colorControls(object){
    if(!object||!['chest','storage_chest'].includes(object.kind)||!me().buildMode||me().editingBuildId!==object.id)return '';
    return `<div class="notice"><strong>Chest color</strong><p>Use one matching paint. ${owner()?'Paint comes from your inventory.':'Paint comes from the shared materials chest.'}</p><div class="item-actions">${colors.map(color=>button(pretty(color),'paint_build',{target:object.id,item:color},object.color===color||materialCount('paint_'+color)<1)).join('')}</div></div>`;
  }
  function building(){
    const current=island(),p=me();
    if(!current)return `<div class="island-welcome"><h3>${p.homeIsland?'Your island is waiting':'A home of your own'}</h3><p>${p.homeIsland?'Travel home to build, plant your garden, and store your belongings.':'Taste the pink lollipop in the desert cave to buy your first home island.'}</p>${travel()}</div>`;
    const tabs=[['building','Building'],['decor','Decor'],['appliances','Appliances'],['garden','Garden']];
    const editing=current.objects?.find(o=>o.id===p.editingBuildId);
    const packed=Object.entries(p.inventory||{}).filter(([item,n])=>n>0&&item.startsWith('build_'));
    return `<p class="panel-intro">${owner()?'Your home island':"Your friend’s home island"} · ${canBuild()?(owner()?'Build with materials in your inventory.':'Guest builders use materials directly from the shared chest. Your own items stay yours.'):'The owner must grant building permission before you can edit this island.'}</p><div class="notice${p.buildMode?' good':''}"><strong>${p.buildMode?'Build mode is on':'Explore mode'}</strong><p>${p.buildMode?'Look at an object and press E to take edit control. R rotates, T destroys, F packs into your inventory, and left click places it. Escape cancels the current edit.':'Objects can only be placed, moved, painted, packed, or destroyed in build mode.'}</p>${button(p.buildMode?'Exit build mode':'Enter build mode','island-build-mode',{enabled:!p.buildMode},!canBuild(),true)}${p.buildMode?button('Return to building','island-close'):''}</div>${editing?`<div class="section-label">EDITING ${esc(name(editing.kind))}</div>${colorControls(editing)}`:''}<div class="build-toolbar">${tabs.map(([key,label])=>button(label,'build-category',{category:key},false,key===category)).join('')}</div><p class="field-hint">Choose an item, aim, and left click to place it. R rotates 45°. Page Up / Page Down changes height. Canceling an edit leaves the original object in place.</p><div class="recipe-grid build-catalog">${catalog().filter(r=>r.category===category).map(r=>`<article class="recipe-card"><div class="recipe-top"><span class="shop-icon">${icons[r.id]||'◇'}</span><div><h3>${esc(r.name)}</h3><span>${esc(r.width)} × ${esc(r.depth)} m</span></div></div><p>${esc(costText(r.cost))}</p>${button((r.category==='garden'?'Plant ':'Place ')+r.name,'build-place',{item:r.id},!canBuild()||!p.buildMode||!afford(r.cost),true)}</article>`).join('')}</div>${owner()&&packed.length?`<div class="section-label island-section">PACKED FURNISHINGS</div><div class="shop-list">${packed.map(([item,n])=>shopRow('▣',itemName(item),`${n} packed · no materials needed`,button('Place','build-place',{item:item.slice(6).split(':')[0],packed:item},!p.buildMode,true))).join('')}</div>`:''}<div class="item-actions island-section">${button('Shared materials chest','tab',{tab:'chest'})}${button('Friends & permissions','tab',{tab:'friends'})}</div>${travel()}`;
  }
  function buildObject(){
    const object=island()?.objects?.find(o=>o.id===state.context?.id);
    if(!object)return '<div class="empty-message">This object is no longer on the island.</div>';
    const p=me(), now=state.snapshot?.serverTime||Date.now()/1000;
    let use='';
    const labels={chair:p.posture?'Stand up':'Sit in chair',bed:p.posture?'Get up':'Lie on bed',door:object.open?'Close door':'Open door',mixer:'Make dough',oven:'Use oven',fire:object.lit?'Put out campfire':'Light campfire'};
    if(labels[object.kind])use=button(labels[object.kind],'use_build',{target:object.id},!!object.editingBy,true);
    if(object.kind==='fire')use+=button('Bake by this campfire','tab',{tab:'recipes'},!object.lit,true);
    if(object.kind==='garden_bed')use=object.planted?button(object.readyAt>now?'Growing…':'Harvest '+pretty(object.planted),'use_build',{target:object.id},!canBuild()||object.readyAt>now,true):button('Plant berries','use_build',{target:object.id,item:'berry'},!canBuild()||materialCount('berry')<1,true)+button('Plant nuts','use_build',{target:object.id,item:'nut'},!canBuild()||materialCount('nut')<1);
    if(object.kind==='chest')use=button('Open shared materials chest','tab',{tab:'chest'},false,true);
    if(object.kind==='storage_chest')use=button('Open personal storage','storage-open',{id:object.id},!owner(),true);
    if(object.kind==='pond')use=`<p>${object.adopted?'Your rescued '+esc(object.adopted)+' is safe in this pond.':'Adopt a wounded fish or turtle from the ocean into this pond.'}</p>`;
    return `<div class="island-object-heading"><span class="island-emblem">${icons[object.kind]||'◇'}</span><div><h3>${esc(name(object.kind))}</h3></div></div><div class="item-actions">${use}</div><p class="field-hint">To edit this object, enter build mode, look at it, and press E.</p>${colorControls(object)}<div class="item-actions">${button('Home island menu','tab',{tab:'build'})}${button('Back to the island','island-close')}</div>`;
  }
  function chest(){
    const current=island();if(!current)return '<div class="notice">Travel to a home island to use its shared chest.</div>';
    const materials=['wood','stick','stone','berry','nut','cactus','shell','pearl','sea_salt',...colors.map(c=>'paint_'+c)];
    const near=current.objects?.some(o=>o.kind==='chest'&&!o.editingBy&&Math.hypot(o.x-me().x,o.z-me().z)<=5&&Math.abs((o.y||0)-(me().y||0))<=3);
    return `<p class="panel-intro">Visitors may donate building and garden materials. Only the island owner may withdraw them. Permitted guest builders use shared materials directly when placing objects.</p><div class="notice${near?' good':''}">${near?'The shared chest is within reach.':'Stand next to a shared chest and finish editing it first.'}</div><div class="shop-list">${materials.filter(item=>count(item)>0||current.chest?.[item]>0).map(item=>{const held=count(item),stored=current.chest?.[item]||0;return shopRow('◇',pretty(item),`${held} carried${canBuild()?` · ${stored} shared`:''}`,button('Give one','chest_deposit',{item,amount:1},!near||held<1)+button(held>20?'Give 20':'Give all','chest_deposit',{item,amount:Math.min(20,held)},!near||held<1,true)+(owner()?button('Take one','chest_withdraw',{item,amount:1},!near||stored<1)+button(stored>20?'Take 20':'Take all','chest_withdraw',{item,amount:Math.min(20,stored)},!near||stored<1):''));}).join('')||'<div class="empty-message">Bring materials to contribute to this island.</div>'}</div>${travel()}`;
  }
  function storage(){
    if(!owner())return '<div class="notice">Personal storage is private to the island owner.</div>';
    const object=island()?.objects?.find(o=>o.id===state.context?.id&&o.kind==='storage_chest');
    if(!object)return '<div class="notice">Walk to a personal storage chest and press E to open it.</div>';
    const contents=island().storage?.[object.id]||{},near=!object.editingBy&&Math.hypot(object.x-me().x,object.z-me().z)<=5&&Math.abs((object.y||0)-(me().y||0))<=3;
    const items=[...new Set([...Object.keys(me().inventory||{}),...Object.keys(contents)])].filter(item=>count(item)>0||contents[item]>0);
    return `<p class="panel-intro">Store any item here, including cookies, ingredients, paint, and packed furnishings. Only you can see or use this chest.</p><div class="notice${near?' good':''}">${near?'Your storage chest is within reach.':'Stand next to this chest and finish editing it first.'}</div><div class="shop-list">${items.map(item=>{const held=count(item),stored=contents[item]||0;return shopRow('▣',itemName(item),`${held} carried · ${stored} stored`,button('Store one','storage_deposit',{target:object.id,item,amount:1},!near||held<1)+button(held>20?'Store 20':'Store all','storage_deposit',{target:object.id,item,amount:Math.min(20,held)},!near||held<1,true)+button('Take one','storage_withdraw',{target:object.id,item,amount:1},!near||stored<1)+button(stored>20?'Take 20':'Take all','storage_withdraw',{target:object.id,item,amount:Math.min(20,stored)},!near||stored<1));}).join('')||'<div class="empty-message">Your chest is empty.</div>'}</div>`;
  }
  function handle(kind,data){
    if(kind==='island-build-mode'){deps.cancelBuild?.();action('build_mode',{enabled:data.enabled});if(data.enabled)closeModal();return true;}
    if(kind==='build-category'){category=data.category;renderModal();return true;}
    if(kind==='build-place'){if(!canBuild()||!me().buildMode)return true;closeModal();deps.beginBuild?.({item:data.item,packed:data.packed});return true;}
    if(kind==='storage-open'){const object=island()?.objects?.find(o=>o.id===data.id);if(object)openModal('storage',object);return true;}
    if(kind==='island-object'){const object=island()?.objects?.find(o=>o.id===data.id);if(object)openModal('build_object',object);return true;}
    return false;
  }
  return {building,buildObject,chest,storage,handle};
}
