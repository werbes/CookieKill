// Home, friendship and island controls share the game's authoritative snapshot.
export function createIslandPanels(deps) {
  const {$, state, action, toast, escapeHTML, pretty, button, shopRow, openModal, closeModal, renderModal} = deps;
  const esc = value => escapeHTML(String(value ?? ''));
  const me = () => state.snapshot?.me || {};
  const island = () => state.snapshot?.island || null;
  const catalog = () => state.snapshot?.buildCatalog || [];
  const friends = () => state.snapshot?.friends || [];
  const count = item => Number(me().inventory?.[item] || 0);
  const ownIsland = () => !!me().homeIsland;
  const isOwner = () => island()?.owner === me().id;
  const canBuild = () => !!island() && (isOwner() || !!island().builders?.[me().id]);
  const recipes = () => state.snapshot?.recipes || state.serverRecipes || [];
  const buildName = id => catalog().find(recipe => recipe.id === id)?.name || pretty(id);
  const buildIcon = id => ({wall:'▥',floor:'▱',roof:'⌂',stairs:'▟',window:'▦',door:'▯',chair:'♧',table:'▰',bed:'▱',plant:'❧',mixer:'◴',oven:'▣',garden_bed:'❧',pond:'≈',chest:'▣',fire:'♨'}[id] || '◇');
  let category = 'building', friendCodeDraft = '', pendingDestroy = '';
  const costs = cost => Object.entries(cost || {}).map(([item, amount]) => `${amount} ${pretty(item)}`).join(' · ') || 'No materials';
  const afford = cost => Object.entries(cost || {}).every(([item, amount]) => count(item) >= amount);
  const now = () => state.snapshot?.serverTime || Date.now()/1000;
  function travelButtons() {
    return `<div class="item-actions">${ownIsland() && me().island !== me().id ? button('Go to my island','teleport_home',{},false,true) : ''}${me().island ? button('Return to main island','teleport_main') : ''}</div>`;
  }
  function pink() {
    const p = me();
    if (ownIsland()) {
      return `<div class="island-welcome"><span class="island-emblem pink" aria-hidden="true">⌂</span><h3>Your home island is ready</h3><p>Would you like to teleport to your home island?</p><div class="item-actions">${button('Yes, take me home','teleport_home',{},false,true)}${button('Not now','island-close')}</div></div>`;
    }
    if (!p.homeOffer) return `<div class="island-welcome"><span class="island-emblem pink" aria-hidden="true">◎</span><h3>A little pink lollipop</h3><p>A sweet discovery at the end of the clay tunnel. Eat it to discover a place of your own.</p>${button('Eat the pink lollipop','eat_lollipop',{item:'pink'},false,true)}</div>`;
    return `<div class="island-welcome"><span class="island-emblem pink" aria-hidden="true">⌂</span><h3>Buy your first land?</h3><p>Your own home island costs <strong>200 coins</strong>. Build, decorate, grow ingredients, and invite your friends.</p><div class="notice">${p.coins || 0} coins available${p.coins < 200 ? ' · Gather and trade to save for your island.' : ''}</div><div class="item-actions">${button('Yes, buy land · 200 coins','island-buy-home',{},p.coins < 200,true)}${button('Not now','island-close')}</div><p class="field-hint">After buying, you can choose whether to teleport there.</p></div>`;
  }
  function blue() {
    if (!me().friendsTeleport) return `<div class="island-welcome"><span class="island-emblem blue" aria-hidden="true">◎</span><h3>The blue lollipop</h3><p>Eat this lollipop to open the friends teleport screen. Friends decide whether their islands welcome visitors.</p>${button('Eat the blue lollipop','eat_lollipop',{item:'blue'},false,true)}</div>`;
    return `<p class="panel-intro">Choose a friend's home island. Only islands whose owners allow visitors can be entered.</p>${friendRows(true)}${travelButtons()}`;
  }
  function camel() {
    const p=me(), price=p.camelDiscount ? 175 : 350, seconds=Math.max(0,(p.camelRentalUntil||0)-now());
    const required=recipes().filter(recipe=>recipe.id!=='fire');
    const giftReady=required.length>0&&required.every(recipe=>count(recipe.id)>0);
    const gifts=required.map(recipe=>`<span class="ingredient-chip${count(recipe.id)>0?'':' missing'}">${esc(recipe.name || pretty(recipe.id))} · ${count(recipe.id)>0?'ready':'missing'}</span>`).join('');
    return `<p class="panel-intro">Abu Fanous keeps his camels on the far eastern edge of the main island. Rent a companion for a short journey, or buy one to keep.</p><div class="notice good">${p.coins||0} coins${p.camelDiscount?' · Your baker’s discount is permanent.':''}</div><div class="shop-list">${shopRow('♞','Borrow a camel','50 coins for 20 minutes.',button(seconds>0?`Rental active · ${Math.ceil(seconds/60)} min`:'Rent · 50 coins','camel_rent',{},p.camelOwned||seconds>0||p.coins<50,true))}${shopRow('♞',p.camelOwned?'Your camel':'A lifelong companion',p.camelOwned?'You own a camel permanently.':`${price} coins to keep your camel permanently.`,button(p.camelOwned?'Owned':`Buy · ${price} coins`,'camel_buy',{},p.camelOwned||p.coins<price,true))}${shopRow('◉','A taste of every recipe',p.camelDiscount?'Abu Fanous has tasted your complete collection.':'Give one of every kind of cookie and cake to reduce the purchase price to 175 coins.',button(p.camelDiscount?'Discount unlocked':'Give the complete collection','camel_gift',{},p.camelDiscount||!giftReady))}</div><div class="recipe-cost camel-gifts">${gifts}</div>${p.camelOwned||seconds>0?`<div class="item-actions">${button(p.ridingCamel?'Dismount':'Ride my camel','camel_ride',{},false,true)}</div>`:''}`;
  }
  function friendRows(teleportOnly=false) {
    const list=friends().filter(friend=>!friend.request);
    if(!list.length)return '<div class="empty-message">No friends yet. Add a friend using their permanent friend code in the Friends menu.</div>';
    return `<div class="shop-list">${list.map(friend=>{
      const status=[friend.online?'Online':'Offline',friend.ownsIsland?(friend.visitors?'Visitors welcome':'Visitors closed'):'No home island'];
      let buttons='';
      if(teleportOnly)buttons+=button('Visit island','visit_island',{target:friend.id},!friend.ownsIsland||!friend.visitors,true);
      else {
        if(me().friendsTeleport)buttons+=button('Visit island','visit_island',{target:friend.id},!friend.ownsIsland||!friend.visitors,true);
        if(ownIsland())buttons+=button(me().homeIsland?.builders?.[friend.id]?'Revoke building permission':'Allow this friend to build','island_builder',{target:friend.id,enabled:!me().homeIsland?.builders?.[friend.id]});
        buttons+=button('Remove friend','friend_remove',{target:friend.id});
      }
      return shopRow('◉',friend.callName||friend.username,`@${friend.username} · ${status.join(' · ')}${friend.canBuild?' · You may build on this island':''}`,buttons);
    }).join('')}</div>`;
  }
  function friendsPanel() {
    const p=me(), requests=friends().filter(friend=>friend.request);
    return `<p class="panel-intro">Your friend code and username are permanent. Share your code with another player so they can send a request. Your call name can still change.</p><div class="friend-identity"><div><span class="section-label">YOUR FRIEND CODE</span><strong>${esc(p.friendCode||'Loading…')}</strong><small>@${esc(p.username||'')} · ${p.publicFriendCode?'Shown below your call name':'Only you can see your code'}</small></div>${button(p.publicFriendCode?'Hide my code':'Show my code publicly','friend_public',{enabled:!p.publicFriendCode})}</div><form id="friend-request-form" class="friend-request-form"><label for="friend-code-input">ADD A FRIEND BY CODE</label><div class="inline-fields"><input id="friend-code-input" name="friendCode" autocomplete="off" maxlength="8" pattern="[A-Za-z]{4}[0-9]{4}" placeholder="FERN1234" value="${esc(friendCodeDraft)}" aria-describedby="friend-code-hint" required><button class="small-button accent" type="submit">Send request</button></div><p id="friend-code-hint" class="field-hint">A four-letter word followed by four numbers.</p></form>${requests.length?`<div class="section-label">FRIEND REQUESTS <span>${requests.length} waiting</span></div><div class="shop-list">${requests.map(friend=>shopRow('＋',friend.callName||friend.username,`@${friend.username} wants to be your friend.`,button('Accept','friend_accept',{target:friend.id},false,true)+button('Decline','friend_decline',{target:friend.id}))).join('')}</div>`:''}${ownIsland()?`<div class="section-label island-section">YOUR ISLAND</div><div class="notice${p.homeIsland.visitors?' good':''}"><strong>${p.homeIsland.visitors?'Friends may visit':'Visits are closed'}</strong><p>Building is allowed only for the individual friends you choose below. Friends can contribute materials to your island chest.</p>${button(p.homeIsland.visitors?'Close visits':'Allow visitors','island_visitors',{enabled:!p.homeIsland.visitors})}</div>`:''}<div class="section-label island-section">YOUR FRIENDS</div>${friendRows()}${travelButtons()}`;
  }
  function building() {
    const p=me(), current=island();
    if(!current) return `<div class="island-welcome"><span class="island-emblem" aria-hidden="true">⌂</span><h3>${ownIsland()?'Your next project is waiting':'A home of your own'}</h3><p>${ownIsland()?'Travel to your home island to build and decorate. Your build menu lives in hotbar slot 6.':'Find the rock-covered cave near the middle of Sunbaked Sands. The pink lollipop offers your first home island for 200 coins.'}</p><div class="notice">House building is reserved for home islands. Main-island campfires burn for 25 minutes.</div>${travelButtons()}</div>`;
    const tabs=[['building','Building'],['decor','Decor'],['appliances','Appliances']];
    const list=catalog().filter(recipe=>recipe.category.toLowerCase()===category);
    const objects=current.objects||[];
    return `<p class="panel-intro">${isOwner()?'Your home island':"Your friend's home island"} · ${canBuild()?'Build with materials you carry. Withdraw shared supplies from the island chest.':'The owner must grant you building permission before you can edit this island.'}</p><div class="build-toolbar">${tabs.map(([key,label])=>`<button class="build-category${key===category?' active':''}" type="button" data-action="build-category" data-payload="${esc(JSON.stringify({category:key}))}" aria-pressed="${key===category}">${label}</button>`).join('')}</div><div class="notice">Choose an item, aim to place it, and click. <strong>R</strong> rotates by 45°. <strong>Page Up / Page Down</strong> adjusts height. <strong>Esc</strong> cancels. Decor can overlap and stack; appliances need separate space. All placed items have collision.</div><div class="recipe-grid build-catalog">${list.map(recipe=>`<article class="recipe-card"><div class="recipe-top"><span class="shop-icon" aria-hidden="true">${buildIcon(recipe.id)}</span><div><h3>${esc(recipe.name)}</h3><span>${esc(recipe.width)} × ${esc(recipe.depth)} m · ${esc(recipe.height)} m tall</span></div></div><p>${esc(costs(recipe.cost))}</p>${button('Place '+recipe.name,'build-place',{item:recipe.id},!canBuild()||!afford(recipe.cost),true)}</article>`).join('')||'<div class="empty-message">The build catalog is loading.</div>'}</div><div class="section-label island-section">ON THIS ISLAND <span>${objects.length} placed objects</span></div><div class="shop-list">${objects.map(object=>shopRow(buildIcon(object.kind),buildName(object.kind),`${Math.round(object.rotation||0)}° · ${Number(object.x).toFixed(1)}, ${Number(object.z).toFixed(1)}`,button('Inspect','island-object',{id:object.id}))).join('')||'<div class="empty-message">A blank canvas. Your first build starts here.</div>'}</div><div class="item-actions island-section">${button('Island material chest','tab',{tab:'chest'})}${button('Friends & permissions','tab',{tab:'friends'})}${button('Return to main island','teleport_main')}</div>`;
  }
  function buildObject() {
    const object=island()?.objects?.find(object=>object.id===state.context?.id);
    if(!object)return '<div class="empty-message">This object is no longer on the island.</div>';
    let use='';
    const usable={chair:me().posture?'Stand up':'Sit in chair',bed:me().posture?'Get up':'Lie on bed',door:object.open?'Close door':'Open door',mixer:'Make dough',oven:'Use oven',fire:object.lit?'Put out campfire':'Light campfire'};
    if(usable[object.kind])use=button(usable[object.kind],'use_build',{target:object.id},false,true);
    if(object.kind==='fire')use+=button('Bake by this campfire','tab',{tab:'recipes'},!object.lit,true);
    if(object.kind==='garden_bed')use=object.planted?button(object.readyAt>now()?`Growing · ${Math.ceil((object.readyAt-now())/60)} min`:'Harvest '+pretty(object.planted),'use_build',{target:object.id},!canBuild()||object.readyAt>now(),true):button('Plant berries','use_build',{target:object.id,item:'berry'},!canBuild()||count('berry')<1,true)+button('Plant nuts','use_build',{target:object.id,item:'nut'},!canBuild()||count('nut')<1);
    if(object.kind==='chest')use=button('Open material chest','tab',{tab:'chest'},false,true);
    if(object.kind==='pond')use=`<p class="panel-intro">${object.adopted?'Your adopted '+esc(typeof object.adopted==='string'?object.adopted:object.adopted.species||'animal')+' has a home here.':'Find a wounded fish or turtle in the ocean and interact with it to offer this pond as its new home.'}</p>`;
    const recipe=catalog().find(recipe=>recipe.id===object.kind);
    const refund=Object.fromEntries(Object.entries(recipe?.cost||{}).map(([key,value])=>[key,Math.floor(value/2)]).filter(([,value])=>value>0));
    const actions=canBuild()?button('Move','build-move',{id:object.id})+button('Rotate 45°','build-rotate',{id:object.id})+button('Destroy…','build-destroy-prompt',{id:object.id}):'';
    return `<div class="island-object-heading"><span class="island-emblem">${buildIcon(object.kind)}</span><div><h3>${esc(buildName(object.kind))}</h3><p>${Math.round(object.rotation||0)}° · Height ${Number(object.y||0).toFixed(1)} m</p></div></div><div class="item-actions">${use}</div><div class="notice island-section">Move and rotate your builds at any time. Destroying returns half of each material, rounded down: ${esc(costs(refund))}.</div><div class="item-actions">${actions}</div>${pendingDestroy===object.id?`<div class="notice destructive-confirm island-section"><strong>Destroy this ${esc(buildName(object.kind).toLowerCase())}?</strong><p>You will receive ${esc(costs(refund))}.</p><div class="item-actions">${button('Yes, destroy','destroy_build',{target:object.id})}${button('Keep it','build-destroy-cancel')}</div></div>`:''}<div class="item-actions island-section">${button('Build catalog','tab',{tab:'build'})}${button('Back to the island','island-close')}</div>`;
  }
  function chest() {
    const current=island();
    if(!current)return '<div class="notice">Travel to a home island to open its material chest.</div>';
    const materials=['wood','stick','stone','berry','nut','cactus','shell','pearl','sea_salt'];
    const available=materials.filter(item=>count(item)>0||current.chest?.[item]>0);
    const nearby=current.objects?.some(object=>object.kind==='chest'&&Math.hypot(object.x-me().x,object.z-me().z)<=5);
    return `<p class="panel-intro">Contribute materials to help the owner build a home. The owner and friends with building permission can withdraw chest materials into their bags to build with.</p><div class="notice${nearby?' good':''}">${nearby?'The chest is within reach.':'Stand within five metres of a material chest to donate or withdraw.'}${!canBuild()?' Only the owner and permitted builders can view and withdraw its contents.':''}</div><div class="shop-list">${available.map(item=>{const held=count(item),stored=Number(current.chest?.[item]||0);return shopRow('◇',pretty(item),`${held} carried${canBuild()?` · ${stored} in the chest`:''}`,button('Give one','chest_deposit',{item,amount:1},!nearby||held<1)+button(held>20?'Give 20':'Give all','chest_deposit',{item,amount:Math.min(20,held)},!nearby||held<1,true)+button('Take one','chest_withdraw',{item,amount:1},!nearby||!canBuild()||stored<1)+button(stored>20?'Take 20':'Take all','chest_withdraw',{item,amount:Math.min(20,stored)},!nearby||!canBuild()||stored<1));}).join('')||'<div class="empty-message">No building materials to show. Bring some materials to contribute.</div>'}</div>${travelButtons()}`;
  }
  function animal() {
    const creature=state.snapshot?.animals?.find(animal=>animal.id===state.context?.id)||state.context||{};
    const ponds=me().homeIsland?.objects?.filter(object=>object.kind==='pond'&&!object.adopted)||[];
    const eligible=['fish','turtle'].includes(creature.species)&&creature.need==='wounded'&&!creature.egg&&creature.health>0;
    return `<p class="panel-intro">${esc(pretty(creature.species||'Ocean animal'))} · ${creature.need?'Needs help with '+esc(pretty(creature.need).toLowerCase()):'Healthy'}</p><div class="item-actions">${button('Help this animal','interact',{target:creature.id},!creature.need,true)}${button('Adopt into my pond','adopt_animal',{target:creature.id},!eligible||ponds.length<1)}</div><p class="field-hint">A wounded fish or turtle can be adopted into an empty pond on your home island. Each pond shelters one animal.</p>`;
  }
  function companion() {
    const p=me(), seconds=Math.max(0,(p.camelRentalUntil||0)-now());
    if(!p.camelOwned&&seconds<=0)return '';
    return `<div class="section-label island-section">YOUR CAMEL <span>${p.camelOwned?'A permanent companion':`${Math.ceil(seconds/60)} rental minutes remaining`}</span></div><div class="notice good"><p>${p.ridingCamel?'Your camel is carrying you.':'Your camel is ready to travel on dry land.'} Press <strong>V</strong> to mount or dismount.</p>${button(p.ridingCamel?'Dismount':'Ride my camel','camel_ride',{},false,true)}</div>`;
  }
  function render(type) {
    return ({friends:friendsPanel,build:building,build_object:buildObject,chest,animal})[type]?.() ?? null;
  }
  function shop(kind) {
    return ({pink_lollipop:pink,blue_lollipop:blue,abu_fanous:camel})[kind]?.() ?? null;
  }
  function handle(kind,data) {
    if(kind==='island-close'){closeModal();return true;}
    if(kind==='island-buy-home'){action('buy_home');return true;}
    if(kind==='build-category'){category=data.category;renderModal();return true;}
    if(kind==='island-object'){const object=island()?.objects?.find(object=>object.id===data.id);if(object)openModal('build_object',object);return true;}
    if(kind==='build-place'||kind==='build-move') {
      const object=kind==='build-move'?island()?.objects?.find(object=>object.id===data.id):null;
      if(!canBuild()||kind==='build-move'&&!object)return true;
      if(!deps.beginBuild){toast('Placement controls are loading. Try again in a moment.');return true;}
      closeModal();deps.beginBuild({item:object?.kind||data.item,...(object?{object}:{})});return true;
    }
    if(kind==='build-rotate'){const object=island()?.objects?.find(object=>object.id===data.id);if(object)action('move_build',{target:object.id,x:object.x,y:object.y||0,z:object.z,rotation:((object.rotation||0)+45)%360});return true;}
    if(kind==='build-destroy-prompt'){pendingDestroy=data.id;renderModal();return true;}
    if(kind==='build-destroy-cancel'){pendingDestroy='';renderModal();return true;}
    if(['teleport_home','teleport_main','visit_island','adopt_animal'].includes(kind)){action(kind,data);closeModal();return true;}
    if(kind==='use_build'&&island()?.objects?.find(object=>object.id===data.target)?.kind==='oven'){action(kind,data);openModal('recipes');return true;}
    return false;
  }
  function input(event){if(event.target.id==='friend-code-input')friendCodeDraft=event.target.value;}
  function submit(event) {
    if(event.target.id!=='friend-request-form')return false;
    event.preventDefault();const code=$('friend-code-input').value.trim().toUpperCase();
    if(!/^[A-Z]{4}\d{4}$/.test(code)){toast('Enter a four-letter word and four numbers, such as FERN1234.');return true;}
    action('friend_request',{target:code});friendCodeDraft='';renderModal();return true;
  }
  return {render,shop,handle,input,submit,companion};
}
