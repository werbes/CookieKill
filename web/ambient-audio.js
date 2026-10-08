const clamp=(n,a,b)=>Math.max(a,Math.min(b,n));
const smooth=n=>{n=clamp(n,0,1);return n*n*(3-2*n);};
const preferenceKey='cookiekill-ambient-audio';
const channels=['ocean','forest','disco','cozy','desert'];
const hz=midi=>440*2**((midi-69)/12);

// Travel, rather than key presses, produces footsteps. Blocked movement, jumps,
// swimming and teleports stay silent; nearby players use the same world layer.
export function createFootstepTracker(){
  const tracks=new Map();
  function update(actors,listener,now){
    if(!listener||!Number.isFinite(now)){tracks.clear();return [];}
    const present=new Set(),sounds=[];
    for(const p of actors||[]){
      if(!p?.id||!Number.isFinite(p.x)||!Number.isFinite(p.z))continue;
      present.add(p.id);
      const old=tracks.get(p.id),world=(p.island||'')+'|'+!!p.inCave,dx=p.x-listener.x,dz=p.z-listener.z,distance=Math.hypot(dx,dz),local=p.id===listener.id;
      const swimming=p.swimming||(!p.island&&!p.inCave&&p.x<0&&p.z>32+11*Math.sin((p.x+240)*.021)+5*Math.sin(p.x*.057))||(p.island==='cookie_practice'&&(p.y||0)<-.1);
      const eligible=(p.island||'')===(listener.island||'')&&!!p.inCave===!!listener.inCave&&Math.abs((p.y||0)-(listener.y||0))<4&&distance<18&&p.grounded!==false&&!swimming&&!p.ridingCamel&&!p.fallen&&!p.posture&&p.health!==0;
      const next={x:p.x,y:p.y||0,z:p.z,world,at:now,distance:0,last:old?.last??-Infinity,side:old?.side||1,eligible};tracks.set(p.id,next);
      if(!eligible||!old?.eligible||old.world!==world)continue;
      const dt=now-old.at,moved=Math.hypot(p.x-old.x,p.z-old.z);
      if(dt<=0||dt>.65||moved>Math.max(3,dt*24)||Math.abs((p.y||0)-old.y)>1.5)continue;
      next.distance=Math.min(1.8,old.distance+moved);
      if(moved<.002||next.distance<1.15||now-old.last<(p.sprinting?.28:.42))continue;
      next.distance=0;next.last=now;next.side=-old.side;
      const pan=local?next.side*.035:(dx*Math.cos(listener.yaw||0)-dz*Math.sin(listener.yaw||0))/Math.max(1,distance);
      const surface=p.inCave?'stone':(p.y||0)>3?'wood':p.island==='cookie_ring'||p.island==='cookie_practice'||(!p.island&&p.x>=0&&p.z>=0)?'stone':!p.island&&(p.x>=0||p.z>10)?'sand':'grass';
      sounds.push({id:p.id,local,distance,pan:clamp(pan,-1,1),level:local?.65:(1-distance/18)**2,surface,at:now});
    }
    for(const id of tracks.keys())if(!present.has(id))tracks.delete(id);
    return sounds.sort((a,b)=>a.distance-b.distance).slice(0,8);
  }
  return {update,clear(){tracks.clear();}};
}

// The same coastline as the authoritative map; venue footprints come from its layout.
export function ambientMix(snapshot,listener=snapshot?.me,layout=snapshot?.layout){
  const mix={ocean:0,forest:0,disco:0,cozy:0,desert:0};
  if(!listener||listener.island||!Number.isFinite(listener.x)||!Number.isFinite(listener.z))return mix;
  const {x,z}=listener;
  if(listener.inCave){
    const club=(z<=-204&&z>=-227&&Math.abs(x+202)<=4.4)||(x>=-218&&x<=-186&&z>=-250&&z<=-226);
    if(club)mix.disco=smooth((-204-z)/15);
    return mix;
  }
  const west=1-smooth((x+3)/6),north=1-smooth((z+3)/6),shore=32+11*Math.sin((x+240)*.021)+5*Math.sin(x*.057);
  mix.ocean=west*smooth((z-shore+28)/21);
  mix.forest=west*(1-smooth((z-shore+29)/18));
  mix.desert=(1-west)*north;
  if(x>=0&&z>=0){
    for(const prop of layout?.props||[]){
      if(prop.kind!=='bakery'&&!prop.kind?.includes('shop'))continue;
      if(prop.x<0||prop.z<0)continue;
      const inset=Math.min((prop.width||0)/2-Math.abs(x-prop.x),(prop.depth||0)/2-Math.abs(z-prop.z));
      mix.cozy=Math.max(mix.cozy,smooth((inset+8)/9));
    }
  }
  return mix;
}

// Original small arrangements, synthesized locally. The controller owns all of
// its voices; it never changes the separately controlled John Pork ringing.
export function createAmbientAudio({AudioContext=globalThis.AudioContext||globalThis.webkitAudioContext,storage,onEvent}={}){
  if(storage===undefined){try{storage=globalThis.localStorage;}catch{}}
  let muted=false,volume=.55;
  try{const saved=JSON.parse(storage?.getItem(preferenceKey)||'null');if(saved){muted=saved.muted===true;if(Number.isFinite(saved.volume))volume=clamp(saved.volume,0,1);}}catch{}
  let context,master,compressor,noiseBuffer,disposed=false,enabled=false,stopped=true,seed=0x71c03f,lastDolphin=-Infinity;
  const buses={},targets=Object.fromEntries(channels.map(id=>[id,0])),voices=new Set(),dolphins=new Map(),next={},steps={};
  let surf=null;const footsteps=createFootstepTracker();
  const random=()=>{seed=(Math.imul(seed,1664525)+1013904223)>>>0;return seed/4294967296;};
  const emit=(type,data={})=>onEvent?.({type,...data});
  const persist=()=>{try{storage?.setItem(preferenceKey,JSON.stringify({muted,volume}));}catch{}};
  function track(source,nodes,end,kind){
    const voice={source,nodes,end,kind};voices.add(voice);
    const cleanup=()=>{if(!voices.delete(voice))return;try{source.disconnect();}catch{}for(const n of nodes)try{n.disconnect();}catch{}};
    source.onended=cleanup;return voice;
  }
  function stopVoice(v){try{v.source.stop();}catch{}try{v.source.disconnect();}catch{}for(const n of v.nodes)try{n.disconnect();}catch{}voices.delete(v);}
  function hardStop(){
    if(master&&context){master.gain.cancelScheduledValues(context.currentTime);master.gain.setValueAtTime(0,context.currentTime);}
    for(const v of [...voices])stopVoice(v);surf=null;
    for(const id of channels){targets[id]=0;if(buses[id]&&context){buses[id].gain.cancelScheduledValues(context.currentTime);buses[id].gain.setValueAtTime(0,context.currentTime);}next[id]=0;steps[id]=0;}
    dolphins.clear();footsteps.clear();lastDolphin=-Infinity;stopped=true;
  }
  function unlock(){
    if(disposed||!AudioContext)return Promise.resolve(false);
    try{
      if(!context){
        context=new AudioContext();master=context.createGain();master.gain.value=0;compressor=context.createDynamicsCompressor();compressor.threshold.value=-12;compressor.knee.value=14;compressor.ratio.value=3;compressor.attack.value=.01;compressor.release.value=.25;master.connect(compressor);compressor.connect(context.destination);
        for(const id of channels){buses[id]=context.createGain();buses[id].gain.value=0;buses[id].connect(master);}
        buses.dolphin=context.createGain();buses.dolphin.gain.value=1;buses.dolphin.connect(master);
        buses.footsteps=context.createGain();buses.footsteps.gain.value=1;buses.footsteps.connect(master);
        // A repeatable noise bed becomes surf, breath and soft percussion.
        noiseBuffer=context.createBuffer(1,context.sampleRate*4,context.sampleRate);const data=noiseBuffer.getChannelData(0);for(let i=0;i<data.length;i++)data[i]=random()*2-1;
      }
      return Promise.resolve(context.state==='suspended'?context.resume():undefined).then(()=>context?.state==='running').catch(()=>false);
    }catch{return Promise.resolve(false);}
  }
  function envelope(gain,at,duration,level,attack=.008){gain.setValueAtTime(.0001,at);gain.exponentialRampToValueAtTime(Math.max(.0002,level),at+Math.min(attack,duration*.25));gain.exponentialRampToValueAtTime(.0001,at+duration);}
  function panRoute(node,bus,pan,nodes){if(context.createStereoPanner){const p=context.createStereoPanner();p.pan.value=clamp(pan,-1,1);node.connect(p);p.connect(bus);nodes.push(p);}else node.connect(bus);}
  function tone(channel,at,frequency,duration,level,type='sine',pan=0,to=frequency,attack=.008,kind='note'){
    if(voices.size>=96)return;
    const osc=context.createOscillator(),gain=context.createGain(),nodes=[gain];osc.type=type;osc.frequency.setValueAtTime(Math.max(30,frequency),at);if(to!==frequency)osc.frequency.exponentialRampToValueAtTime(Math.max(30,to),at+duration*.85);envelope(gain.gain,at,duration,level,attack);osc.connect(gain);panRoute(gain,buses[channel],pan,nodes);track(osc,nodes,at+duration+.02,kind);osc.start(at);osc.stop(at+duration+.02);
  }
  function noise(channel,at,duration,level,cutoff,pan=0,kind='noise',highpass=false){
    if(voices.size>=96)return;
    const source=context.createBufferSource(),filter=context.createBiquadFilter(),gain=context.createGain(),nodes=[filter,gain];source.buffer=noiseBuffer;filter.type=highpass?'highpass':'lowpass';filter.frequency.value=cutoff;filter.Q.value=.5;envelope(gain.gain,at,duration,level,.025);source.connect(filter);filter.connect(gain);panRoute(gain,buses[channel],pan,nodes);track(source,nodes,at+duration+.02,kind);source.start(at,random()*2);source.stop(at+duration+.02);
  }
  function ocean(now){
    if(surf)return;
    const source=context.createBufferSource(),filter=context.createBiquadFilter(),gain=context.createGain(),lfo=context.createOscillator(),depth=context.createGain();source.buffer=noiseBuffer;source.loop=true;filter.type='lowpass';filter.frequency.value=1050;filter.Q.value=.55;gain.gain.value=.19;lfo.frequency.value=.105;depth.gain.value=.105;source.connect(filter);filter.connect(gain);gain.connect(buses.ocean);lfo.connect(depth);depth.connect(gain.gain);track(source,[filter,gain],Infinity,'surf');track(lfo,[depth],Infinity,'surf-motion');source.start(now);lfo.start(now);surf={source,lfo};emit('surf',{at:now});
  }
  function piano(channel,at,note,duration,level,pan=0){tone(channel,at,hz(note),duration,level,'sine',pan);tone(channel,at,hz(note)*2,duration*.5,level*.2,'sine',pan);}
  function pluck(at,note){for(const [partial,level,decay]of [[1,.038,.62],[2,.026,.27],[3,.015,.18],[5,.006,.1]])tone('desert',at,hz(note)*partial,decay,level,'sine',.3);noise('desert',at,.035,.025,2200,.3,'banjo-pick',true);}
  function disco(at,step){
    const bar=Math.floor(step/16)%4,n=step%16,bass=[40,36,43,38][bar];
    if(n%4===0)tone('disco',at,130,.18,.3,'sine',0,42,.002,'kick');
    if(n%8===4)noise('disco',at,.13,.13,1250,0,'clap',true);
    noise('disco',at,n%2?.11:.045,n%2?.038:.025,6500,.25,'hat',true);
    if([0,3,6,8,11,14].includes(n))tone('disco',at,hz(bass+(n===6||n===14?12:0)),.18,.12,'triangle',-.08);
    if(n===2||n===10){for(const interval of [12,15,19,22])piano('disco',at,bass+interval,.24,.027,interval%2?.3:-.3);}
    if(n%4===3)tone('disco',at,hz([76,79,74,71][(bar+n)%4]),.12,.027,'triangle',-.25);
  }
  function cozy(at,step){
    const bar=Math.floor(step/16)%4,n=step%16,root=[48,45,50,43][bar],chord=[[0,4,7,11],[0,3,7,10],[0,3,7,10],[0,4,7,9]][bar];
    if(n%8===0){for(const interval of chord)piano('cozy',at,root+12+interval,1.65,.025,interval%2?-.25:.2);tone('cozy',at,hz(root),.6,.06,'sine',0);}
    const tune=[76,null,79,77,76,72,null,74,76,null,71,72,74,76,null,72];if(tune[n]!=null)piano('cozy',at,tune[n]+[0,-3,2,-5][bar],.72,.045,.15);
    if(n%4===2)noise('cozy',at,.08,.013,4500,-.35,'brush',true);
  }
  function desert(at,step){
    const n=step%16,transpose=Math.floor(step/32)%2?5:0,pattern=[50,57,62,65,57,62,69,65];pluck(at,pattern[n%8]+transpose);
    if(n%4===0){const note=[74,77,76,69][Math.floor(n/4)]+transpose,f=hz(note);tone('desert',at,f,1.12,.066,'sine',-.25,f*1.003,.11,'flute');tone('desert',at,f*2,.8,.009,'sine',-.25,f*2.002,.09,'flute');noise('desert',at,.82,.022,1600,-.25,'flute-breath');}
    if(n===0)tone('desert',at,hz(38+transpose),3.5,.025,'sine');
  }
  function birds(at){
    const pan=random()*1.6-.8,f=1700+random()*1300;
    for(let i=0;i<3;i++){const start=at+i*.19;tone('forest',start,f*(i%2?1.16:1),.12,.046,'sine',pan,f*(i%2?.95:1.36),.014,'bird');}
    emit('birds',{at});
  }
  function dolphinJumps(animals,listener,now){
    const present=new Set();
    for(const animal of animals||[]){
      if(animal.species!=='dolphin'||animal.egg||animal.health<=0)continue;
      present.add(animal.id);const high=(animal.y||0)>.16,old=dolphins.get(animal.id);
      if(old!==undefined&&!old&&high&&!listener.island&&!listener.inCave&&now-lastDolphin>.28){
        const dx=animal.x-listener.x,dz=animal.z-listener.z,distance=Math.hypot(dx,dz);
        if(distance<75){const pan=(dx*Math.cos(listener.yaw||0)-dz*Math.sin(listener.yaw||0))/Math.max(1,distance),level=.13*(1-distance/75)**1.3;for(let i=0;i<3;i++)tone('dolphin',now+.015+i*.085,1400+i*450,.14,level/(i+1),'sine',pan,2700+i*300,.012,'dolphin');lastDolphin=now;emit('dolphin',{id:animal.id,at:now,distance});}
      }
      // Hysteresis means descending through the threshold cannot chirp again.
      dolphins.set(animal.id,!!(high||old&&(animal.y||0)>.08));
    }
    for(const id of dolphins.keys())if(!present.has(id))dolphins.delete(id);
  }
  function update(snapshot,{listener,layout,animals,players,enabled:allow=true,hidden=globalThis.document?.hidden||false}={}){
    enabled=!!allow&&!hidden&&!!snapshot?.me;
    if(disposed)return;
    if(!enabled||muted||volume===0){if(!stopped)hardStop();return;}
    if(!context||context.state!=='running')return;
    const now=context.currentTime,me={...snapshot.me,...listener};
    // Reap expired voices even when a browser delays onended while backgrounded.
    for(const voice of [...voices])if(voice.end<=now)stopVoice(voice);
    if(stopped){master.gain.setTargetAtTime(volume*.6,now,.12);stopped=false;}
    const mix=ambientMix(snapshot,me,layout||snapshot.layout);
    for(const id of channels){if(Math.abs(mix[id]-targets[id])>.005){targets[id]=mix[id];buses[id].gain.setTargetAtTime(mix[id],now,.45);}if(mix[id]>.005&&id==='ocean')ocean(now);}
    if(surf&&mix.ocean<.005&&buses.ocean.gain.value<.002){for(const voice of [...voices])if(voice.kind==='surf'||voice.kind==='surf-motion')stopVoice(voice);surf=null;}
    // Less than 180ms of lookahead; never catch up an old backlog after a pause.
    for(const [id,interval,play]of [['disco',60/116/4,disco],['cozy',60/78/2,cozy],['desert',60/84/2,desert],['forest',2.8,birds]]){
      if(mix[id]<.005){next[id]=0;continue;}
      if(!next[id]||next[id]<now-.2)next[id]=now+.035;
      let count=0;while(next[id]<now+.18&&count++<3){const at=next[id],step=steps[id]||0;play(at,step);emit('step',{channel:id,at,step});steps[id]=step+1;next[id]+=interval*(id==='forest'?.8+random()*.8:1);}
    }
    dolphinJumps(animals||snapshot.animals,me,now);
    const feet={...me,y:snapshot.me.y||0};
    for(const step of footsteps.update([feet,...(players||snapshot.players||[])],feet,now)){
      const cutoff={stone:800,wood:500,sand:1500,grass:2200}[step.surface];
      noise('footsteps',now+.006,.09,.045*step.level,cutoff,step.pan,'footstep');
      tone('footsteps',now+.006,step.surface==='wood'?150:85,.065,.03*step.level,'sine',step.pan,55,.004,'footstep');
      emit('footstep',step);
    }
  }
  return {unlock,update,clear(){enabled=false;hardStop();},setMuted(value){muted=!!value;persist();if(muted)hardStop();},setVolume(value){if(!Number.isFinite(Number(value)))return;volume=clamp(Number(value),0,1);persist();if(!volume)hardStop();else if(master&&context&&!muted&&!stopped)master.gain.setTargetAtTime(volume*.6,context.currentTime,.08);},dispose(){if(disposed)return;disposed=true;hardStop();for(const bus of Object.values(buses))bus.disconnect();master?.disconnect();compressor?.disconnect();context?.close()?.catch?.(()=>{});context=null;},get muted(){return muted;},get volume(){return volume;},get playing(){return !disposed&&!stopped&&!muted&&enabled&&context?.state==='running';},getState(){return {unlocked:!!context,playing:!stopped,voices:voices.size,targets:{...targets},muted,volume};}};
}
