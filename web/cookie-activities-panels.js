const activeWorld=snapshot=>['cookie_ring','cookie_practice'].includes(snapshot?.me?.island);

export function createCookieActivityPanels({state,button,escapeHTML}){
  const esc=value=>escapeHTML(String(value??''));
  function entry(kind){
    const practice=kind==='tree_silver_cookie',me=state.snapshot?.me||{},x=practice?215:213,z=practice?181.5:177.5,near=!me.island&&!me.inCave&&!me.ridingCamel&&me.grounded!==false&&Math.hypot(me.x-x,me.z-z,(me.y||0)-12)<=3.2;
    return `<div class="section-label">THE COOKIE TREE <span>${practice?'Silver cookie':'Golden cookie'}</span></div><p class="panel-intro">${practice?'Visit a peaceful aim range and physical gym. Practice cookie throws, run on a treadmill, lift weights, use the leg press, and swim real pool laps.':'Enter the cookie fighting ring with other players. Use the cover, aim your throws, and keep moving.'}</p><div class="notice good"><p>Unlimited practice sugar cookies are supplied inside. Your own cookie inventory stays with you. Use the return cookie or R to return to this tree hut.</p>${button(practice?'Eat silver cookie · Practice & gym':'Eat golden cookie · Fighting ring',practice?'cookie_practice_enter':'cookie_ring_enter',{target:kind},!state.connected||!near,true)}</div>`;
  }
  function render(){
    const s=state.snapshot||{},me=s.me||{},arena=s.cookieArena||{},practice=me.island==='cookie_practice';
    if(!activeWorld(s))return `<div class="notice"><strong>Climb the Cookie Tree</strong><p>Find the slender tree at the southern edge of the city, near 210, 184. Stairs spiral around its outside to a hut with a golden cookie for fighting and a silver cookie for practice and gym exercises.</p></div>`;
    let content='';
    if(practice){
      const p=s.cookiePractice||{},exercise=p.exercise||{},station=p.stations?.find(v=>v.id===exercise.station),progress=Number(exercise.progress||0),goal=Number(exercise.goal||station?.goal||0);
      content=`<div class="section-label">AIM PRACTICE <span>${p.hits||0} target hits</span></div><p class="panel-intro">Aim toward the four target boards and left-click to throw unlimited sugar cookies. One target moves sideways. M opens the local map.</p><div class="section-label">PHYSICAL GYM <span>${p.fee||2} coins per completed level · Maximum level ${p.levelCap||10}</span></div><p class="field-hint">Walk to an apparatus and press E to use it. Exercise in the world to complete a set. W runs the treadmill; fresh Space presses lift or press weights. Q or E stops an apparatus. Swimming follows a full pool lane out and back.</p>${exercise.station?`<div class="notice good"><strong>${esc(station?.name||'Swimming practice')}</strong><p>${progress.toFixed(exercise.unit==='seconds'?1:0)} / ${goal} ${esc(exercise.unit||'work')} · ${esc(exercise.status||'Keep going')}${exercise.restIn>0?' · Rest '+Number(exercise.restIn).toFixed(1)+' s':''}</p>${button('Stop exercise','practice_stop',{},!state.connected)}</div>`:''}<div class="shop-list">${(p.stations||[]).map(st=>`<div class="notice"><strong>${esc(st.name)}</strong><p>${esc(st.instructions)}</p><p class="field-hint">${esc(st.stat)} level ${me.levels?.[st.stat]||0} / ${p.levelCap||10}${st.occupiedBy&&st.occupiedBy!==me.id?' · In use':''}</p></div>`).join('')}</div>`;
    }else content=`<div class="notice good"><strong>${arena.kills||0} ring wins · ${arena.deaths||0} times crumbled</strong><p>W A S D moves, Shift sprints, Space jumps, and left-click throws unlimited sugar cookies. Use the four cover blocks to break line of sight. M shows the arena map.${arena.shieldSeconds>0?' You have '+Number(arena.shieldSeconds).toFixed(1)+' seconds of spawn protection.':''}</p></div>`;
    return `<div class="section-label">${practice?'COOKIE PRACTICE & GYM':'COOKIE FIGHTING RING'}</div>${content}<div class="item-actions">${button('Return to the tree hut','cookie_arena_leave',{},!state.connected,true)}</div>`;
  }
  return {render,shop:kind=>['tree_golden_cookie','tree_silver_cookie'].includes(kind)?entry(kind):null};
}

export function createCookieActivityHUD({document,state,openModal}){
  if(!document.getElementById('cookie-activity-style')){const style=document.createElement('style');style.id='cookie-activity-style';style.textContent='#cookie-activity-hud{position:fixed;left:50%;bottom:142px;transform:translateX(-50%);z-index:15;max-width:min(560px,90vw);white-space:pre-line;text-align:center;border:1px solid #f0dbaa7a;border-radius:9px;padding:10px 16px;background:#3e5948ed;color:#fff0d2;font:700 11px/1.7 monospace;cursor:pointer}#cookie-activity-hud[hidden]{display:none}';document.head.appendChild(style);}
  const root=document.createElement('button');root.id='cookie-activity-hud';root.type='button';root.hidden=true;root.addEventListener('click',()=>openModal('cookie_activity'));document.body.appendChild(root);
  function update(){
    const s=state.snapshot;root.hidden=!state.connected||!state.hasPlayed||!state.locked||!!state.menuOpen||!!state.modal||!activeWorld(s);if(root.hidden)return;
    const p=s.cookiePractice,e=p?.exercise,a=s.cookieArena;let text;
    if(s.me.island==='cookie_practice'){
      if(e?.station){const station=p.stations?.find(st=>st.id===e.station);text=(station?.name||'SWIMMING').toUpperCase()+' · '+Number(e.progress||0).toFixed(e.unit==='seconds'?1:0)+' / '+e.goal+' '+(e.unit||'work')+'\n'+(e.status||'Keep exercising')+(e.restIn>0?' · REST '+Number(e.restIn).toFixed(1)+' s':'')+' · Q STOP · R DETAILS';}
      else text='AIM PRACTICE · '+(p?.hits||0)+' HITS · LEFT CLICK THROWS\nE USES GYM EQUIPMENT · SWIM A FULL POOL LAP · R RETURN MENU';
    }else text='COOKIE CLASH · '+(a?.kills||0)+' WINS · '+(a?.deaths||0)+' CRUMBLED\nUNLIMITED SUGAR COOKIES · R DETAILS & RETURN';
    if(root.textContent!==text)root.textContent=text;
  }
  return {root,update,dispose(){root.remove();}};
}
