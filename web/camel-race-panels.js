import {isCamelRaceIsland,camelRaceReminder} from './camel-race-state.js';

export const raceClock=seconds=>{const n=Math.max(0,Math.ceil(Number(seconds)||0));return Math.floor(n/60)+':'+String(n%60).padStart(2,'0');};

export function createCamelRacePanels({state,button,escapeHTML}){
  const esc=value=>escapeHTML(String(value??''));
  const inRace=(me,race)=>isCamelRaceIsland(me.island)||!!race.inArena;
  const teleportButton=()=>button('Teleport menu','tab',{tab:'teleport'});
  function casualMenu(){
    const s=state.snapshot||{},me=s.me||{},race=s.camelRace||{},active=inRace(me,race),disabled=!state.connected||!s.me||!!me.fallen||active;
    if(active)return `<div class="notice good"><strong>You are in a camel race</strong><p>Open your current race to see the standings or return to your previous location.</p>${button('Current race / return','tab',{tab:'race'})}</div>`;
    const lobbies=(race.casualLobbies||[]).filter(lobby=>lobby.phase==='lobby');
    return `<div class="section-label">PLAYFUL CAMEL RACES <span>Free to host or join</span></div><p class="panel-intro">Meet friends on the track from wherever you are. The host starts a ten-second countdown when everyone is ready. Ride your own camel or borrow one, then return to where you came from. Playful races are for fun; coin prizes and camel coins are earned in scheduled races.</p><div class="item-actions">${button('Host a playful race','race_casual_host',{},disabled,true)}</div>${lobbies.length?`<div class="shop-list">${lobbies.map(lobby=>`<div class="shop-row"><div class="shop-copy"><h3>${esc(lobby.hostName||'A fellow rider')}'s race</h3><p>${esc(lobby.entrantCount||0)} racers waiting</p></div><div class="item-actions">${button('Join playful race','race_casual_join',{target:lobby.id},disabled)}</div></div>`).join('')}</div>`:'<p class="field-hint">No open playful races yet. Host one and invite your friends to join from their Teleport menu.</p>'}`;
  }
  function standings(race,me,casual){
    if(!race.entrants?.length)return '';
    return `<div class="section-label">THE RACE</div><div class="shop-list">${[...race.entrants].sort((a,b)=>(a.rank||999)-(b.rank||999)).map(e=>{
      const detail=e.status==='finished'?(casual?`Place ${e.rank} · ${raceClock(e.finishTime)}`:`${e.coins||0} coins + ${e.camelCoins||0} camel coins`):e.status==='dnf'?'Withdrawn':race.phase==='lobby'?'Waiting for the host':`Lap ${Math.min(3,(e.lap||0)+1)} / 3`;
      return `<div class="shop-row"><span class="shop-icon">${esc(e.rank||'♞')}</span><div class="shop-copy"><h3>${esc(e.name)}${e.id===me.id?' · you':''}</h3><p>${esc(detail)}</p></div></div>`;
    }).join('')}</div>`;
  }
  function panel(){
    const s=state.snapshot||{},race=s.camelRace||{},me=s.me||{},self=race.entrants?.find(e=>e.id===me.id),inArena=inRace(me,race),casual=inArena&&race.mode==='casual',registered=(race.registeredStartAt||0)>s.time;
    const start=registered?race.registeredStartAt:race.nextStartAt,participants=race.entrantCount||0,others=Math.max(0,participants-1),disabled=!state.connected||!s.me||!!me.fallen;
    const riding=`<p class="panel-intro">Use W A S D and the mouse to ride, Shift to sprint, and Space to jump. Follow the arrows and golden checkpoint strip around the ring. Complete every checkpoint in order for three laps. Keep an eye on stamina.</p>`;
    if(casual){
      const host=race.hostId===me.id,status=race.phase==='lobby'?(host?'Your lobby is open. Start when everyone is ready.':'Waiting for the host to start.'):race.phase==='gathering'?'On the starting grid · '+raceClock(race.startAt-s.time)+' to go':self?.status==='finished'?'Finished in place '+self.rank:race.phase==='racing'?'Playful race in progress':'Playful race complete';
      const startButton=host&&race.phase==='lobby'?button('Start playful race','race_casual_start',{},disabled,true):'';
      return `<div class="club-intro"><span>CAMEL COVE / PLAYFUL RACE</span><h3>A few laps with friends.</h3><p>Three laps on the same sandy oval. Hosting and joining are free. The host starts a ten-second countdown once everyone is ready. Playful races are for fun; coin prizes and camel coins are earned in scheduled races.</p></div><div class="notice good"><strong>${esc(status)}</strong><p>Your mounted or owned camel joins you; a loan camel is provided if needed. Every camel keeps its own speed, jump, stamina, and happiness.</p><div class="item-actions">${startButton}${button(self?.status==='racing'&&race.phase==='racing'?'Leave race · withdraw':'Return to previous location','race_leave',{},disabled)}${teleportButton()}</div></div>${riding}<p class="field-hint">${esc(participants)} racer${participants===1?'':'s'} in this playful race. Your camel and previous location are restored when you leave.</p>${standings(race,me,true)}`;
    }
    const status=inArena?(race.phase==='gathering'?'On the starting grid · '+raceClock(race.startAt-s.time)+' to go':self?.status==='finished'?'Finished in place '+self.rank:race.phase==='racing'?'Race in progress':'Race complete'):registered?'You are entered. Gather in '+raceClock(start-20-s.time):'Next race in '+raceClock(start-s.time);
    const controls=inArena?button(self?.status==='racing'?'Leave race · withdraw':'Return to previous location','race_leave',{},disabled):registered?button('Cancel race entry','race_leave',{},disabled):button('Eat camel lollipop & enter','race_enter',{target:'camel_race_lollipop'},disabled||!!me.island||!!me.inCave||Math.hypot(me.x+227,me.z-20)>4,true);
    return `<div class="club-intro"><span>WEST BEACH / DUNE DASH</span><h3>A little desert thunder.</h3><p>Three laps around a sandy oval. A race starts every 18 minutes, two minutes after sunrise. Eat the camel-shaped lollipop to enter the next available race. Twenty seconds before the start, entered players gather on the grid.</p></div><div class="notice good"><strong>${esc(status)}</strong><p>Your mounted camel joins you, or we choose an owned camel. A loan camel is provided if you need one. Every camel has its own speed, jump, stamina, and happiness.</p><div class="item-actions">${controls}${teleportButton()}</div></div>${riding}<div class="stats-grid">${[[200,50,'1st'],[100,30,'2nd'],[50,10,'3rd'],[0,0,'4th+']].map(([coins,camelCoins,label])=>`<div class="stat-card"><strong>${label}</strong><span>${coins} × other racers in coins<br>${camelCoins} camel coins${inArena?'<br>'+coins*others+' coins this race':''}</span></div>`).join('')}</div><p class="field-hint">${participants} racer${participants===1?'':'s'} in the current ring. Prizes use the number who gathered, excluding yourself. Camel coins are a separate leaderboard score. Leaving before finishing forfeits your place. Your camel and previous location are restored when you return.</p>${standings(race,me,false)}`;
  }
  return {shop:kind=>kind==='camel_race_lollipop'?panel():null,render:panel,casualMenu};
}

export function createCamelRaceHUD({document,state,openModal}){
  const root=document.createElement('button');root.id='camel-race-hud';root.type='button';root.hidden=true;root.addEventListener('click',()=>openModal('race'));document.body.appendChild(root);
  const style=document.createElement('style');style.textContent='#camel-race-hud{position:fixed;z-index:8;top:88px;left:50%;transform:translateX(-50%);border:1px solid #e4c38380;border-radius:12px;padding:12px 22px;background:#29463eef;color:#ffedc7;font:700 12px monospace;cursor:pointer;white-space:pre-line;text-align:center;line-height:1.6;box-shadow:0 5px 18px #13231c33}#camel-race-hud[hidden]{display:none}@media(max-width:700px){#camel-race-hud{top:164px;max-width:92vw;font-size:10px;padding:9px 13px}}';document.head.appendChild(style);
  function update(){
    const s=state.snapshot,me=s?.me,r=s?.camelRace,reminder=camelRaceReminder(s),active=!!r?.inArena||isCamelRaceIsland(me?.island),registered=(r?.registeredStartAt||0)>(s?.time||0);
    root.hidden=!me||!state.connected||!!state.modal||!state.hasPlayed||!r||!(active||registered||reminder);if(root.hidden)return;
    const own=r.entrants?.find(e=>e.id===me.id),casual=r.mode==='casual';let text;
    if(active){
      if(casual&&r.phase==='lobby')text=r.hostId===me.id?'PLAYFUL RACE · YOUR LOBBY IS OPEN\nE to start / race menu':'PLAYFUL RACE · WAITING FOR HOST\nE or R for racers / return';
      else if(r.phase==='gathering')text=(casual?'PLAYFUL RACE · ':'')+'START IN '+raceClock(r.startAt-s.time)+' · HOLD YOUR CAMEL\nE or R for standings / return';
      else if(own?.status==='finished')text=casual?'PLACE '+own.rank+' · TIME '+raceClock(own.finishTime)+'\nE or R for standings / return':'PLACE '+own.rank+' · +'+(own.coins||0)+' COINS · +'+(own.camelCoins||0)+' CAMEL COINS\nE or R for standings / return';
      else if(own?.status==='dnf'||r.phase==='finished')text='RACE ENDED · CLICK TO RETURN';
      else text=(casual?'PLAYFUL RACE':'DUNE DASH')+' · LAP '+Math.min(3,(own?.lap||0)+1)+'/3 · '+(r.entrantCount||0)+' RACERS\nE or R for standings / return';
      if(casual&&reminder)text+='\nScheduled Dune Dash in '+raceClock(reminder);
    }else if(registered)text='RACE ENTRY SAVED · GATHER IN '+raceClock(r.registeredStartAt-20-s.time)+'\nClick for race details';
    else text='DUNE DASH STARTS IN '+raceClock(reminder)+'\n'+(reminder<=20?'Racers are on the starting grid':'Visit the Camel Cove lollipop to enter')+' · Click for race details';
    if(root.textContent!==text)root.textContent=text;
  }
  return {root,update,dispose(){root.remove();style.remove();}};
}
