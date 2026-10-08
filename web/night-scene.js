const clamp=(value,min,max)=>Math.max(min,Math.min(max,value));

// Phase lengths come from the server. A short dusk/dawn fade avoids an abrupt flash.
export function nightFactor(cycle){
  if(!cycle)return 0;
  const edge=12;
  return cycle.phase==='night'?Math.min(1,Math.max(0,cycle.elapsed)/edge):Math.max(0,1-Math.max(0,cycle.elapsed)/edge)*(cycle.elapsed<edge&&cycle.dayNumber>0?1:0);
}
export function johnPorkWarning(snapshot){
  const me=snapshot?.me,john=snapshot?.johnPork;
  if(!me||!john||john.sleeping||snapshot.dayCycle?.phase!=='night'||(me.island||'')!==(john.island||'')||!!me.inCave!==!!john.inCave)return null;
  const distance=Math.hypot(me.x-john.x,(me.y||0)-(john.y||0),me.z-john.z);
  if(distance>100)return null;
  const urgent=john.target===me.id&&distance<=10;
  return {distance,urgent,text:urgent?'RUN FROM JOHN PORK':'john pork is near'};
}

// A locally synthesized phone-call melody; no audio is fetched from another site.
export const PHONE_MELODY=[
  [0,74],[.16,81],[.32,78],[.48,81],[.64,74],[.8,81],[.96,78],[1.12,81],
  [1.48,73],[1.64,81],[1.8,78],[1.96,81],[2.12,73],[2.28,81],[2.44,78],[2.6,81],
];
export function createPhoneRinging({AudioContext=globalThis.AudioContext||globalThis.webkitAudioContext}={}){
  let context,bus,active=false,muted=false,next=0,volume=0;
  const voices=new Set();
  function stop(){active=false;next=0;if(bus&&context){bus.gain.cancelScheduledValues(context.currentTime);bus.gain.setValueAtTime(0,context.currentTime);}for(const voice of voices){try{voice.stop();}catch{}}voices.clear();}
  function unlock(){
    if(!AudioContext)return;
    try{if(!context){context=new AudioContext();bus=context.createGain();bus.gain.value=0;bus.connect(context.destination);}if(context.state==='suspended')context.resume()?.catch?.(()=>{});}catch{}
  }
  function setRange(distance,enabled=true){
    const shouldPlay=enabled&&!muted&&Number.isFinite(distance)&&distance<=100;
    if(!shouldPlay){stop();return;}
    active=true;volume=.04+.12*Math.pow(clamp(1-distance/100,0,1),.7);
    if(context&&bus)bus.gain.setTargetAtTime(volume,context.currentTime,.08);
  }
  function update(){
    if(!active||!context||context.state!=='running')return;
    const now=context.currentTime;
    if(now<next)return;
    const start=now+.025;next=now+4.6;
    for(const [offset,midi]of PHONE_MELODY){
      const at=start+offset,frequency=440*Math.pow(2,(midi-69)/12);
      for(const [multiple,strength]of [[1,.75],[2,.18],[3,.04]]){
        const oscillator=context.createOscillator(),envelope=context.createGain();oscillator.type='sine';oscillator.frequency.value=frequency*multiple;
        envelope.gain.setValueAtTime(.0001,at);envelope.gain.exponentialRampToValueAtTime(strength,at+.006);envelope.gain.exponentialRampToValueAtTime(.0001,at+.29);
        oscillator.connect(envelope);envelope.connect(bus);voices.add(oscillator);oscillator.onended=()=>{voices.delete(oscillator);oscillator.disconnect();envelope.disconnect();};oscillator.start(at);oscillator.stop(at+.3);
      }
    }
  }
  return {unlock,setRange,update,stop,setMuted(value){muted=!!value;if(muted)stop();},dispose(){stop();context?.close()?.catch?.(()=>{});context=null;},get playing(){return active&&!muted&&context?.state==='running';}};
}

export function createNightScene({THREE,parent,box,sphere,cylinder,disposeObject,document=globalThis.document,audio=createPhoneRinging(),portraitTexture}){
  const root=new THREE.Group();root.name='john-pork';root.visible=false;parent.add(root);
  const body=new THREE.Group();body.name='john-pork-body';root.add(body);
  cylinder(.3,.35,.75,'#6a6650',0,1.1,0,body,8);
  box(.26,.6,.055,'#e5ddd0',0,1.09,-.3,body);
  for(const side of [-1,1]){const lapel=box(.12,.42,.075,'#494c3e',side*.19,1.26,-.3,body);lapel.rotation.z=side*.18;}
  box(.52,.69,.04,'#efe5d6',0,1.12,-.35,body);
  if(portraitTexture===undefined&&globalThis.document?.createElementNS)portraitTexture=new THREE.TextureLoader().load('/assets/john-pork.jpg');
  if(portraitTexture)portraitTexture.colorSpace=THREE.SRGBColorSpace;
  const portrait=new THREE.Mesh(new THREE.PlaneGeometry(.47,.64),new THREE.MeshBasicMaterial({map:portraitTexture||null,color:portraitTexture?'#ffffff':'#d8b6a1',toneMapped:false}));portrait.name='john-pork-shirt-portrait';portrait.position.set(0,1.12,-.38);portrait.rotation.y=Math.PI;portrait.userData.source='/assets/john-pork.jpg';body.add(portrait);
  const head=new THREE.Group();head.name='pig-head';head.position.y=1.79;body.add(head);
  sphere(.34,'#d79c8e',0,0,0,head,2).scale.set(.95,1.06,.92);
  const snout=sphere(.19,'#e3aa9c',0,-.05,-.28,head,1);snout.scale.set(1.05,.68,.6);
  const eyes=[],closedEyes=[];
  for(const side of [-1,1]){
    sphere(.025,'#81574e',side*.065,-.045,-.389,head,1).scale.set(.8,1.2,.5);
    const ear=sphere(.16,'#cc9387',side*.245,.245,.015,head,0);ear.scale.set(.65,1.3,.43);ear.rotation.z=-side*.5;
    const inner=sphere(.09,'#a66663',side*.255,.265,-.044,head,0);inner.scale.set(.6,1.15,.35);inner.rotation.z=-side*.5;
    eyes.push(sphere(.038,'#322c29',side*.14,.075,-.278,head,1),sphere(.011,'#fff5df',side*.143,.085,-.31,head,0));
    const closed=box(.078,.012,.024,'#795a51',side*.14,.075,-.293,head);closed.name='john-pork-closed-eye';closed.visible=false;closedEyes.push(closed);
    box(.085,.022,.024,'#795a51',side*.14,.137,-.28,head);
  }
  const arms=[],legs=[];
  for(const side of [-1,1]){
    const leg=new THREE.Group();leg.position.set(side*.17,.76,0);body.add(leg);legs.push(leg);
    box(.23,.65,.24,'#3f5261',0,-.33,0,leg);box(.27,.14,.4,'#363933',0,-.66,-.08,leg);
    const arm=new THREE.Group();arm.position.set(side*.38,1.4,0);body.add(arm);arms.push(arm);
    cylinder(.105,.12,.53,'#686750',0,-.26,0,arm,7);sphere(.105,'#d79c8e',0,-.58,0,arm,1);
  }
  const phone=box(.16,.27,.045,'#1f2929',0,-.5,-.1,arms[1]);phone.name='ringing-phone';box(.125,.2,.012,'#98c3b3',0,-.5,-.128,arms[1]);
  const sleepingSign=new THREE.Group();sleepingSign.name='john-pork-sleeping-zzz';sleepingSign.visible=false;sleepingSign.position.set(.4,1.35,1.55);root.add(sleepingSign);
  const sleepCanvas=globalThis.document?.createElement?.('canvas'),sleepContext=sleepCanvas?.getContext?.('2d');
  if(sleepContext){sleepCanvas.width=256;sleepCanvas.height=128;sleepContext.font='bold 74px Georgia';sleepContext.fillStyle='#fff0cf';sleepContext.textAlign='center';sleepContext.fillText('Zzz',128,88);const texture=new THREE.CanvasTexture(sleepCanvas);texture.colorSpace=THREE.SRGBColorSpace;const label=new THREE.Sprite(new THREE.SpriteMaterial({map:texture,transparent:true,depthWrite:false}));label.scale.set(1.2,.6,1);sleepingSign.add(label);}
  let latest=null,warning=null,lastWarning='',lastClock='',muted=false,disposed=false;
  let hud,clock,notice,mute;
  if(document){
    if(!document.getElementById('night-scene-style')){const style=document.createElement('style');style.id='night-scene-style';style.textContent='#night-hud{position:fixed;z-index:12;top:77px;right:29px;display:flex;align-items:center;gap:8px;font:700 10px monospace;color:#f8efd9}#day-cycle{padding:8px 12px;border-radius:18px;background:#233d35df;letter-spacing:.6px}#ringing-toggle{border:1px solid #e5deba50;border-radius:16px;padding:7px 10px;background:#233d35df;color:inherit;font:inherit}#john-pork-warning{position:fixed;z-index:13;top:108px;left:50%;transform:translateX(-50%);padding:10px 18px;border:1px solid #e5bd72;border-radius:9px;background:#2e3028ed;color:#ffe2a4;font:700 12px monospace;text-align:center;white-space:nowrap;box-shadow:0 4px 18px #10191840}#john-pork-warning[data-urgent="true"]{border-color:#ed9a79;background:#6f342ded;color:#fff0d1;font-size:14px}@media(max-width:700px){#night-hud{top:118px;right:13px;font-size:9px}#john-pork-warning{top:161px;font-size:10px}#john-pork-warning[data-urgent="true"]{font-size:12px}}';document.head.appendChild(style);}
    hud=document.createElement('div');hud.id='night-hud';hud.hidden=true;hud.style.pointerEvents='none';hud.style.zIndex='6';
    clock=document.createElement('span');clock.id='day-cycle';hud.appendChild(clock);
    mute=document.createElement('button');mute.id='ringing-toggle';mute.type='button';mute.textContent='Ring on';mute.title='Toggle John Pork ringing';mute.style.pointerEvents='auto';hud.appendChild(mute);
    notice=document.createElement('div');notice.id='john-pork-warning';notice.hidden=true;notice.style.pointerEvents='none';notice.setAttribute('role','status');notice.setAttribute('aria-live','polite');document.body.append(hud,notice);
    try{muted=globalThis.localStorage?.getItem('cookiekill-ringing-muted')==='true';}catch{}
    const refreshMute=()=>{mute.textContent=muted?'Ring off':'Ring on';mute.setAttribute('aria-pressed',String(muted));audio.setMuted(muted);};refreshMute();
    mute.addEventListener('click',()=>{muted=!muted;try{globalThis.localStorage?.setItem('cookiekill-ringing-muted',String(muted));}catch{}refreshMute();audio.unlock();});
  }
  function sync(snapshot){
    const previous=latest?.johnPork;latest=snapshot;warning=johnPorkWarning(snapshot);const john=snapshot?.johnPork,me=snapshot?.me;
    const same=!!john&&!!me&&(john.sleeping||snapshot.dayCycle?.phase==='night')&&(john.island||'')===(me.island||'')&&!!john.inCave===!!me.inCave;
    if(same&&(!root.visible||previous?.sleeping!==john.sleeping||previous?.target!==john.target||previous?.island!==john.island||previous?.inCave!==john.inCave||Math.hypot(root.position.x-john.x,root.position.z-john.z)>20)){root.position.set(john.x,john.y||0,john.z);root.rotation.y=john.yaw||0;}root.visible=same;
    sleepingSign.visible=same&&!!john.sleeping;body.rotation.x=john?.sleeping?Math.PI/2:0;
    for(const eye of eyes)eye.visible=!john?.sleeping;for(const eye of closedEyes)eye.visible=!!john?.sleeping;
    if(hud){hud.hidden=!me||document.getElementById('modal')?.hidden===false;if(me&&snapshot.dayCycle){const cycle=snapshot.dayCycle,seconds=Math.max(0,Math.ceil(cycle.remaining)),text=(cycle.phase==='night'?'NIGHT':'DAY')+' '+Math.floor(seconds/60)+':'+String(seconds%60).padStart(2,'0');if(text!==lastClock){clock.textContent=text;lastClock=text;}}}
    if(notice){notice.hidden=!warning;if(warning&&lastWarning!==warning.text){notice.textContent=warning.text;notice.dataset.urgent=String(warning.urgent);lastWarning=warning.text;}if(!warning)lastWarning='';}
    audio.setRange(warning?.distance,!!warning&&!document?.hidden);
  }
  function update(time,dt){
    if(disposed)return;
    if(hud)hud.hidden=!latest?.me||document.getElementById('modal')?.hidden===false;
    if(root.visible&&latest?.johnPork){const john=latest.johnPork,blend=1-Math.exp(-Math.min(.1,dt)*14),distance=Math.hypot(root.position.x-john.x,root.position.z-john.z);root.position.lerp(new THREE.Vector3(john.x,john.y||0,john.z),blend);const turn=Math.atan2(Math.sin((john.yaw||0)-root.rotation.y),Math.cos((john.yaw||0)-root.rotation.y));root.rotation.y+=turn*blend;const stride=!john.sleeping&&distance>.015?Math.sin(time*10)*.55:0;legs[0].rotation.x=stride;legs[1].rotation.x=-stride;arms[0].rotation.x=!john.sleeping&&john.eating?-.85+Math.sin(time*14)*.18:-stride*.7;arms[1].rotation.x=john.sleeping?0:-.55;body.position.y=john.sleeping?Math.sin(time*1.3)*.008:Math.abs(stride)*.045;head.rotation.x=!john.sleeping&&john.eating?Math.sin(time*14)*.065:0;}
    if(document?.hidden)audio.stop();else audio.setRange(warning?.distance,!!warning);
    audio.update();
  }
  function clear(){latest=null;warning=null;root.visible=false;if(hud)hud.hidden=true;if(notice)notice.hidden=true;audio.stop();}
  return {root,sync,update,unlockAudio:()=>audio.unlock(),clear,dispose(){disposed=true;clear();root.removeFromParent();disposeObject?.(root);hud?.remove();notice?.remove();audio.dispose();}};
}
