import {rodentSpeciesForItem} from './rodents-panels.js';

export function createNutIslandPanels({state,button,escapeHTML:esc,pretty=value=>String(value).replaceAll('_',' ')}){
  const me=()=>state.snapshot?.me||{},view=()=>state.snapshot?.nutIsland;
  const own=()=>!!view()&&view().owner===me().id&&me().island==='nut:'+me().id;
  const available=()=>state.connected!==false&&!me().fallen;
  const name=species=>state.snapshot?.rodentCatalog?.find(c=>c.id===species)?.name||pretty(species);
  const reach=(point,radius)=>Math.hypot(me().x-point.x,me().z-point.z)<=radius&&Math.abs((me().y||0)-(point.y||0))<=2.5;
  const nearPet=pet=>own()&&available()&&reach(pet,pet.cageId?5:4);
  function entry(){
    const p=me(),unlocked=!!p.nutIsland,near=!p.island&&p.inCave&&p.grounded!==false&&Math.hypot(p.x-120,p.z+126)<=4&&Math.abs((p.y||0)+24)<=2.5;
    return `<div class="island-welcome"><h3>A little island for little companions</h3><p>A quiet garden, shady groves, and room for the animals you catch. Let them roam freely or build roomy cages.</p></div>${unlocked?`<div class="notice good"><strong>Your nut island is ready</strong><p>Travel there to release animals from your inventory and care for your collection.</p>${button('Go to my nut island','teleport_nut_island',{},!available(),true)}</div>`:`<div class="notice"><strong>Unlock your nut island · 1,000 coins</strong><p>Visit the green lollipop in the Clay Grotto. The island is yours permanently; cages are purchased separately.</p>${button('Unlock nut island · 1,000 coins','nut_island_unlock',{target:'green_lollipop'},!available()||!near||(p.coins||0)<1000,true)}</div>`}`;
  }
  function petControls(pet){
    const cages=view()?.cages||[],near=nearPet(pet);
    return `<div class="item-actions">${pet.cageId?button('Let roam freely','nut_pet_free',{target:pet.id},!near):''}${button('Return to inventory','nut_pet_pack',{target:pet.id},!near)}${cages.filter(cage=>cage.id!==pet.cageId).map((cage,i)=>{const count=(view().pets||[]).filter(p=>p.cageId===cage.id).length;return button('Move to cage '+(cages.indexOf(cage)+1),'nut_pet_cage',{target:pet.id,item:cage.id},!near||count>=(cage.capacity||4));}).join('')}</div>`;
  }
  function render(){
    if(!own())return entry();
    const island=view(),pets=island.pets||[],cages=island.cages||[],price=island.cagePrice||150,max=island.maxCages||8,nearShop=reach({x:4,z:22},5),selected=state.context?.id;
    const chosenPet=pets.find(p=>p.id===selected),chosenCage=cages.find(c=>c.id===selected);
    const shown=chosenPet?[chosenPet]:chosenCage?pets.filter(p=>p.cageId===chosenCage.id):pets;
    return `<div class="island-welcome"><h3>${chosenPet?esc(name(chosenPet.species)):chosenCage?'A roomy little home':'Your nut island'}</h3><p>${pets.length} / ${island.maxPets||100} companions · ${cages.length} / ${max} cages. Animals roam across the garden unless assigned to a cage.</p></div><div class="notice"><strong>Cage workshop · ${price} coins</strong><p>Each cage holds four companions. Buy one near the workshop by the arrival path, then assign an animal while standing beside it.</p>${button('Build next cage · '+price+' coins','nut_cage_buy',{},!available()||!nearShop||cages.length>=max||(me().coins||0)<price,true)}</div><div class="shop-list">${shown.map(pet=>`<article class="shop-row"><div><h3>${esc(name(pet.species))}</h3><p>${pet.cageId?'Cage '+(cages.findIndex(c=>c.id===pet.cageId)+1):'Roaming freely'}${nearPet(pet)?'':' · Move within four metres to care for this animal.'}</p>${petControls(pet)}</div></article>`).join('')||'<p class="empty-message">Catch an animal with H, then select it in Inventory and choose Release here.</p>'}</div><div class="item-actions">${button('Return to the Clay Grotto','nut_island_leave',{},!available())}</div>`;
  }
  function inventoryControls(item){
    const species=rodentSpeciesForItem(item);if(!species)return '';
    if(!own())return `<p class="field-hint">Release this companion on your nut island.</p>${me().nutIsland?button('Go to my nut island','teleport_nut_island',{},!available(),true):'<p class="field-hint">The green lollipop in the Clay Grotto unlocks your own nut island.</p>'}`;
    const island=view(),pets=island.pets||[],has=(me().inventory?.[item]||0)>0,full=pets.length>=(island.maxPets||100);
    return `<div class="notice good"><strong>A home for ${esc(name(species))}</strong><p>Release one from your inventory. A free companion explores the garden; a nearby cage keeps up to four together.</p><div class="item-actions">${button('Release here · roam freely','rodent_release',{item},!available()||!has||full,true)}${(island.cages||[]).map((cage,i)=>button('Release into cage '+(i+1),'rodent_release',{item,target:cage.id},!available()||!has||full||!reach(cage,5)||pets.filter(p=>p.cageId===cage.id).length>=(cage.capacity||4))).join('')}</div></div>`;
  }
  function travelControls(){
    const p=me(),blocked=!available()||!!state.snapshot?.rollerCoaster?.ride||!!p.island&&p.island!==p.id&&!p.island.startsWith('nut:')&&!state.snapshot?.island;
    return own()?button('Return from nut island','nut_island_leave',{},blocked):p.nutIsland?button('Go to my nut island','teleport_nut_island',{},blocked,true):'<p class="field-hint">Unlock a nut island for 1,000 coins at the green lollipop in the Clay Grotto.</p>';
  }
  return {render,inventoryControls,travelControls,shop:kind=>['green_lollipop','nut_cage_shop','nut_cage','nut_pet'].includes(kind)?render():null};
}
