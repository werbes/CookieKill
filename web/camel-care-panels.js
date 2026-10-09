import {isCamelRaceIsland} from './camel-race-state.js';
export function createCamelCarePanels({state,button,escapeHTML}){
  const esc=value=>escapeHTML(String(value??''));
  const stats=[['speed','Speed','m/s'],['jump','Jump','m/s'],['stamina','Stamina',''],['happiness','Happiness','%']];
  function herd(){
    const me=state.snapshot?.me||{},camels=me.camels||[],treats=state.snapshot?.town?.treatCatalog||[];
    if(!camels.length)return '';
    return `<div class="section-label">CAMEL CARE <span>Each companion is one of a kind</span></div><p class="field-hint">Put a treat in your hotbar, hold it, and press E near your camel to feed it. Call a parked camel to make it run over. Speed is its natural pace; happiness brings it closer to its best. Treats double progress from matching exercises in the Camel Cove training gardens. Feeding alone never changes a stat.</p><div class="shop-list">${camels.map((c,i)=>{
      const activeTraining=me.island==='camel_training'&&state.snapshot?.camelTraining?.ownCamel?.id===c.id;
      if(activeTraining)c=state.snapshot.camelTraining.ownCamel;
      const sameWorld=!isCamelRaceIsland(me.island)&&me.island!=='camel_training'&&(c.island||'')===(me.island||'')&&!!c.inCave===!!me.inCave;
      const near=activeTraining||sameWorld&&Math.hypot(c.x-me.x,c.z-me.z)<=5&&Math.abs((c.y||0)-(me.y||0))<=2;
      const mounted=!!me.ridingCamel&&me.mountedCamelId===c.id;
      const location=mounted?'You are riding this camel.':c.following?'Running to you…':!sameWorld?(c.island?'Travel to this camel’s home island to call it.':'Return to this camel’s area on the main island to call it.'):c.homePenId?'This camel has a home in a fenced pen and returns there when you log out.':'Parked in this area.';
      return `<div class="notice"><strong>${esc(c.name||'Camel '+(i+1))}</strong><p>${stats.map(([key,name,unit])=>name+': '+Number(c[key]||0).toFixed(key==='speed'||key==='jump'?2:0)+unit).join(' · ')}</p><div class="item-actions">${button(c.following?'Coming to you…':'Call camel','camel_call',{target:c.id},!state.connected||!sameWorld||mounted||!!c.following,false)}${c.homePenId?button('Remove pen assignment','camel_unhouse',{target:c.id},!state.connected||!near):''}</div><p class="field-hint">${esc(location)}</p><div class="item-actions">${treats.map(t=>button(`${t.name} (${me.inventory?.[t.id]||0}) · 2x ${t.stat} training`,'feed_camel',{target:c.id,item:t.id},!state.connected||!near||!(me.inventory?.[t.id]>0)||(c[t.stat]||0)>=t.cap||(c.trainingBoost?.[t.stat]||0)+(t.work||120)>(t.maxWork||240))).join('')}</div><p class="field-hint">${near?'Treat boosts are saved for training: '+stats.map(([key,label])=>label+' '+Math.ceil(c.trainingBoost?.[key]||0)+' work').join(' / '):'Come within five metres of this camel to feed it.'}</p></div>`;
    }).join('')}</div>`;
  }
  return {herd};
}
