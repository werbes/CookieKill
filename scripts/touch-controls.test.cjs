const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const modulePromise=import('data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(__dirname,'../web/touch-controls.js'),'utf8')).toString('base64'));
const idle={forward:0,side:0,sprint:false,jump:false};

class Element{
  constructor(parent=null,dataset={}){this.parent=parent;this.ownerDocument=parent?.ownerDocument||parent;this.dataset=dataset;this.style={};this.attributes={};this.listeners=new Map();this.captured=new Set();this.rect={left:0,top:0,width:100,height:100};}
  addEventListener(type,fn){if(!this.listeners.has(type))this.listeners.set(type,[]);this.listeners.get(type).push(fn);}
  setAttribute(name,value){this.attributes[name]=value;}
  getBoundingClientRect(){return this.rect;}
  contains(node){while(node){if(node===this)return true;node=node.parent;}return false;}
  closest(selector){for(let node=this;node;node=node.parent)if(selector==='[data-touch-action]'&&node.dataset.touchAction)return node;return null;}
  setPointerCapture(id){if(this.failCapture)throw Error('capture unavailable');this.captured.add(id);}
  releasePointerCapture(id){if(this.failCapture)throw Error('capture unavailable');if(this.captured.delete(id))this.emit('lostpointercapture',{pointerId:id});}
  emit(type,extra={}){
    const event={type,target:this,pointerType:type==='click'?'':'touch',pointerId:type==='click'?-1:1,detail:type==='click'?0:undefined,clientX:50,clientY:50,preventDefault(){this.defaultPrevented=true;},...extra};
    for(let node=this;node;node=node.parent)for(const listener of node.listeners.get(type)||[])listener(event);
    return event;
  }
}
async function fixture({activate=false}={}){
  const {createTouchControls}=await modulePromise,document=new Element(),root=new Element(document),joystick=new Element(root),stick=new Element(joystick),sprintButton=new Element(root),jumpButton=new Element(root),lookSurface=new Element(document),actionButton=new Element(root,{touchAction:'interact'}),icon=new Element(actionButton);
  const moves=[],looks=[],actions=[];let activations=0,controls;
  controls=createTouchControls({root,joystick,stick,sprintButton,jumpButton,lookSurface,onMove:()=>moves.push(controls.movement()),onLook:(...delta)=>looks.push(delta),onAction:action=>actions.push(action),onActivate:()=>{activations++;if(activate){controls.setEnabled(true);controls.setActive(true);}}});
  return {document,root,joystick,stick,sprintButton,jumpButton,lookSurface,actionButton,icon,controls,moves,looks,actions,get activations(){return activations;},start(){controls.setEnabled(true);controls.setActive(true);}};
}
const near=(value,expected)=>assert.ok(Math.abs(value-expected)<1e-9,`${value} differs from ${expected}`);

test('touch activation precedes gating, while mouse input cannot move or look',async()=>{
  const f=await fixture({activate:true});assert.equal(f.root.hidden,true);assert.deepEqual(f.controls.movement(),idle);
  f.lookSurface.emit('pointerdown',{pointerType:'mouse'});f.document.emit('pointermove',{pointerType:'mouse',clientX:80});assert.equal(f.activations,0);assert.equal(f.root.hidden,true);assert.deepEqual(f.looks,[]);
  f.lookSurface.emit('pointerdown',{pointerType:'pen'});assert.equal(f.activations,1);assert.equal(f.root.hidden,false);
  f.document.emit('pointermove',{pointerType:'pen',clientX:80,clientY:30});assert.deepEqual(f.looks,[[30,-20]]);
  f.lookSurface.emit('click');assert.deepEqual(f.actions,[],'swiping or tapping the view never throws a cookie');
});

test('joystick has a rescaled dead zone, analog speed and bounded diagonal movement',async()=>{
  const f=await fixture();f.start();f.joystick.emit('pointerdown');assert.equal(f.moves.length,1);assert.deepEqual(f.controls.movement(),idle);
  f.document.emit('pointermove',{clientX:54});assert.deepEqual(f.controls.movement(),idle);
  f.document.emit('pointermove',{clientX:75});near(f.controls.movement().side,(.5-.12)/.88);near(f.controls.movement().forward,0);assert.equal(f.moves.length,2,'leaving dead zone sends input immediately');
  f.document.emit('pointermove',{clientX:150,clientY:-50});const m=f.controls.movement();near(Math.hypot(m.forward,m.side),1);near(m.forward,m.side);assert.match(f.stick.style.transform,/translate\(/);
  f.document.emit('pointermove');assert.deepEqual(f.controls.movement(),idle);assert.equal(f.moves.length,3,'returning to center stops immediately');
  f.document.emit('pointerup');assert.equal(f.joystick.captured.size,0);assert.equal(f.stick.style.transform,'translate(0px, 0px)');assert.equal(f.moves.length,4);
});

test('movement, looking and jump hold use separate fingers and survive unrelated releases',async()=>{
  const f=await fixture();f.start();f.joystick.emit('pointerdown',{pointerId:1,clientY:0});f.lookSurface.emit('pointerdown',{pointerId:2});f.jumpButton.emit('pointerdown',{pointerId:3});
  assert.equal(f.controls.movement().jump,true);near(f.controls.movement().forward,1);assert.equal(f.jumpButton.attributes['aria-pressed'],'true');
  f.document.emit('pointermove',{pointerId:2,clientX:65,clientY:70});assert.deepEqual(f.looks,[[15,20]]);near(f.controls.movement().forward,1);
  f.lookSurface.emit('pointerdown',{pointerId:4});f.document.emit('pointermove',{pointerId:4,clientX:90});f.document.emit('pointerup',{pointerId:4});assert.equal(f.looks.length,1,'second look finger cannot replace the first');
  f.document.emit('pointercancel',{pointerId:1});assert.equal(f.controls.movement().forward,0);assert.equal(f.controls.movement().jump,true);
  f.document.emit('pointermove',{pointerId:2,clientX:70,clientY:72});assert.deepEqual(f.looks[1],[5,2]);
  f.document.emit('pointerup',{pointerId:3});assert.equal(f.controls.movement().jump,false);assert.equal(f.jumpButton.attributes['aria-pressed'],'false');
  f.lookSurface.emit('lostpointercapture',{pointerId:2});f.document.emit('pointermove',{pointerId:2,clientX:90});assert.equal(f.looks.length,2);
});

test('sprint and actions support keyboard clicks and reject disabled or inactive input',async()=>{
  const f=await fixture();f.sprintButton.emit('click');f.icon.emit('click');assert.deepEqual(f.actions,[]);assert.equal(f.controls.movement().sprint,false);
  f.start();f.sprintButton.emit('click',{detail:0});assert.equal(f.controls.movement().sprint,true);assert.equal(f.sprintButton.attributes['aria-pressed'],'true');
  f.sprintButton.emit('click');assert.equal(f.controls.movement().sprint,false);f.icon.emit('click',{detail:0});assert.deepEqual(f.actions,['interact']);
  f.actionButton.disabled=true;f.icon.emit('click');assert.deepEqual(f.actions,['interact']);f.sprintButton.disabled=true;f.sprintButton.emit('click');assert.equal(f.controls.movement().sprint,false);
  f.controls.setActive(false);f.actionButton.disabled=false;f.icon.emit('click');assert.deepEqual(f.actions,['interact']);
});

test('reset, pause and disable release every finger and cannot resume stale movement',async()=>{
  for(const stop of [f=>f.controls.reset(),f=>f.controls.setActive(false),f=>f.controls.setEnabled(false)]){
    const f=await fixture();f.start();f.joystick.emit('pointerdown',{pointerId:1,clientY:0});f.lookSurface.emit('pointerdown',{pointerId:2});f.jumpButton.emit('pointerdown',{pointerId:3});f.sprintButton.emit('click');
    const count=f.moves.length;stop(f);assert.deepEqual(f.controls.movement(),idle);assert.equal(f.moves.length,count+1,'one immediate stop packet');
    for(const element of [f.joystick,f.lookSurface,f.jumpButton])assert.equal(element.captured.size,0);
    assert.equal(f.sprintButton.attributes['aria-pressed'],'false');f.start();assert.equal(f.root.hidden,false);
    f.document.emit('pointermove',{pointerId:1,clientY:-50});f.document.emit('pointermove',{pointerId:2,clientX:90});assert.deepEqual(f.controls.movement(),idle);assert.deepEqual(f.looks,[]);
    f.jumpButton.emit('pointerdown',{pointerId:5});assert.equal(f.controls.movement().jump,true,'new touch works after resume');
  }
});

test('capture failure and lost capture still release inputs immediately',async()=>{
  const f=await fixture();f.start();f.joystick.failCapture=true;f.jumpButton.failCapture=true;
  assert.doesNotThrow(()=>f.joystick.emit('pointerdown',{clientY:0}));near(f.controls.movement().forward,1);
  f.document.emit('pointerup');assert.deepEqual(f.controls.movement(),idle);
  f.jumpButton.emit('pointerdown',{pointerId:2});assert.equal(f.controls.movement().jump,true);f.jumpButton.emit('lostpointercapture',{pointerId:2});assert.deepEqual(f.controls.movement(),idle);
  f.joystick.emit('pointerdown',{pointerId:3,clientY:0});assert.doesNotThrow(()=>f.controls.reset());assert.deepEqual(f.controls.movement(),idle);
});

test('actions and sprint respond to second-finger presses without requiring browser clicks',async()=>{
  const f=await fixture();f.start();f.joystick.emit('pointerdown',{pointerId:1,clientY:0});f.lookSurface.emit('pointerdown',{pointerId:2});
  const actionPress=f.icon.emit('pointerdown',{pointerId:3});assert.equal(actionPress.defaultPrevented,true);assert.deepEqual(f.actions,['interact']);near(f.controls.movement().forward,1);
  f.document.emit('pointerup',{pointerId:3});f.icon.emit('click',{pointerType:'touch',pointerId:3,detail:1});assert.deepEqual(f.actions,['interact'],'compatibility clicks cannot duplicate actions');
  const sprintPress=f.sprintButton.emit('pointerdown',{pointerId:4});assert.equal(sprintPress.defaultPrevented,true);assert.equal(f.controls.movement().sprint,true);
  f.document.emit('pointerup',{pointerId:4});f.sprintButton.emit('click',{pointerType:'touch',pointerId:4,detail:1});assert.equal(f.controls.movement().sprint,true);
  f.sprintButton.emit('pointerdown',{pointerId:5,pointerType:'pen'});assert.equal(f.controls.movement().sprint,false);
});

test('canceled button gestures and pre-pause fingers cannot produce delayed duplicate clicks',async()=>{
  for(const buttonName of ['actionButton','sprintButton'])for(const cancel of [f=>f.document.emit('pointercancel'),f=>f[buttonName].emit('lostpointercapture'),f=>{f.controls.setActive(false);f.controls.setActive(true);}]){
    const f=await fixture();f.start();const button=f[buttonName];button.emit('pointerdown');cancel(f);f.document.emit('pointerup');button.emit('click',{pointerType:'touch',pointerId:1,detail:1});
    assert.equal(f.actions.length,buttonName==='actionButton'?1:0);const afterCancel=f.controls.movement().sprint;
    button.emit('click',{pointerType:'',detail:1,sourceCapabilities:{firesTouchEvents:true}});assert.equal(f.controls.movement().sprint,afterCancel);assert.equal(f.actions.length,buttonName==='actionButton'?1:0);
    button.emit('pointerdown',{pointerId:2});f.document.emit('pointerup',{pointerId:2});button.emit('lostpointercapture',{pointerId:2});button.emit('click',{pointerType:'touch',pointerId:2,detail:1});
    if(buttonName==='sprintButton')assert.equal(f.controls.movement().sprint,!afterCancel,'a fresh press works normally');
    else assert.deepEqual(f.actions,['interact','interact'],'a fresh press works normally');
  }
});

test('repeated active/enabled updates preserve held fingers and sprint',async()=>{
  const f=await fixture();f.start();f.joystick.emit('pointerdown',{clientY:0});f.jumpButton.emit('pointerdown',{pointerId:2});f.sprintButton.emit('click');const count=f.moves.length;
  for(let i=0;i<20;i++){f.controls.setActive(true);f.controls.setEnabled(true);}
  assert.deepEqual(f.controls.movement(),{forward:1,side:0,sprint:true,jump:true});assert.equal(f.moves.length,count);
});
