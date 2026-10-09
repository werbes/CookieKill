const names={wood_mouse:'Wood mouse',house_mouse:'House mouse',rat:'Rat',squirrel:'Squirrel',cactus_mouse:'Cactus mouse',blind_albino_rat:'Blind albino rat',flying_squirrel:'Flying squirrel',beaver:'Beaver',hamster:'Hamster',otter:'Otter',chipmunk:'Chipmunk',mole_rat:'Mole rat'};
const icons={wood_mouse:'🐭',house_mouse:'🐭',rat:'🐀',blind_albino_rat:'🐁',squirrel:'🐿️',flying_squirrel:'🐿️',cactus_mouse:'🐭',beaver:'🦫',hamster:'🐹',otter:'🦦',chipmunk:'🐿️',mole_rat:'🐾'};
const blocked=new Set(['hidden','diving','burrowing','rainbow']);
const rarityOrder={common:0,uncommon:1,rare:2,legendary:3,mythic:4};
export const rodentSpeciesForItem=item=>typeof item==='string'&&item.startsWith('pet_')&&names[item.slice(4)]?item.slice(4):null;
function reachableAnimals(snapshot){
  const me=snapshot?.me;if(!me||me.fallen||me.inCave||snapshot.rollerCoaster?.ride)return [];
  return (snapshot.rodents||[]).filter(animal=>(animal.island||'')===(me.island||'')&&!blocked.has(animal.state))
    .map(animal=>({animal,distance:Math.hypot(animal.x-me.x,(animal.y||0)-(me.y||0),animal.z-me.z)}))
    .filter(({animal,distance})=>distance<=(animal.captureRadius||snapshot.rodentCatalog?.find(c=>c.id===animal.species)?.captureRadius||5))
    .sort((a,b)=>a.distance-b.distance).map(({animal})=>animal);
}
const prepared=animal=>animal.species!=='otter'||(animal.nutFed&&animal.state==='confused');
export function nearestCatchable(snapshot){return reachableAnimals(snapshot).find(prepared)||null;}
export function rodentCaptureHint(snapshot){
  const animals=reachableAnimals(snapshot),animal=animals.find(prepared);
  if(animal)return 'H · Catch '+(animal.name||names[animal.species]||'animal')+' · '+(animal.rarity||'common');
  const otter=animals.find(animal=>animal.species==='otter');
  return otter?'Otter · '+(otter.nutFed?'Cosmic cake next':'Nut crunch first'):'';
}
export function createRodentCaptureHUD({document,state,action}){
  const root=document.createElement('button');root.id='rodent-capture-hud';root.type='button';root.hidden=true;document.body.appendChild(root);
  Object.assign(root.style,{position:'fixed',zIndex:'7',left:'50%',top:'52%',transform:'translateX(-50%)',padding:'10px 16px',border:'1px solid #c9dca5',borderRadius:'9px',background:'#254b39ed',color:'#fff0cc',font:'600 12px sans-serif',whiteSpace:'nowrap'});
  function catchNearest(){if(!state.connected||!state.locked||state.modal||state.menuOpen)return false;const animal=nearestCatchable(state.snapshot);if(!animal)return false;action('catch_rodent',{target:animal.id});return true;}
  root.addEventListener('click',catchNearest);
  function update(){const hint=rodentCaptureHint(state.snapshot);root.hidden=!hint||!state.connected||!state.hasPlayed||!state.locked||!!state.modal||!!state.menuOpen;root.textContent=hint;root.disabled=!nearestCatchable(state.snapshot);}
  return {root,update,catchNearest,dispose(){root.remove();}};
}
export function createRodentsPanels({state,escapeHTML:esc,pretty=value=>String(value).replaceAll('_',' ')}){
  const entry=item=>{const species=rodentSpeciesForItem(item);return species?(state.snapshot?.rodentCatalog?.find(c=>c.id===species)||{id:species,name:names[species]}):null;};
  const itemName=item=>entry(item)?.name;
  const itemIcon=item=>icons[rodentSpeciesForItem(item)];
  function inventoryControls(item){const pet=entry(item);return pet?`<div class="notice good"><strong>${esc(pet.name)}${pet.rarity?' · '+esc(pretty(pet.rarity)):''}</strong><p>${esc(pet.description||'A small companion you have caught. Release it on your nut island to give it a home.')}</p></div>`:'';}
  function render(){
    const catalog=[...(state.snapshot?.rodentCatalog||[])].sort((a,b)=>(rarityOrder[a.rarity]??5)-(rarityOrder[b.rarity]??5));
    return `<p class="panel-intro">Little lives all around the island. Move close and press H to catch the nearest animal. A caught animal occupies a real inventory slot.</p><div class="recipe-grid">${catalog.map(pet=>`<article class="recipe-card"><h3>${icons[pet.id]||'🐾'} ${esc(pet.name)}</h3><p>${esc(pretty(pet.rarity))} · ${esc(pet.captureRadius||5)} m reach</p><p>${esc(pet.description||'')}</p></article>`).join('')}</div>`;
  }
  return {inventoryControls,itemName,itemIcon,render,isRodentItem:item=>!!entry(item),nearestCatchable:()=>nearestCatchable(state.snapshot),captureHint:()=>rodentCaptureHint(state.snapshot)};
}
