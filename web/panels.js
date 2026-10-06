import {createIslandPanels} from './island-panels.js';

export function createPanels(deps) {
  const {$, state, action, toast, cookieIcon, materialEmoji, cookieNames, escapeHTML, pretty} = deps;
  const esc = value => escapeHTML(String(value ?? ''));
  const me = () => state.snapshot?.me || {inventory:{},levels:{},coins:0};
  const count = key => Number(me().inventory?.[key] || 0);
  const known = key => !cookieNames[key] || !!me().discovered?.[key];
  const recipeList = () => state.snapshot?.recipes || state.serverRecipes || [];
  const name = key => cookieNames[key] ? (known(key) ? (recipeList().find(r=>r.id===key)?.name || cookieNames[key]) : 'Mystery bake '+(Object.keys(cookieNames).indexOf(key)+1)) : pretty(key);
  const icon = key => cookieNames[key] || key==='fire' ? cookieIcon(key) : `<span class="material-icon" aria-hidden="true">${esc(materialEmoji[key] || '◇')}</span>`;
  const aliases = {craft:'recipes',crafting:'recipes',guide:'help',world:'map',worldmap:'map',context:'shop'};
  const titles = {inventory:'A place for everything',recipes:'Your cookbook',map:'A world worth exploring',help:'The field guide',shop:'A good place to stop',customize:'Make yourself at home',build:'A little place of your own',friends:'Friends & your island',build_object:'Make it your own',chest:'A chest to share',animal:'A little ocean kindness'};
  const areas = {safe:'safeSlots',bag:'bagSlots',hotbar:'hotbar'};
  let previousMarkup='', selectedSlot=null, avatarDraft=null, callNameDraft='', pendingCustomization=null, pendingSnapshot=null, customizationSaved=false;
  function button(label, kind, data={}, disabled=false, accent=false) {
    return `<button type="button" class="small-button${accent?' accent':''}" data-action="${esc(kind)}" data-payload="${esc(JSON.stringify(data))}"${disabled?' disabled':''}>${esc(label)}</button>`;
  }
  const stat = (value,label) => `<div class="stat-card"><strong>${esc(value)}</strong><span>${esc(label)}</span></div>`;
  function shopRow(symbol,label,description,buttons) {
    return `<div class="shop-row"><span class="shop-icon" aria-hidden="true">${esc(symbol)}</span><div class="shop-copy"><h3>${esc(label)}</h3><p>${esc(description)}</p></div><div class="item-actions">${buttons}</div></div>`;
  }
  const islandPanels=createIslandPanels({...deps,button,shopRow,openModal,closeModal,renderModal});
  function slotGrid(area,size) {
    return `<div class="storage-grid storage-${area}" aria-label="${area==='safe'?'Permanent storage':area==='hotbar'?'Protected hotbar':'Inventory'}">${Array.from({length:size},(_,index)=>{
      const stack=me()[areas[area]]?.[index], occupied=stack?.item&&stack.count>0;
      const selected=selectedSlot?.area===area&&selectedSlot.index===index;
      const label=occupied?`${name(stack.item)}, ${stack.count}`:'Empty slot';
      return `<button class="storage-slot${occupied?' occupied':''}${selected?' picked':''}" type="button" data-action="pick-slot" data-payload="${esc(JSON.stringify({area,index}))}" aria-label="${esc(area)} slot ${index+1}: ${esc(label)}${selected?', selected':''}" aria-pressed="${selected}"><span class="storage-number">${index+1}${area==='hotbar'?' · key '+(index+1):''}</span>${occupied?icon(stack.item):'<span class="slot-empty">＋</span>'}<span class="storage-name">${esc(occupied?name(stack.item):'Empty')}</span>${occupied?`<strong class="storage-count">${stack.count}</strong>`:''}</button>`;
    }).join('')}</div>`;
  }
  function inventory() {
    const p=me(), chosen=selectedSlot&&p[areas[selectedSlot.area]]?.[selectedSlot.index];
    let detail='<div class="notice">Click an item, then a destination slot to move the whole stack. Occupied slots swap. Matching stacks merge.</div>';
    if(chosen?.item&&chosen.count>0) {
      let buttons=button('Cancel selection','cancel-slot');
      if(selectedSlot.area==='hotbar') buttons+=button('Select hotbar slot','select',{slot:selectedSlot.index},false,true);
      if(cookieNames[chosen.item]&&selectedSlot.area==='hotbar') buttons+=button('Eat one','eat',{item:chosen.item});
      if(chosen.item==='protein_powder'||chosen.item==='protein_drink') buttons+=button('Use for training','consume_protein',{item:chosen.item});
      detail=`<div class="notice"><strong>${esc(name(chosen.item))} × ${chosen.count}</strong><p>Choose a destination slot to move or swap this stack.</p><div class="item-actions">${buttons}</div></div>`;
    }
    const recovery=Object.entries(p.recovery||{}).filter(([,n])=>n>0);
    const reputation=Object.entries(p.reputation||{}).filter(([,v])=>v!==0).map(([id,v])=>{
      const animal=state.snapshot?.animals?.find(a=>a.id===id);
      return `<span class="reputation-pill${v<0?' bad':''}">${esc(pretty(animal?.species||id))}: ${v>0?'+':''}${v}</span>`;
    }).join('');
    return `<p class="panel-intro">Your permanent storage and hotbar survive every defeat. A player who defeats you can loot five random occupied inventory stacks.</p>${detail}<div class="section-label">PERMANENT STORAGE <span>3 slots · always protected</span></div>${slotGrid('safe',3)}<div class="section-label">HOTBAR <span>5 slots · keys 1–5 · always protected</span></div>${slotGrid('hotbar',5)}${p.homeIsland?`<div class="notice good"><strong>6 - Build menu</strong><p>Your sixth hotbar slot opens Building, Decor, and Appliances on a home island.</p>${button('Open build menu','tab',{tab:'build'})}</div>`:''}<div class="section-label">INVENTORY <span>15 slots · five occupied stacks can be looted</span></div>${slotGrid('bag',15)}${islandPanels.companion()}${recovery.length?`<div class="section-label">SAVED ITEM RECOVERY <span>Your earlier items are safe here until you make room</span></div><div class="shop-list">${recovery.map(([key,n])=>shopRow('◇',name(key),`${n} saved`,button('Move to inventory','claim_recovery',{item:key},false,true))).join('')}</div>`:''}<div class="stats-grid">${stat(p.coins||0,'COINS')}${stat(Object.entries(p.inventory||{}).reduce((sum,[key,n])=>sum+(cookieNames[key]?n:0),0),'BAKED GOODS')}${stat(p.kills||0,'PLAYER DEFEATS')}${stat(p.deaths||0,'TIMES RESPAWNED')}</div><p class="panel-intro">Coin loot uses balances before the defeat: an opponent with fewer coins receives 10% rounded up; otherwise they receive 5% rounded down.</p>${reputation?`<div class="section-label">OCEAN REPUTATION</div><div class="reputation-list">${reputation}</div>`:''}`;
  }
  function recipes() {
    const list=[{id:'fire',name:'Campfire',description:'Build on dry land with room around you. Bake while standing close to its warmth.',cost:{wood:2,stick:2,stone:3}},...recipeList()];
    return `<p class="panel-intro">Every recipe is recorded here. Bake an experiment beside a campfire or a home-island or bakery oven to discover its name and abilities. Discoveries stay in your cookbook permanently.</p><div class="notice">Ingredients are visible from the beginning. Desert recipes still need a fire or oven; cakes require an oven.</div><div class="recipe-grid">${list.map((r,index)=>{
      const discovered=r.id==='fire'||known(r.id), costMap=r.cost||{};
      const canAfford=amount=>Object.entries(costMap).every(([key,n])=>(r.id==='protein_cookie'&&key==='protein_powder'?count('protein_powder')+count('protein_drink'):count(key))>=n*amount);
      const cost=Object.entries(costMap).map(([key,n])=>{
        const protein=r.id==='protein_cookie'&&key==='protein_powder', have=protein?count('protein_powder')+count('protein_drink'):count(key);
        return `<span class="ingredient-chip${have<n?' missing':''}">${n} ${esc(protein?'protein powder or drink':name(key))} · ${have} held</span>`;
      }).join('');
      return `<article class="recipe-card${discovered?'':' recipe-mystery'}"><div class="recipe-top">${icon(r.id)}<div><h3>${esc(discovered?r.name:'Undiscovered recipe '+index)}</h3><span>${r.id==='fire'?'YOUR FIRST KITCHEN':discovered?'SAVED IN YOUR COOKBOOK':'BAKE TO DISCOVER'}</span></div></div><p>${esc(discovered?r.description:'The ingredients are familiar. What they make is yours to discover.')}</p>${discovered&&r.damage?`<div class="recipe-stats"><span>↗ ${r.damage} damage</span><span>♥ ${r.heal} health</span></div>`:''}<div class="recipe-cost">${cost}</div><div class="item-actions">${button(r.id==='fire'?'Build campfire':discovered?'Bake one':'Try this recipe','craft',{item:r.id,amount:1},!canAfford(1),true)}${r.id!=='fire'?button('Bake five','craft',{item:r.id,amount:5},!canAfford(5)):''}</div></article>`;
    }).join('')}</div>`;
  }
  const colors={teal:'#387f79',blue:'#3f68a1',red:'#ba574c',purple:'#806096',sand:'#d7be8c',black:'#363a38',white:'#f5f0dd'};
  function bakeryRows(p) {
    const level=p.bakeryLevel||0, ratio=level/(level+3), upgrade=50+level*25;
    const yieldCount=2+Math.floor(4*ratio), seconds=8*(1-.4*ratio)*(p.equipment?.mixer ? .85 : 1);
    let rows=shopRow('◕','Make fresh dough',`${yieldCount} dough every ${seconds.toFixed(1)} seconds.`,button('Make dough','make_dough',{},false,true));
    rows+=shopRow('↗','Order a delivery','8 dough delivered for 5 coins.',button('Order · 5 coins','order_dough',{amount:1},p.coins<5));
    rows+=shopRow('⌂',`Bakery upgrade ${level+1}`,`${upgrade} coins. Production gradually improves; higher levels give smaller gains. Current batch bonus up to ${(ratio*35).toFixed(0)}%, sale bonus up to ${(ratio*50).toFixed(0)}%.`,button(`Upgrade · ${upgrade}`,'upgrade',{},p.coins<upgrade,true));
    rows+=shopRow('▤','Open your cookbook','Bake by your oven and permanently record each new discovery.',button('Open cookbook','tab',{tab:'recipes'}));
    for(const key of Object.keys(cookieNames)) {
      const base=key==='cake'?12:key==='sugar'?2:4, displayBonus=p.equipment?.display?1:0;
      const saleTotal=amount=>base*amount+Math.floor(base*amount*.5*ratio)+displayBonus*amount;
      rows+=shopRow('¢',`Sell ${name(key)}`,`One sells for ${saleTotal(1)} coins; five sell for ${saleTotal(5)} coins. ${count(key)} held.`,button(`Sell one · ${saleTotal(1)} coins`,'trade',{target:'bakery',item:key,amount:1},count(key)<1)+button(`Sell five · ${saleTotal(5)} coins`,'trade',{target:'bakery',item:key,amount:5},count(key)<5));
    }
    const paint=Object.keys(colors).filter(color=>count('paint_'+color)>0);
    if(paint.length) {
      rows+='<div class="section-label">PERSONALIZE YOUR BAKERY <span>Each change uses one tin of paint</span></div>';
      for(const target of ['walls','oven','mixer','display']) {
        if(['mixer','display'].includes(target)&&!p.equipment?.[target])continue;
        rows+=shopRow('◈',pretty(target),`Current color: ${pretty(p.paint?.[target]||'sand')}`,paint.map(color=>button(pretty(color),'paint',{target,item:color})).join(''));
      }
    }
    return rows;
  }
  function shop() {
    const p=me(), original=state.context||{}, n=state.snapshot?.nodes?.find(node=>node.id===original.id)||original, kind=n.kind||n.id;
    const islandShop=islandPanels.shop(kind);if(islandShop!==null)return islandShop;
    let rows='', intro='Stay close to the station while trading. Your items and coins update when the server completes the trade.';
    if(kind==='peace') {
      intro='Peace pays three coins for every kilogram of beach trash. A clean coast is a good beginning; shells and ocean treasures are welcome, too.';
      for(const [key,price] of Object.entries({trash:3,shell:2,stone:1,pearl:12,treasure:25,old_coin:5,rare_item:60})) {
        const amount=count(key), batch=Math.min(20,amount);
        rows+=shopRow(materialEmoji[key]||'◇',name(key),`${amount} held · ${price} coins ${key==='trash'?'per kg':'each'}`,button('Trade one','trade',{target:'peace',item:key,amount:1},!amount)+button(`Trade ${batch}`,'trade',{target:'peace',item:key,amount:batch||1},batch<2,true));
      }
    } else if(kind==='desert_trader') {
      intro='The village bakers trade ingredients and treasures. Bring a little of the forest or sea to exchange for their unfamiliar bakes.';
      for(const [item,amount,description] of [['berry',3,`2 ${name('sun_cookie')}`],['nut',3,`2 ${name('cactus_cookie')}`],['shell',4,`2 ${name('cactus_cookie')}`],['treasure',1,`3 ${name('sun_cookie')} + 3 ${name('cactus_cookie')}`]]) {
        rows+=shopRow(materialEmoji[item]||'◇',`${amount} ${name(item)}`,`Exchange for ${description}.`,button('Barter','trade',{target:n.id||'desert_trader',item,amount:1},count(item)<amount,true));
      }
    } else if(kind==='gym') {
      intro='Each set costs two coins. Train legs for running speed, stamina for endurance, arms for throws, and chest and back for swimming.';
      for(const [key,note] of Object.entries({legs:'Run faster on land.',stamina:'Sprint for longer.',arms:'Throw faster and harder.',swim:'Swim faster; enough training lets you outrun a shark.'})) {
        const level=p.levels?.[key]||0;
        rows+=shopRow(key==='arms'?'↗':key==='swim'?'≈':'ϟ',pretty(key),`${note} Level ${level}/10.`,button(level>=10?'Maximum level':'Train · 2 coins','train',{item:key},level>=10||p.coins<2,true));
      }
      for(const key of ['protein_powder','protein_drink'])rows+=shopRow('▤',name(key),`${count(key)} held · reduces rest between workouts`,button('Use for training','consume_protein',{item:key},count(key)<1));
    } else if(kind==='vending') {
      intro='Use protein to train more often, or save it to experiment in your cookbook.';
      for(const [key,cost] of [['protein_powder',8],['protein_drink',6]])rows+=shopRow('▤',name(key),`${cost} coins · ${count(key)} held`,button(`Buy · ${cost} coins`,'trade',{target:'vending',item:key,amount:1},p.coins<cost,true)+button('Use','consume_protein',{item:key},count(key)<1));
    } else if(['bakery_plot','land','bakery'].includes(kind)) {
      const isOwn=p.bakeryPlot===n.id || kind==='bakery'&&p.bakery;
      if(isOwn) {intro=`Your bakery · level ${p.bakeryLevel||0}. This plot belongs to ${p.callName||p.name} (@${p.username}).`;rows=bakeryRows(p);}
      else if(n.owner) {intro=`This bakery belongs to ${n.callName||n.username||'another baker'}. Explore the city streets to find an unclaimed plot.`;rows=shopRow('⌂',n.label||'Claimed bakery',n.username?'@'+n.username:'Privately owned','');}
      else {
        intro='Twelve bakery plots line the city streets. Claim an available plot for 250 coins and make it your own.';
        rows=shopRow('⌂',p.bakery?'You already own a bakery':'An oven with your name on it',p.bakery?'Your own bakery is marked on the world map.':'This plot costs 250 coins and can be claimed by one player.',button(p.bakery?'Already own a plot':'Claim bakery · 250','buy_land',{target:n.id},p.bakery||p.coins<250,true));
      }
    } else if(kind==='kitchen_shop'||kind==='paint_shop') {
      intro='Equip your bakery with a mixer, oven, and display. Once your kitchen is fully equipped, this shop supplies paint for your walls and equipment.';
      for(const [key,cost,note] of (kind==='paint_shop'?[]:[['mixer',40,'Make dough 15% sooner.'],['oven',60,'Bake 20% sooner.'],['display',35,'Add one coin to each sale.']])) {
        rows+=shopRow('▣',pretty(key),note,button(p.equipment?.[key]?'Installed':`Buy · ${cost} coins`,'trade',{target:'kitchen_shop',item:key,amount:1},!p.bakery||p.equipment?.[key]||p.coins<cost,true));
      }
      const paintsUnlocked=['mixer','oven','display'].every(key=>p.equipment?.[key]);
      for(const color of Object.keys(colors))rows+=shopRow('◈',pretty(color)+' paint',`12 coins · ${count('paint_'+color)} held${paintsUnlocked?'':' · unlock by equipping your kitchen'}`,button('Buy paint · 12','trade',{target:kind,item:'paint_'+color,amount:1},!paintsUnlocked||p.coins<12));
    } else if(kind==='general_shop') {
      intro='City provisions for your next journey or your next batch.';
      for(const [key,cost] of Object.entries({dough:3,berry:2,nut:2,wood:2,stick:1,stone:1,sea_salt:3}))rows+=shopRow(materialEmoji[key]||'◇',name(key),`${cost} coins each · ${count(key)} held`,button('Buy one','trade',{target:'general_shop',item:key,amount:1},p.coins<cost,true)+button('Buy five','trade',{target:'general_shop',item:key,amount:5},p.coins<cost*5));
    } else if(kind==='outfit_shop') return customize();
    else if(kind==='fire') return recipes();
    else if(kind==='home'||kind==='tent'||kind==='cabin') return islandPanels.render('build');
    else return `<p class="panel-intro">${esc(n.label||'Explore nearby resources and stations.')}</p>${button('Open inventory','tab',{tab:'inventory'})}${button('Open cookbook','tab',{tab:'recipes'})}`;
    return `<p class="panel-intro">${esc(intro)}</p>${kind==='desert_trader'?'<div class="notice good">Village barter · bring ingredients and treasures</div>':`<div class="notice good">${p.coins||0} coins in your pocket</div>`}<div class="shop-list">${rows}</div>`;
  }
  const skinTones=['#f4dac5','#dfb797','#bd8862','#8a583c','#4f3228'];
  const hats={chef:['Chef hat','Discover your first recipe'],recycler:['Recycler cap','Trade 10 kg of trash with Peace'],ocean:['Ocean crown','Help three ocean animals'],champion:['Champion cap','Defeat five players'],builder:['Builder hat','Own a bakery or a home']};
  function customize() {
    const p=me();
    if(!avatarDraft) {avatarDraft={skin:0,shirt:'tee',pants:'trousers',shirtColor:'teal',pantsColor:'sand',hat:'',...p.avatar};callNameDraft=p.callName||p.name||'';}
    const now=state.snapshot?.serverTime||p.serverTime||Date.now()/1000;
    const wait=Math.max(0,(p.callNameChangedAt||0)+3600-now), changedName=callNameDraft.trim()!==(p.callName||p.name||'');
    if(pendingCustomization&&state.snapshot!==pendingSnapshot&&p.callName===pendingCustomization.callName&&Object.entries(pendingCustomization.avatar).every(([key,value])=>p.avatar?.[key]===value)) {pendingCustomization=null;pendingSnapshot=null;customizationSaved=true;}
    const saved=customizationSaved;
    const option=(value,label,current)=>`<option value="${esc(value)}"${value===current?' selected':''}>${esc(label)}</option>`;
    const colorSelect=(key,label)=>`<label>${label}<select data-avatar="${key}">${Object.keys(colors).map(color=>option(color,pretty(color),avatarDraft[key])).join('')}</select></label>`;
    return `<p class="panel-intro">Your account keeps one permanent username. Your call name is the name other players see above you and on your home sign.</p><form id="customize-form"><div class="customize-layout"><div class="avatar-preview" aria-label="Avatar preview"><div class="avatar-preview-figure" style="--avatar-skin:${skinTones[avatarDraft.skin]||skinTones[0]};--avatar-shirt:${colors[avatarDraft.shirtColor]||colors.teal};--avatar-pants:${colors[avatarDraft.pantsColor]||colors.sand}"><div class="preview-hat">${avatarDraft.hat?esc({chef:'♧',recycler:'♻',ocean:'♆',champion:'★',builder:'⌂'}[avatarDraft.hat]||''):''}</div><div class="preview-head"><span>• •</span></div><div class="preview-body ${esc(avatarDraft.shirt)}"><i></i><b></b></div><div class="preview-legs ${esc(avatarDraft.pants)}"><i></i><i></i></div></div><strong id="preview-call-name">${esc(callNameDraft)}</strong><span>@${esc(p.username||'username')}</span></div><div class="customize-fields"><label for="permanent-username">PERMANENT USERNAME</label><input id="permanent-username" value="${esc(p.username)}" readonly aria-readonly="true"><label for="call-name-input">CALL NAME</label><input id="call-name-input" name="callName" maxlength="24" minlength="2" value="${esc(callNameDraft)}" autocomplete="nickname"><p class="field-hint">${wait>0?`Your next call name change is available in ${Math.ceil(wait/60)} minute${Math.ceil(wait/60)===1?'':'s'}.`:'You can change your call name now. After a change, wait one hour.'}</p><div class="section-label">SKIN TONE <span>Choose from five tones</span></div><div class="skin-swatches">${skinTones.map((color,index)=>`<button type="button" class="skin-swatch${avatarDraft.skin===index?' chosen':''}" style="--swatch:${color}" aria-label="Skin tone ${index+1}, ${['very light','light','medium','dark','very dark'][index]}" aria-pressed="${avatarDraft.skin===index}" data-action="avatar-skin" data-payload="${esc(JSON.stringify({skin:index}))}"></button>`).join('')}</div><div class="customize-selects"><label>SHIRT STYLE<select data-avatar="shirt">${[['tee','T-shirt'],['hoodie','Hoodie'],['tank','Tank top']].map(([v,l])=>option(v,l,avatarDraft.shirt)).join('')}</select></label>${colorSelect('shirtColor','SHIRT COLOR')}<label>PANTS STYLE<select data-avatar="pants">${[['trousers','Trousers'],['shorts','Shorts']].map(([v,l])=>option(v,l,avatarDraft.pants)).join('')}</select></label>${colorSelect('pantsColor','PANTS COLOR')}</div></div></div><div class="section-label">ACHIEVEMENT HATS <span>Earn a hat, then wear it</span></div><div class="hat-options">${button(avatarDraft.hat===''?'✓ No hat':'No hat','avatar-hat',{hat:''})}${Object.entries(hats).map(([key,[label,condition]])=>`<div class="hat-option${p.hats?.[key]?'':' locked'}">${button((avatarDraft.hat===key?'✓ ':'')+label,'avatar-hat',{hat:key},!p.hats?.[key])}<span>${p.hats?.[key]?'Unlocked':esc(condition)}</span></div>`).join('')}</div><div class="customize-save"><button type="submit" class="small-button accent"${changedName&&wait>0?' disabled':''}>Save appearance${changedName?' & call name':''}</button><span id="customize-status" role="status">${saved?'Saved to your account.':pendingCustomization?'Waiting for the server to save your changes.':'Changes save when confirmed by the server.'}</span></div></form>`;
  }
  function guide() {
    return `<p class="panel-intro">Ten mystery bakes. Four regions. A cookbook full of discoveries waiting to happen.</p><div class="field-guide"><section class="guide-section"><h3>Your first batch</h3><ol><li>Gather 2 wood, 2 sticks, and 3 stones with <kbd>E</kbd>.</li><li>Find dough in chests or dig pale ground patches.</li><li>Press <kbd>C</kbd> to open your cookbook and build a campfire.</li><li>Bake beside the fire. Your first successful bake reveals that recipe's name and effects permanently.</li></ol></section><section class="guide-section"><h3>Aim, bake, repeat</h3><p><kbd>W A S D</kbd> move · mouse to look.<br><kbd>SHIFT</kbd> sprint on land; train chest and back to swim faster.<br><kbd>LEFT CLICK</kbd> throw · <kbd>Q</kbd> or right click eat.<br><kbd>E</kbd> interact · <kbd>I</kbd> inventory.<br><kbd>C</kbd> cookbook · <kbd>M</kbd> map.<br><kbd>1-5</kbd> select a hotbar slot. <kbd>6</kbd> opens your build menu.<br><kbd>V</kbd> mount or dismount your camel.<br><kbd>ESC</kbd> opens or closes the game menu.</p></section><section class="guide-section"><h3>Your bag and your battles</h3><p>Three permanent slots and five hotbar slots keep their contents through every defeat. Your fifteen inventory slots may lose five random occupied stacks to the player who defeats you. A killer with fewer coins than you receives 10% of your coins rounded up; otherwise they receive 5% rounded down. A headshot awards 5 coins, a hand or foot hit 3, and another body hit 1.</p></section><section class="guide-section"><h3>A place of your own</h3><p>Explore the rock-covered cave near the middle of Sunbaked Sands. Its clay tunnels lead to pink and blue lollipops. Eat the pink one to buy your home island for 200 coins, then choose whether to teleport. Hotbar slot 6 opens Building, Decor, and Appliances. Rotate by 45 degrees, move your creations, or destroy them for half the materials back.</p></section><section class="guide-section"><h3>Village exchanges</h3><p>Look inside the desert's clay village buildings for bakers. They barter for berries, nuts, shells, and ocean treasures. Coins are used in the city and at Peace's beach stand. Bake desert experiments near a fire or oven.</p></section><section class="guide-section"><h3>The ocean remembers</h3><p>Smaller groups swim at their own pace while staying close. Dolphins jump independently. Help sick animals quickly; illness and old age can end their lives. Leave an egg undisturbed for two minutes to let it hatch. Build a pond to adopt a wounded fish or turtle. Peace pays 3 coins per kilogram of beach trash.</p></section><section class="guide-section"><h3>Build, gather, and bake</h3><p>Build an oven and mixer on your home island. Garden beds grow nuts and berries. Chop trees for wood: larger trees take more hits and yield more wood, from three to six pieces. Campfires on the main island disappear after 25 minutes; home-island fires remain until put out or destroyed.</p></section><section class="guide-section"><h3>A familiar face</h3><p>Choose Customize from the Escape menu for five skin tones, shirt and pants styles, colors, and earned achievement hats. Your username stays permanent. You may change your call name once per hour.</p></section><section class="guide-section"><h3>Friends across the water</h3><p>Open Friends from the Escape menu. Every player has a permanent code: a four-letter word and four numbers. Make yours public to display it below your call name. Accept requests, allow island visits, and grant building permission to specific friends. The blue lollipop opens island visits. Share materials in the island chest.</p></section><section class="guide-section"><h3>Travel with Abu Fanous</h3><p>Find Abu Fanous in the far east. Borrow a camel for 20 minutes for 50 coins, or buy one for 350. Bring one of every cookie and cake to lower the permanent price to 175 coins.</p></section></div>`;
  }
  function mapPanel() {
    if(me().island)return `<p class="panel-intro">${me().island===me().id?'Your home island':'A friend’s home island'}. Furnishings and visitors appear below. Open Home island to build, or Friends to manage visits.</p><div class="map-canvas-wrap"><canvas id="world-map" width="880" height="650" aria-label="Home island map with placed objects and players"></canvas></div><div class="item-actions">${button('Home island','tab',{tab:'build'})}${button('Return to main island','teleport_main')}</div>`;
    return `<p class="panel-intro">You are the golden arrow. Explore the cave in Sunbaked Sands for your first home island and friend visits. Look for Abu Fanous in the far east.</p><div class="map-canvas-wrap"><canvas id="world-map" width="880" height="650" aria-label="Map of four regions, coastline, the desert cave, shops, stations, and players"></canvas></div><div class="map-legend"><div><h3><i class="biome-dot forest"></i>Wildwood</h3><p>Forest ingredients, dough, and trees to chop.</p></div><div><h3><i class="biome-dot desert"></i>Sunbaked Sands</h3><p>Clay villages, cave lollipops, and Abu Fanous.</p></div><div><h3><i class="biome-dot ocean"></i>The Blue</h3><p>Peace, winding beaches, and open ocean.</p></div><div><h3><i class="biome-dot city"></i>Crumb City</h3><p>City gardens, bakery businesses, shops, and the gym.</p></div></div><div class="map-key"><span><b>▲</b> You</span><span><b>●</b> Players</span><span><b>◆</b> Stations</span><span><b style="color:#487d4d">&#9632;</b> Available bakery</span><span><b style="color:#996659">&#9632;</b> Claimed bakery</span></div>`;
  }
  function paintMap(canvas,mini=false) {
    if(!canvas)return;
    if(me().island){paintIslandMap(canvas,mini);return;}
    const ctx=canvas.getContext('2d');if(!ctx)return;
    const layout=state.snapshot?.layout||state.layout||{}, minX=layout.minX??-240,maxX=layout.maxX??240,minZ=layout.minZ??-260,maxZ=layout.maxZ??200;
    const width=canvas.width,height=canvas.height,margin=mini?0:25,mw=width-margin*2,mh=height-margin*2;
    const px=x=>margin+(x-minX)/(maxX-minX)*mw,pz=z=>margin+(z-minZ)/(maxZ-minZ)*mh;
    const rect=(x,z,w,d)=>ctx.fillRect(px(x-w/2),pz(z-d/2),w/(maxX-minX)*mw,d/(maxZ-minZ)*mh);
    ctx.clearRect(0,0,width,height);ctx.fillStyle='#e4dec3';ctx.fillRect(0,0,width,height);
    ctx.save();ctx.beginPath();ctx.rect(margin,margin,mw,mh);ctx.clip();
    ctx.fillStyle='#6f8d61';ctx.fillRect(margin,margin,mw,mh);
    ctx.fillStyle='#ddb882';ctx.fillRect(px(0),pz(minZ),px(maxX)-px(0),pz(0)-pz(minZ));
    ctx.fillStyle='#c1bca2';ctx.fillRect(px(0),pz(0),px(maxX)-px(0),pz(maxZ)-pz(0));
    const coast=layout.coast?.length?layout.coast:[{x:minX,z:22},{x:-180,z:42},{x:-100,z:25},{x:-50,z:42},{x:0,z:22}];
    const oceanShape=(offset)=>{ctx.beginPath();ctx.moveTo(px(minX),pz(maxZ));for(const point of coast)ctx.lineTo(px(point.x),pz(point.z+offset));ctx.lineTo(px(0),pz(maxZ));ctx.closePath();};
    ctx.fillStyle='#c3cb8e';oceanShape(-22);ctx.fill();ctx.fillStyle='#e4d4a0';oceanShape(-15);ctx.fill();
    const sea=ctx.createLinearGradient(0,pz(15),0,pz(100));sea.addColorStop(0,'#8ac8c2');sea.addColorStop(1,'#4b879d');ctx.fillStyle=sea;oceanShape(0);ctx.fill();
    ctx.strokeStyle='#eff3d199';ctx.lineWidth=mini?1:3;ctx.beginPath();for(const [i,point]of coast.entries()){if(i===0)ctx.moveTo(px(point.x),pz(point.z));else ctx.lineTo(px(point.x),pz(point.z));}ctx.stroke();
    ctx.fillStyle='#e7d2a3';for(const path of layout.paths||[])rect(path.x,path.z,path.width,path.depth);
    for(const prop of layout.props||[]) {
      if(prop.kind==='tree'||prop.kind==='bush') {ctx.fillStyle=prop.kind==='tree'?'#365f3c88':'#87a166';ctx.beginPath();ctx.arc(px(prop.x),pz(prop.z),mini?1:Math.max(1,(prop.width||3)/(maxX-minX)*mw*.5),0,Math.PI*2);ctx.fill();}
      else if(['building','wall','village','desert_building','gym','shop','kitchen_shop','general_shop','outfit_shop','paint_shop'].includes(prop.kind)) {ctx.fillStyle='#a58762';rect(prop.x,prop.z,prop.width||4,prop.depth||4);}
    }
    for(const zone of layout.zones||[]) {
      if(!/build|clear|home/i.test(zone.id+' '+zone.name))continue;
      ctx.fillStyle='#eff1bd50';rect(zone.x,zone.z,zone.width,zone.depth);ctx.strokeStyle='#f7f4cb';ctx.lineWidth=mini?1:1.4;ctx.setLineDash([3,3]);ctx.strokeRect(px(zone.x-zone.width/2),pz(zone.z-zone.depth/2),zone.width/(maxX-minX)*mw,zone.depth/(maxZ-minZ)*mh);ctx.setLineDash([]);
      if(!mini) {ctx.font='9px Arial';ctx.textAlign='center';ctx.fillStyle='#fff8df';ctx.fillText(zone.name||'Building clearing',px(zone.x),pz(zone.z));}
    }
    const nodes=state.snapshot?.nodes||[];
    for(const plot of layout.plots||[]) {
      const node=nodes.find(n=>n.id===plot.id), mine=me().bakeryPlot===plot.id;
      ctx.fillStyle=mine?'#f4bc60':node?.owner?'#996659':'#487d4d';rect(plot.x,plot.z,plot.width||12,plot.depth||12);
      if(!mini) {ctx.fillStyle='#304c38';ctx.font='8px Arial';ctx.textAlign='center';ctx.fillText(mine?'Your bakery':node?.callName||'Available',px(plot.x),pz(plot.z)+13);}
    }
    const stationNames={pink_lollipop:'Home island',blue_lollipop:'Friend visits',abu_fanous:'Abu Fanous',peace:'Peace',desert_trader:'Village barter',gym:'Gym',kitchen_shop:'Kitchen shop',general_shop:'Provisions',paint_shop:'Paint',outfit_shop:'Outfits',vending:'Protein',dummy:'Practice'};
    for(const node of nodes) {
      const label=stationNames[node.kind];if(!label&&node.kind!=='fire')continue;
      const x=px(node.x),y=pz(node.z);ctx.fillStyle=node.kind==='fire'?'#c26c37':'#68573c';ctx.save();ctx.translate(x,y);ctx.rotate(Math.PI/4);ctx.fillRect(-2,-2,mini?4:6,mini?4:6);ctx.restore();
      if(!mini&&label&&node.kind!=='vending') {ctx.font='9px Arial';ctx.textAlign='left';ctx.fillStyle='#354735';ctx.fillText(label,x+6,y-5);}
    }
    for(const home of state.snapshot?.homes||[]) {
      const x=px(home.x),y=pz(home.z);ctx.fillStyle=home.owner===me().id?'#f7ce71':'#f6ead1';ctx.beginPath();ctx.moveTo(x,y-5);ctx.lineTo(x+5,y);ctx.lineTo(x+4,y+4);ctx.lineTo(x-4,y+4);ctx.lineTo(x-5,y);ctx.closePath();ctx.fill();
      if(home.safe) {ctx.beginPath();ctx.strokeStyle='#d8f5aa';ctx.lineWidth=1;ctx.arc(x,y,mini?4:8,0,Math.PI*2);ctx.stroke();}
      if(!mini){ctx.font='9px Arial';ctx.fillStyle='#fdf5dc';ctx.textAlign='center';ctx.fillText(home.callName||home.username,x,y+15);}
    }
    if(!mini) {ctx.font='bold 15px "Trebuchet MS",Arial';ctx.textAlign='center';for(const [label,x,z,color]of [['THE WILDWOOD',-120,-225,'#f4edd1'],['SUNBAKED SANDS',120,-225,'#805e39'],['THE BLUE',-120,155,'#e1f2e2'],['CRUMB CITY',120,183,'#4d5748']]){ctx.fillStyle=color;ctx.fillText(label,px(x),pz(z));}}
    for(const player of state.snapshot?.players||[]) {ctx.fillStyle='#fff5d9';ctx.beginPath();ctx.arc(px(player.x),pz(player.z),mini?2:3.5,0,Math.PI*2);ctx.fill();ctx.strokeStyle='#527256';ctx.lineWidth=1;ctx.stroke();}
    const p=state.snapshot?.me||{x:-35,z:-35,yaw:0};ctx.save();ctx.translate(px(p.x),pz(p.z));ctx.rotate(-(p.yaw||0));ctx.fillStyle='#ffcb71';ctx.strokeStyle='#6b542c';ctx.lineWidth=1.5;const r=mini?6:9;ctx.beginPath();ctx.moveTo(0,-r);ctx.lineTo(r*.6,r*.65);ctx.lineTo(0,r*.3);ctx.lineTo(-r*.6,r*.65);ctx.closePath();ctx.fill();ctx.stroke();ctx.restore();ctx.restore();
    if(!mini) {ctx.font='bold 11px Arial';ctx.textAlign='center';ctx.fillStyle='#5a654f';ctx.fillText('N',width/2,16);ctx.fillText('S',width/2,height-8);ctx.strokeStyle='#60754b55';ctx.strokeRect(margin,margin,mw,mh);}
  }
  function paintIslandMap(canvas,mini=false) {
    const ctx=canvas.getContext('2d');if(!ctx)return;
    const width=canvas.width,height=canvas.height,scale=Math.min(width,height)/100;
    const px=x=>width/2+x*scale,pz=z=>height/2+z*scale;
    ctx.clearRect(0,0,width,height);ctx.fillStyle='#4b879d';ctx.fillRect(0,0,width,height);
    for(const [radius,color]of [[42,'#8ac8c2'],[38,'#e4d4a0'],[33,'#86a565']]){ctx.fillStyle=color;ctx.beginPath();ctx.arc(width/2,height/2,radius*scale,0,Math.PI*2);ctx.fill();}
    for(const object of state.snapshot?.island?.objects||[]) {
      const recipe=state.snapshot?.buildCatalog?.find(recipe=>recipe.id===object.kind);
      ctx.save();ctx.translate(px(object.x),pz(object.z));ctx.rotate(-(object.rotation||0)*Math.PI/180);ctx.fillStyle=object.kind==='pond'?'#679daf':object.kind==='garden_bed'?'#6d633c':object.kind==='chest'?'#d8ac68':'#a78660';
      ctx.fillRect(-(recipe?.width||1)*scale/2,-(recipe?.depth||1)*scale/2,Math.max(2,(recipe?.width||1)*scale),Math.max(2,(recipe?.depth||1)*scale));ctx.restore();
      if(!mini){ctx.font='10px Arial';ctx.fillStyle='#314931';ctx.textAlign='center';ctx.fillText(recipe?.name||pretty(object.kind),px(object.x),pz(object.z)+12);}
    }
    for(const player of state.snapshot?.players||[]){ctx.fillStyle='#fff5d9';ctx.beginPath();ctx.arc(px(player.x),pz(player.z),mini?2:4,0,Math.PI*2);ctx.fill();}
    const p=me(),r=mini?6:10;ctx.save();ctx.translate(px(p.x),pz(p.z));ctx.rotate(-(p.yaw||0));ctx.fillStyle='#ffcb71';ctx.strokeStyle='#6b542c';ctx.lineWidth=1.5;ctx.beginPath();ctx.moveTo(0,-r);ctx.lineTo(r*.6,r*.65);ctx.lineTo(0,r*.3);ctx.lineTo(-r*.6,r*.65);ctx.closePath();ctx.fill();ctx.stroke();ctx.restore();
    if(!mini){ctx.font='bold 15px Arial';ctx.fillStyle='#fff5d9';ctx.textAlign='center';ctx.fillText(p.island===p.id?'YOUR HOME ISLAND':'A FRIEND’S HOME ISLAND',width/2,28);}
  }
  function drawWorldMap(){paintMap($('world-map'));}
  function drawMinimap(){paintMap($('minimap'),true);}
  function renderModal() {
    if(!state.modal)return;
    const tab=aliases[state.modal]||state.modal;state.modal=tab;
    $('modal-title').textContent=tab==='shop'?(state.context?.label||titles.shop):titles[tab]||titles.help;
    $('modal-eyebrow').textContent=tab==='shop'?'NEIGHBORS & NEW BEGINNINGS':tab==='recipes'?'LEARN BY BAKING':'YOUR LITTLE CORNER OF THE WORLD';
    const tabs=[['inventory','Inventory'],['recipes','Cookbook'],['build','Home island'],['friends','Friends'],['map','World map'],['help','Field guide']];
    if(tab==='shop')tabs.unshift(['shop','This station']);
    if(tab==='customize')tabs.unshift(['customize','Customize']);
    $('modal-tabs').innerHTML=tabs.map(([key,label])=>`<button class="modal-tab${tab===key?' active':''}" data-tab="${key}" type="button">${label}</button>`).join('');
    $('modal-footer-text').textContent=state.snapshot?`${me().coins||0} coins · Progress saves automatically. The world keeps moving while this panel is open.`:'Sign in to explore, gather, bake, and play.';
    const markup=tab==='inventory'?inventory():tab==='recipes'?recipes():tab==='map'?mapPanel():tab==='shop'?shop():tab==='customize'?customize():(islandPanels.render(tab) ?? guide());
    if(markup!==previousMarkup) {
      const content=$('modal-content'),scroll=content.scrollTop,focused=document.activeElement;
      const focusID=content.contains(focused)&&focused.id?focused.id:null;
      const caret=focusID&&typeof focused.selectionStart==='number'?[focused.selectionStart,focused.selectionEnd]:null;
      content.innerHTML=markup;content.scrollTop=scroll;previousMarkup=markup;
      if(focusID&&$(focusID)){const field=$(focusID);field.focus({preventScroll:true});if(caret&&field.setSelectionRange)field.setSelectionRange(...caret);}
    }
    if(tab==='map')drawWorldMap();
  }
  function openModal(type='inventory',context=null) {
    const next=aliases[type]||type;
    if(next==='customize'&&state.modal!=='customize'){avatarDraft=null;pendingCustomization=null;customizationSaved=false;}
    state.modal=next;if(context)state.context=context;
    state.menuOpen=false;state.keys?.clear();state.wasModalLocked=!!document.pointerLockElement;
    if(document.pointerLockElement)document.exitPointerLock();
    $('play-overlay').hidden=true;$('modal').hidden=false;previousMarkup='';$('modal-content').scrollTop=0;renderModal();$('close-modal').focus();
  }
  function closeModal({resume=true}={}) {
    state.modal=null;state.context=null;selectedSlot=null;$('modal').hidden=true;previousMarkup='';
    if(resume&&(state.player||state.snapshot)) {
      const resumeGame=deps.resume||deps.resumeFromPanel;
      if(resumeGame)resumeGame();
      else if($('world')?.requestPointerLock){const result=$('world').requestPointerLock();result?.catch?.(()=>{});}
    }
  }
  function toggleModal(type,context=null){if(state.modal===(aliases[type]||type))closeModal();else openModal(type,context);}
  $('modal-tabs').addEventListener('click',event=>{const tab=event.target.closest('[data-tab]');if(tab)openModal(tab.dataset.tab);});
  $('close-modal').addEventListener('click',()=>closeModal());
  $('friends-button')?.addEventListener('click',()=>openModal('friends'));
  $('modal').addEventListener('click',event=>{if(event.target===$('modal'))closeModal();});
  $('modal-content').addEventListener('click',event=>{
    const target=event.target.closest('[data-action]');if(!target||target.disabled)return;
    let data={};try{data=JSON.parse(target.dataset.payload||'{}');}catch{return;}
    const kind=target.dataset.action;
    if(kind==='tab'){openModal(data.tab);return;}
    if(islandPanels.handle(kind,data))return;
    if(!state.snapshot){toast('Sign in to begin your adventure.');return;}
    if(kind==='pick-slot'){
      const stack=me()[areas[data.area]]?.[data.index];
      if(!selectedSlot){if(stack?.item&&stack.count>0)selectedSlot=data;}
      else if(selectedSlot.area===data.area&&selectedSlot.index===data.index)selectedSlot=null;
      else {action('move_item',{from:selectedSlot.area,fromSlot:selectedSlot.index,to:data.area,toSlot:data.index});selectedSlot=null;}
      renderModal();return;
    }
    if(kind==='cancel-slot'){selectedSlot=null;renderModal();return;}
    if(kind==='avatar-skin'||kind==='avatar-hat'){
      if(avatarDraft){avatarDraft={...avatarDraft,...data};customizationSaved=false;renderModal();}return;
    }
    action(kind,data);
  });
  $('modal-content').addEventListener('input',event=>{
    islandPanels.input(event);
    if(event.target.id==='call-name-input'){
      callNameDraft=event.target.value;customizationSaved=false;
      if($('preview-call-name'))$('preview-call-name').textContent=callNameDraft;
      const submit=$('customize-form')?.querySelector('[type="submit"]'),p=me(),now=state.snapshot?.serverTime||p.serverTime||Date.now()/1000;
      if(submit)submit.disabled=callNameDraft.trim()!==(p.callName||p.name||'')&&(p.callNameChangedAt||0)+3600>now;
    }
  });
  $('modal-content').addEventListener('change',event=>{
    if(event.target.dataset.avatar&&avatarDraft){avatarDraft[event.target.dataset.avatar]=event.target.value;customizationSaved=false;renderModal();}
  });
  $('modal-content').addEventListener('submit',event=>{
    if(islandPanels.submit(event))return;
    if(event.target.id!=='customize-form')return;event.preventDefault();
    if(!state.snapshot||!avatarDraft)return;
    callNameDraft=$('call-name-input').value;
    pendingCustomization={callName:callNameDraft.trim(),avatar:{...avatarDraft}};
    pendingSnapshot=state.snapshot;customizationSaved=false;action('customize',pendingCustomization);renderModal();
  });
  if(!document.getElementById('panels-v2-style')) {
    const style=document.createElement('style');style.id='panels-v2-style';
    style.textContent=`
      .storage-grid{display:grid;gap:10px;margin:12px 0 26px}.storage-safe{grid-template-columns:repeat(3,1fr)}.storage-hotbar,.storage-bag{grid-template-columns:repeat(5,1fr)}
      .storage-slot{position:relative;min-height:108px;padding:27px 8px 12px;border:1px solid #d4d8c4;border-radius:10px;background:#eef0e2;color:#425d3c;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:9px;transition:border-color .15s,background .15s,transform .15s}
      .storage-slot:hover{background:#e4e9d6;transform:translateY(-2px);border-color:#829269}.storage-slot:focus-visible{outline:3px solid #d48b50;outline-offset:2px}.storage-slot.picked{background:#f6dfb9;border:2px solid #c97b40;box-shadow:0 0 0 3px #d48b5025}.storage-safe .storage-slot{background:#e1e8cf;border-color:#b9c89c}.storage-hotbar .storage-slot{background:#ede4ce;border-color:#d8c79e}.storage-slot.picked{background:#f6dfb9;border-color:#c97b40}
      .storage-number{position:absolute;top:9px;left:10px;font-size:9px;opacity:.62}.storage-name{font-size:10px;line-height:1.4;max-width:100%;overflow-wrap:anywhere}.storage-count{position:absolute;right:10px;top:9px;font-size:11px;color:#687446}.storage-slot .cookie-icon{width:32px;height:32px;flex-shrink:0}.slot-empty{font-size:21px;opacity:.25}.storage-slot:not(.occupied) .storage-name{opacity:.5}.notice p{margin:7px 0;font-size:11px}.notice .item-actions{margin-top:10px}.recipe-mystery{background:linear-gradient(135deg,#f6f1e3,#eeeadb)}.recipe-mystery .recipe-top h3{font-style:italic}.modal-tabs{overflow-x:auto;flex-shrink:0}.modal-tab{white-space:nowrap}.map-key{flex-wrap:wrap;row-gap:9px}
      .customize-layout{display:grid;grid-template-columns:210px 1fr;gap:28px;margin-bottom:25px}.avatar-preview{border:1px solid #d5dcc4;border-radius:14px;background:radial-gradient(ellipse at 50% 32%,#f7f1da,#dce5ca);padding:25px 14px;display:flex;align-items:center;flex-direction:column;justify-content:center;min-height:335px;text-align:center}.avatar-preview>strong{font:20px Georgia,serif;margin-top:20px;overflow-wrap:anywhere;max-width:100%}.avatar-preview>span{font-size:10px;color:#85926f;margin-top:7px}.avatar-preview-figure{height:230px;width:125px;position:relative;filter:drop-shadow(4px 7px 0 #68795220)}.preview-hat{height:36px;text-align:center;font-size:32px;line-height:1;color:#bd7641;position:relative;z-index:2}.preview-head{width:53px;height:58px;margin:0 auto;background:var(--avatar-skin);border-radius:20px 20px 24px 24px;position:relative;border-bottom:5px solid #00000012}.preview-head span{position:absolute;top:18px;left:14px;letter-spacing:6px;font:bold 14px Arial;color:#3b3027}.preview-head:after{content:'';position:absolute;width:10px;height:4px;border-bottom:2px solid #7d4f3b;left:21px;bottom:12px;border-radius:50%}.preview-body{position:relative;background:var(--avatar-shirt);width:66px;height:72px;margin:4px auto 0;border-radius:17px 17px 8px 8px;border-bottom:6px solid #00000012}.preview-body i,.preview-body b{position:absolute;top:5px;width:21px;height:70px;background:linear-gradient(var(--avatar-shirt) 45%,var(--avatar-skin) 46%);border-radius:12px}.preview-body i{left:-19px;transform:rotate(8deg)}.preview-body b{right:-19px;transform:rotate(-8deg)}.preview-body.hoodie:after{content:'';position:absolute;top:6px;left:17px;width:30px;height:10px;border:4px solid #ffffff3a;border-radius:3px 3px 12px 12px}.preview-body.hoodie i,.preview-body.hoodie b{background:linear-gradient(var(--avatar-shirt) 86%,var(--avatar-skin) 87%)}.preview-body.tank i,.preview-body.tank b{background:var(--avatar-skin);width:16px}.preview-legs{display:flex;gap:7px;justify-content:center;height:61px}.preview-legs i{width:26px;height:100%;background:var(--avatar-pants);border-radius:0 0 5px 5px;border-bottom:8px solid #45473b}.preview-legs.shorts i{background:linear-gradient(var(--avatar-pants) 50%,var(--avatar-skin) 51%)}
      .customize-fields label{font-size:9px;letter-spacing:1px;margin-top:17px;margin-bottom:8px}.customize-fields>label:first-child{margin-top:0}.customize-fields input[readonly]{background:#e9ecdf;color:#8b917c}.field-hint{font-size:10px;color:#818d74;line-height:1.65;margin:9px 0 18px}.skin-swatches{display:flex;gap:13px;margin:12px 0 22px}.skin-swatch{width:39px;height:39px;border-radius:50%;background:var(--swatch);border:3px solid #fff8e7;outline:1px solid #cccbb6}.skin-swatch.chosen{outline:3px solid #5a7947;box-shadow:0 0 0 6px #5a79471a}.customize-selects{display:grid;grid-template-columns:1fr 1fr;gap:0 15px}.customize-selects select{display:block;width:100%;padding:10px;margin-top:8px;border:1px solid #d6dbc7;border-radius:6px;background:#fffcf0;color:#4c6240;font:12px Arial}.hat-options{display:flex;flex-wrap:wrap;gap:12px;align-items:flex-start;margin:14px 0 26px}.hat-option{display:flex;flex-direction:column;gap:8px;max-width:160px}.hat-option span{font-size:9px;line-height:1.6;color:#889379}.hat-option.locked{opacity:.65}.customize-save{display:flex;align-items:center;gap:18px;padding-top:18px;border-top:1px solid #dce1cb}.customize-save>span{font-size:10px;line-height:1.6;color:#7c896c}
      @media(max-width:700px){.storage-grid{gap:6px}.storage-slot{padding:26px 4px 9px;min-height:90px}.storage-slot .cookie-icon{width:27px;height:27px}.storage-name{font-size:8px}.storage-number{font-size:7px;left:6px}.storage-count{font-size:9px;right:6px}.customize-layout{grid-template-columns:1fr;gap:20px}.avatar-preview{min-height:260px;padding:15px}.avatar-preview-figure{transform:scale(.85);margin-top:-15px;margin-bottom:-20px}.avatar-preview>strong{margin-top:5px}.customize-save{align-items:flex-start;flex-direction:column}.skin-swatches{gap:14px}.modal-tabs{gap:17px}.section-label{flex-wrap:wrap;gap:5px}}
    `;document.head.appendChild(style);
  }
  return {renderModal,openModal,closeModal,toggleModal,drawWorldMap,drawMinimap};
}
