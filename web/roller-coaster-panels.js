const clock=seconds=>{const n=Math.max(0,Math.ceil(Number(seconds)||0));return Math.floor(n/60)+':'+String(n%60).padStart(2,'0');};
export function createRollerCoasterPanels({state,button,escapeHTML}){
  const esc=value=>escapeHTML(String(value??''));
  function panel(){
    const s=state.snapshot||{},me=s.me||{},view=s.rollerCoaster||{},ride=view.ride,station=view.station||{x:165,y:0,z:-161},mounted=!!me.ridingCamel;
    const disabled=!state.connected||!s.me||!!me.fallen;
    const intro='<div class="club-intro"><span>DESERT / DUNE DIPPER</span><h3>Take the scenic way around.</h3><p>A sweeping desert roller coaster with high climbs, drops, and banked turns. Ride on foot or bring the camel you are riding. Admission is free.</p></div>';
    if(ride){
      const progress=Math.max(0,Math.min(100,Math.round((ride.progress||0)*100))),companion=ride.camelId?`You and ${esc(ride.camelName||'your camel')} are riding together.`:'Your open-air car is following the rails.';
      return intro+`<div class="notice good"><strong>On the coaster · ${progress}% around</strong><p>${companion} ${clock(ride.remaining)} until the station.</p><p>The car drives itself. Look around and enjoy the view. You will return safely to the boarding point when the lap ends.</p><div class="item-actions">${button('Return to station now','coaster_leave',{},disabled,true)}</div></div><p class="field-hint">Your items and camel stay with you. Leaving early also returns you safely to the station. E, R, or Q opens this ride menu.</p>`;
    }
    const near=!me.island&&!me.inCave&&me.grounded!==false&&Math.hypot((me.x??Infinity)-station.x,(me.z??Infinity)-station.z)<=5&&Math.abs((me.y||0)-(station.y||0))<=.5;
    const wait=Math.max(0,view.nextBoardIn||0),camel=me.camels?.find(c=>c.id===me.mountedCamelId);
    return intro+`<div class="notice"><strong>${mounted?'A car with room for your camel':'The next car is yours'}</strong><p>${mounted?`${esc(camel?.name||'Your camel')} comes along and stays mounted after the ride.`:'Walk up to the station to board. To bring a camel, mount it first and ride it to the boarding sign.'}</p><p>One lap takes ${clock(view.duration||48)}. ${wait>0?'The next departure opens in '+clock(wait)+'.':'The boarding gate is ready.'}</p><div class="item-actions">${button(mounted?'Board with camel':'Board on foot','coaster_board',{target:'desert_coaster'},disabled||!near||wait>0,true)}</div></div><p class="field-hint">Find the station in the northern desert, northeast of the Sunbaked caves. Stay near the boarding sign and press E to ride.</p>`;
  }
  return {shop:kind=>kind==='roller_coaster'?panel():null,render:panel};
}

export function createRollerCoasterHUD({document,state,openModal}){
  const root=document.createElement('button');root.id='roller-coaster-hud';root.type='button';root.hidden=true;root.addEventListener('click',()=>openModal('coaster'));document.body.appendChild(root);
  const style=document.createElement('style');style.textContent='#roller-coaster-hud{position:fixed;z-index:8;left:50%;top:88px;transform:translateX(-50%);background:#345c59ed;color:#ffedc5;border:1px solid #e8b66899;border-radius:12px;padding:12px 22px;font:700 12px monospace;white-space:pre-line;text-align:center;line-height:1.6;cursor:pointer}#roller-coaster-hud.coaster-with-race-reminder{top:168px}#roller-coaster-hud[hidden]{display:none}@media(max-width:700px){#roller-coaster-hud{top:164px;max-width:92vw;font-size:10px;padding:9px 13px}#roller-coaster-hud.coaster-with-race-reminder{top:260px}}';document.head.appendChild(style);
  function update(){
    const s=state.snapshot,ride=s?.rollerCoaster?.ride;root.hidden=!state.connected||!state.hasPlayed||!!state.modal||!ride;if(root.hidden)return;
    const remaining=(s.camelRace?.reminderStartAt||0)-(s.time||0),raceNotice=remaining>0&&remaining<=120||(s.camelRace?.registeredStartAt||0)>(s.time||0);
    root.className=raceNotice?'coaster-with-race-reminder':'';
    root.textContent='DUNE DIPPER · '+Math.max(0,Math.min(100,Math.round((ride.progress||0)*100)))+'% · '+clock(ride.remaining)+' TO STATION\n'+(ride.camelId?'CAMEL ON BOARD · ':'')+'E / R / Q for ride menu / safe return';
  }
  return {root,update,dispose(){root.remove();style.remove();}};
}
