import {createIslandBuildingPanels} from './island-building-panels.js';
// Home, friendship and island controls share the game's authoritative snapshot.
export function createIslandPanels(deps) {
  const {$, state, action, toast, escapeHTML, pretty, button, shopRow, openModal, closeModal, renderModal} = deps;
  const esc = value => escapeHTML(String(value ?? ''));
  const me = () => state.snapshot?.me || {};
  const island = () => state.snapshot?.island || null;
  const catalog = () => state.snapshot?.buildCatalog || [];
  const friends = () => state.snapshot?.friends || [];
	const camelNameDrafts=new Map();
	function camelNames(){
	  const herd=(me().camels||[]).filter(c=>!c.rentalUntil||c.rentalUntil>now());
	  if(!herd.length)return '';
	  return `<div class="section-label island-section">NAME YOUR CAMELS</div><p class="field-hint">Names follow each camel wherever you park or ride it.</p>${herd.map((camel,index)=>{
	    if(camelNameDrafts.get(camel.id)?.trim()===camel.name)camelNameDrafts.delete(camel.id);
	    const field='camel-name-'+camel.id;
	    return `<form class="rename-form" data-camel-name="${esc(camel.id)}"><label for="${esc(field)}">${esc(camel.name||'Camel '+(index+1))}</label><div class="inline-fields"><input id="${esc(field)}" data-camel-name="${esc(camel.id)}" value="${esc(camelNameDrafts.get(camel.id)??camel.name??'')}" placeholder="Give this camel a name" maxlength="24" required><button class="small-button accent" type="submit">Save name</button></div></form>`;
	  }).join('')}`;
	}
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
    const p=me(), price=p.camelDiscount ? 175 : 350, seconds=Math.max(0,(p.camelRentalUntil||0)-now()), owned=(p.camels||[]).filter(c=>!c.rentalUntil).length;
    const required=recipes().filter(recipe=>recipe.id!=='fire');
    const giftReady=required.length>0&&required.every(recipe=>count(recipe.id)>0);
    const gifts=required.map(recipe=>`<span class="ingredient-chip${count(recipe.id)>0?'':' missing'}">${esc(recipe.name || pretty(recipe.id))} · ${count(recipe.id)>0?'ready':'missing'}</span>`).join('');
    return `<p class="panel-intro">Abu Fanous keeps his camels on the far eastern edge of the main island. Grow your herd with as many permanent companions as you like.</p><div class="notice good">${p.coins||0} coins · ${owned} owned camel${owned===1?'':'s'}${p.camelDiscount?' · Your baker’s discount is permanent.':''}</div><div class="shop-list">${shopRow('♞','Borrow a camel','50 coins for 20 minutes. One active rental at a time.',button(seconds>0?`Rental active · ${Math.ceil(seconds/60)} min`:'Rent · 50 coins','camel_rent',{},seconds>0||p.coins<50,true))}${shopRow('♞','A lifelong companion',`${price} coins each. Every purchase adds another camel to your herd.`,button(`${owned?'Buy another':'Buy'} · ${price} coins`,'camel_buy',{},p.coins<price,true))}${shopRow('◉','A taste of every recipe',p.camelDiscount?'Abu Fanous has tasted your complete collection.':'Give one of every kind of cookie and cake to reduce the purchase price to 175 coins.',button(p.camelDiscount?'Discount unlocked':'Give the complete collection','camel_gift',{},p.camelDiscount||!giftReady))}</div><div class="recipe-cost camel-gifts">${gifts}</div>${companion()}`;
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
  const buildingPanels=createIslandBuildingPanels({...deps,button,shopRow,openModal,closeModal,renderModal});
  const {building,buildObject,chest,storage}=buildingPanels;
  function animal() {
    const creature=state.snapshot?.animals?.find(animal=>animal.id===state.context?.id)||state.context||{};
    const ponds=me().homeIsland?.objects?.filter(object=>object.kind==='pond'&&!object.adopted)||[];
    const eligible=['fish','turtle'].includes(creature.species)&&creature.need==='wounded'&&!creature.egg&&creature.health>0;
    return `<p class="panel-intro">${esc(pretty(creature.species||'Ocean animal'))} · ${creature.need?'Needs help with '+esc(pretty(creature.need).toLowerCase()):'Healthy'}</p><div class="item-actions">${button('Help this animal','interact',{target:creature.id},!creature.need,true)}${button('Adopt into my pond','adopt_animal',{target:creature.id},!eligible||ponds.length<1)}</div><p class="field-hint">A wounded fish or turtle can be adopted into an empty pond on your home island. Each pond shelters one animal.</p>`;
  }
  function companion() {
    const p=me(), seconds=Math.max(0,(p.camelRentalUntil||0)-now()), owned=(p.camels||[]).filter(c=>!c.rentalUntil).length;
    if(!owned&&seconds<=0)return '';
    const nearby=(state.snapshot?.nodes||[]).filter(n=>n.kind==='camel'&&n.owner===p.id&&!!n.inCave===!!p.inCave&&Math.abs((n.y||0)-(p.y||0))<=2&&Math.hypot(n.x-p.x,n.z-p.z)<=5).sort((a,b)=>Math.hypot(a.x-p.x,a.z-p.z)-Math.hypot(b.x-p.x,b.z-p.z))[0];
    return `<div class="section-label island-section">YOUR HERD <span>${owned} owned camel${owned===1?'':'s'}${seconds>0?` · ${Math.ceil(seconds/60)} rental minutes`:''}</span></div><div class="notice good"><p>${p.ridingCamel?'Press <strong>Q</strong> to dismount. Your mounted camel travels with you between islands and home.':'Parked camels wait where you leave them. Walk up to your camel and press <strong>E</strong> to mount.'}</p>${p.ridingCamel?button('Dismount · Q','camel_dismount',{},false,true):nearby?button('Mount nearby camel · E','camel_mount',{target:nearby.id},false,true):''}</div>`;
  }
  function render(type) {
    const panel=({friends:friendsPanel,build:building,build_object:buildObject,chest,storage,animal})[type]?.() ?? null;
    return type==='friends'?panel+camelNames():panel;
  }
  function shop(kind) {
    return ({pink_lollipop:pink,blue_lollipop:blue,abu_fanous:camel})[kind]?.() ?? null;
  }
  function handle(kind,data) {
    if(kind==='island-close'){closeModal();return true;}
    if(kind==='island-buy-home'){action('buy_home');return true;}
    if(buildingPanels.handle(kind,data))return true;
    if(['teleport_home','teleport_main','visit_island','adopt_animal'].includes(kind)){action(kind,data);closeModal();return true;}
    if(kind==='use_build'&&['oven','mixer','table'].includes(island()?.objects?.find(object=>object.id===data.target)?.kind)){const object=island().objects.find(object=>object.id===data.target);openModal('context',{...object,isBuild:true});return true;}
    return false;
  }
  function input(event){if(event.target.id==='friend-code-input')friendCodeDraft=event.target.value;if(event.target.dataset?.camelName)camelNameDrafts.set(event.target.dataset.camelName,event.target.value);}
  function submit(event) {
    if(event.target.dataset?.camelName){event.preventDefault();const target=event.target.dataset.camelName,name=event.target.querySelector('input').value.trim();camelNameDrafts.set(target,name);action('camel_rename',{target,name});return true;}
    if(event.target.id!=='friend-request-form')return false;
    event.preventDefault();const code=$('friend-code-input').value.trim().toUpperCase();
    if(!/^[A-Z]{4}\d{4}$/.test(code)){toast('Enter a four-letter word and four numbers, such as FERN1234.');return true;}
    action('friend_request',{target:code});friendCodeDraft='';renderModal();return true;
  }
  return {render,shop,handle,input,submit,companion};
}
