const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const modulePromise=import('data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(__dirname,'../web/ambient-audio.js'),'utf8')).toString('base64'));
function fixture(){
  const contexts=[],events=[],saved=new Map(),storage={getItem:key=>saved.get(key),setItem:(key,value)=>saved.set(key,value)};
  const param=()=>({value:0,calls:[],setValueAtTime(value,time){this.value=value;this.calls.push(['set',value,time]);},setTargetAtTime(value,time,constant){this.value=value;this.calls.push(['target',value,time,constant]);},exponentialRampToValueAtTime(value,time){this.calls.push(['ramp',value,time]);},cancelScheduledValues(time){this.calls.push(['cancel',time]);}});
  class AudioContext{
    constructor(){this.currentTime=0;this.sampleRate=8000;this.state='suspended';this.nodes=[];this.destination={};contexts.push(this);}
    node(){const n={gain:param(),frequency:param(),Q:param(),pan:param(),threshold:param(),knee:param(),ratio:param(),attack:param(),release:param(),connections:[],connect(dest){this.connections.push(dest);return dest;},disconnect(){this.disconnected=true;},start(at){this.at=at;this.started=true;},stop(at){this.end=at;if(at===undefined||at<=this.context.currentTime){this.stopped=true;this.onended?.();}},context:this};this.nodes.push(n);return n;}
    resume(){this.state='running';return Promise.resolve();}close(){this.state='closed';return Promise.resolve();}
    createGain(){return this.node();}createDynamicsCompressor(){return this.node();}createOscillator(){const n=this.node();n.oscillator=true;return n;}createBufferSource(){return this.node();}createBiquadFilter(){return this.node();}createStereoPanner(){return this.node();}
    createBuffer(_channels,length){const data=new Float32Array(length);return {getChannelData:()=>data};}
  }
  return {AudioContext,contexts,events,storage,saved,onEvent:event=>events.push(event)};
}
const snap=(x,z,extra={})=>({time:0,me:{id:'me',x,z,y:0,island:'',inCave:false,yaw:0,...extra},animals:[]});

test('region and venue mix follows coastline and authoritative footprints, respecting world layers',async()=>{
  const {ambientMix}=await modulePromise;
  assert.equal(ambientMix(snap(-80,-120)).forest,1);assert.equal(ambientMix(snap(-80,80)).ocean,1);assert.equal(ambientMix(snap(80,-120)).desert,1);
  const city=snap(80,22);city.layout={props:[{kind:'kitchen_shop',x:80,z:22,width:12,depth:9}]};assert.equal(ambientMix(city).cozy,1);assert.equal(ambientMix(city,{...city.me,x:96}).cozy,0);assert.ok(ambientMix(city,{...city.me,x:90}).cozy>0&&ambientMix(city,{...city.me,x:90}).cozy<1);
  const club=snap(-202,-238,{inCave:true});assert.equal(ambientMix(club).disco,1);assert.equal(ambientMix(snap(-202,-205,{inCave:true})).disco>0,true);assert.deepEqual(ambientMix(snap(120,-124,{inCave:true})),{ocean:0,forest:0,disco:0,cozy:0,desert:0});
  for(const island of ['private','camel_race'])assert.ok(Object.values(ambientMix(snap(-80,80,{island}))).every(n=>n===0));
  assert.ok(Object.values(ambientMix(snap(NaN,80))).every(n=>n===0));
});

test('no context or sound is created before user unlock, then each venue schedules its own arrangement',async()=>{
  const {createAmbientAudio}=await modulePromise,f=fixture(),audio=createAmbientAudio(f);
  audio.update(snap(-80,80));assert.equal(f.contexts.length,0);assert.equal(audio.playing,false);await audio.unlock();assert.equal(f.contexts.length,1);audio.update(snap(-80,80));assert.equal(audio.playing,true);assert.equal(f.events.filter(e=>e.type==='surf').length,1);
  audio.update(snap(-80,80));assert.equal(f.events.filter(e=>e.type==='surf').length,1,'one continuous surf bed, not one per frame');
  for(const [x,z,extra,channel]of [[-80,-120,{},'forest'],[-202,-238,{inCave:true},'disco'],[80,-120,{},'desert']]){f.contexts[0].currentTime+=1;audio.update(snap(x,z,extra));assert.ok(f.events.some(e=>e.type==='step'&&e.channel===channel));}
  const city=snap(80,22);audio.update(city,{layout:{props:[{kind:'bakery',x:80,z:22,width:20,depth:16}]}});assert.ok(f.events.some(e=>e.channel==='cozy'));
  const ramps=f.contexts[0].nodes.flatMap(n=>n.gain.calls).filter(c=>c[0]==='target'&&c[3]===.45);assert.ok(ramps.some(c=>c[1]===0)&&ramps.some(c=>c[1]===1),'region changes must ramp old and new buses');audio.dispose();
});

test('dolphin calls occur once on actual nearby upward jumps, using rendered overrides and stereo panning',async()=>{
  const {createAmbientAudio}=await modulePromise,f=fixture(),audio=createAmbientAudio(f);await audio.unlock();const context=f.contexts[0],s=snap(-80,80),d={id:'d1',species:'dolphin',health:10,x:-68,z:80,y:.02};
  s.animals=[{...d,y:1}];audio.update(s,{animals:[d]});assert.equal(f.events.some(e=>e.type==='dolphin'),false,'joining mid-jump must not invent a jump sound');
  context.currentTime=.2;audio.update(s,{animals:[{...d,y:.4}]});assert.equal(f.events.filter(e=>e.type==='dolphin').length,1);assert.ok(context.nodes.some(n=>n.pan.value===1),'a jump to the listener right should pan right');
  for(const y of [.8,1.5,.3,.12,.2]){context.currentTime+=.2;audio.update(s,{animals:[{...d,y}]});}assert.equal(f.events.filter(e=>e.type==='dolphin').length,1);
  context.currentTime+=.5;audio.update(s,{animals:[d]});context.currentTime+=.2;audio.update(s,{animals:[{...d,y:.3}]});assert.equal(f.events.filter(e=>e.type==='dolphin').length,2);
  audio.clear();context.currentTime++;audio.update(s,{animals:[{...d,x:100,y:.02}]});context.currentTime++;audio.update(s,{animals:[{...d,x:100,y:.5}]});assert.equal(f.events.filter(e=>e.type==='dolphin').length,2,'distant dolphins stay inaudible');audio.dispose();
});

test('disconnect, hidden page, mute, zero volume and disposal silence all sources immediately',async()=>{
  const {createAmbientAudio}=await modulePromise;
  for(const stop of [(a,s)=>a.update(s,{enabled:false}),(a,s)=>a.update(s,{hidden:true}),a=>a.setMuted(true),a=>a.setVolume(0),a=>a.clear(),a=>a.dispose()]){
    const f=fixture(),a=createAmbientAudio(f),s=snap(-80,80);await a.unlock();a.update(s);assert.ok(a.getState().voices>0);stop(a,s);assert.equal(a.getState().voices,0);assert.equal(a.playing,false);assert.equal(f.contexts[0].nodes[0].gain.value,0);a.dispose();assert.equal(f.contexts[0].state,'closed');a.update(s);assert.equal(a.getState().voices,0);
  }
});

test('scheduling is bounded during frame stalls, repeated updates, and long play',async()=>{
  const {createAmbientAudio}=await modulePromise,f=fixture(),audio=createAmbientAudio(f);await audio.unlock();const context=f.contexts[0],s=snap(-202,-238,{inCave:true});
  audio.update(s);const count=f.events.length;for(let i=0;i<1000;i++)audio.update(s);assert.equal(f.events.length,count,'duplicate frames do not schedule duplicate beats');
  context.currentTime=3600;const before=f.events.length;audio.update(s);assert.ok(f.events.length-before<=3,'resuming skips missed music instead of replaying an hour');
  for(let i=0;i<2000;i++){context.currentTime+=.05;audio.update(i%400<200?s:snap(80,-120));assert.ok(audio.getState().voices<=96);}
  assert.ok(audio.getState().voices>0);audio.dispose();
});

test('mute/volume preferences persist, clamp, recover from bad storage, and resume cleanly',async()=>{
  const {createAmbientAudio}=await modulePromise,f=fixture(),a=createAmbientAudio(f);a.setVolume(.23);a.setMuted(true);const b=createAmbientAudio(f);assert.equal(b.volume,.23);assert.equal(b.muted,true);await b.unlock();b.update(snap(80,-120));assert.equal(b.playing,false);b.setMuted(false);b.update(snap(80,-120));assert.equal(b.playing,true);b.setVolume(5);assert.equal(b.volume,1);b.setVolume(NaN);assert.equal(b.volume,1);b.setVolume(-2);assert.equal(b.volume,0);
  const broken=createAmbientAudio({storage:{getItem(){throw Error('private storage');},setItem(){throw Error('private storage');}},AudioContext:null});assert.equal(broken.volume,.55);assert.doesNotThrow(()=>broken.setMuted(true));assert.equal(await broken.unlock(),false);a.dispose();b.dispose();broken.dispose();
});

test('quiet footsteps follow real local and nearby travel, with distance and stereo direction',async()=>{
  const {createAmbientAudio}=await modulePromise,f=fixture(),audio=createAmbientAudio(f);await audio.unlock();
  const s=snap(100,180,{grounded:true});s.players=[{id:'friend',x:104,z:180,y:0,grounded:true}];audio.update(s);
  for(let i=1;i<=12;i++){f.contexts[0].currentTime=i*.1;s.me.z-=.3;s.players[0].z-=.3;audio.update(s);}
  const steps=f.events.filter(e=>e.type==='footstep');assert.ok(steps.some(e=>e.local));assert.ok(steps.some(e=>e.id==='friend'&&e.pan>0&&e.distance===4));assert.ok(steps.every(e=>e.level<=.65));
  assert.ok(f.contexts[0].nodes.some(n=>n.pan.value>0));const count=steps.length;
  for(let i=0;i<12;i++){f.contexts[0].currentTime+=.1;audio.update(s);}assert.equal(f.events.filter(e=>e.type==='footstep').length,count,'standing or walking into a wall is silent');
  audio.setMuted(true);assert.equal(audio.getState().voices,0);audio.setMuted(false);s.me.x+=100;f.contexts[0].currentTime+=1;audio.update(s);assert.equal(f.events.filter(e=>e.type==='footstep').length,count,'unmuting or teleporting does not replay travel');audio.dispose();
});

test('footstep tracking excludes air, swimming, mounts, other worlds, floors, and distant players',async()=>{
  const {createFootstepTracker}=await modulePromise;
  for(const overrides of [{grounded:false},{ridingCamel:true},{posture:'sitting'},{fallen:true},{swimming:true},{island:'other'},{inCave:true},{y:12},{x:150}]){
    const tracker=createFootstepTracker(),listener={id:'me',x:100,y:0,z:180},p={id:'other',x:104,y:0,z:180,grounded:true,...overrides};let sounds=[];
    for(let i=0;i<15;i++){p.z-=.2;sounds.push(...tracker.update([p],listener,i*.1));}assert.equal(sounds.length,0,JSON.stringify(overrides));
  }
  const tracker=createFootstepTracker(),p={id:'me',x:100,y:0,z:180,grounded:true};tracker.update([p],p,0);p.x=210;assert.deepEqual(tracker.update([p],p,.1),[]);p.x+=1;assert.deepEqual(tracker.update([p],p,30),[],'a long pause discards old movement');tracker.clear();
  const ocean={id:'me',x:-100,z:100,y:-.6,grounded:true};for(let i=0;i<10;i++){ocean.z+=.5;assert.deepEqual(tracker.update([ocean],ocean,i*.1),[]);}
});
