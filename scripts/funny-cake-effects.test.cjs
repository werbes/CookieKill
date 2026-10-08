const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const moduleURL='data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(__dirname,'../web/funny-cake-effects.js'),'utf8')).toString('base64');
const effects=import(moduleURL);
const canvas=()=>({style:{filter:'',transform:'',transformOrigin:'',willChange:''}});
const hue=style=>Number(style.filter.match(/hue-rotate\(([-\d.]+)deg\)/)[1]);
const saturation=style=>Number(style.filter.match(/saturate\(([\d.]+)\)/)[1]);

test('funny cake gradually starts, drifts color and motion, and fades before expiry',async()=>{
  const {createFunnyCakeEffects}=await effects,world=canvas(),fx=createFunnyCakeEffects({canvas:world});
  fx.update({remaining:30,time:100});assert.equal(saturation(world.style),1);assert.equal(hue(world.style),0);
  fx.update({remaining:30,time:100.45});const midway=saturation(world.style);assert.ok(midway>1&&midway<1.65);
  fx.update({remaining:29,time:101});assert.equal(saturation(world.style),1.65);const filter=world.style.filter,transform=world.style.transform;
  fx.update({remaining:19,time:111});assert.notEqual(world.style.filter,filter);assert.notEqual(world.style.transform,transform);
  fx.update({remaining:1.5,time:128.5});assert.ok(saturation(world.style)>1&&saturation(world.style)<1.65);
  fx.update({remaining:.1,time:129.9});assert.ok(saturation(world.style)<1.01);
  fx.update({remaining:0,time:130});assert.deepEqual(world,canvas());
});

test('reduced preference removes sway and leaves a steady gentle tint',async()=>{
  const {createFunnyCakeEffects}=await effects,world=canvas(),fx=createFunnyCakeEffects({canvas:world});
  fx.update({remaining:40,time:0});fx.update({remaining:39,time:1});assert.notEqual(world.style.transform,'');
  fx.update({remaining:38,time:2,reduced:true});const tint=world.style.filter;
  assert.equal(world.style.transform,'');assert.equal(world.style.willChange,'');assert.equal(hue(world.style),15);assert.equal(saturation(world.style),1.14);
  fx.update({remaining:20,time:20,reduced:true});assert.equal(world.style.filter,tint);assert.equal(world.style.transform,'');
  fx.update({remaining:19,time:21,reduced:false});assert.notEqual(world.style.transform,'');
  fx.dispose();assert.deepEqual(world,canvas());
});

test('canvas overscan covers maximum sway throughout the trip at every aspect ratio',async()=>{
  const {createFunnyCakeEffects}=await effects,world=canvas(),fx=createFunnyCakeEffects({canvas:world});
  for(let time=0;time<60;time+=.13){
    fx.update({remaining:60-time,time});
    const [,x,y,sx,sy]=world.style.transform.match(/translate\(([-\d.]+)%,([-\d.]+)%\) scale\(([\d.]+),([\d.]+)\)/).map(Number);
    assert.ok((sx-1)*50+.0005>=Math.abs(x));assert.ok((sy-1)*50+.0005>=Math.abs(y));
  }
});

test('disconnect, expiry and disposal restore original styles without touching HUD',async()=>{
  const {createFunnyCakeEffects}=await effects;
  const original={filter:'brightness(.9)',transform:'translateZ(0)',transformOrigin:'center',willChange:'opacity'};
  const world={style:{...original}},hud={style:{filter:'',transform:''}},fx=createFunnyCakeEffects({canvas:world,document:{body:hud}});
  fx.update({remaining:30,time:0});fx.update({remaining:29,time:1});assert.match(world.style.filter,/^brightness\(\.9\)/);assert.match(world.style.transform,/translateZ\(0\)$/);
  fx.update({remaining:29,time:2,enabled:false});assert.deepEqual(world.style,original);assert.deepEqual(hud.style,{filter:'',transform:''});
  fx.update({remaining:4,time:3});fx.update({remaining:4,time:4});assert.notDeepEqual(world.style,original);
  fx.update({remaining:4,time:7});assert.deepEqual(world.style,original);
  fx.update({remaining:4,time:8});assert.deepEqual(world.style,original,'stale positive snapshot must not restart an expired trip');
  fx.update({remaining:30,time:9});fx.update({remaining:29,time:10});assert.notDeepEqual(world.style,original);
  fx.dispose();fx.dispose();assert.deepEqual(world.style,original);
});

test('invalid values and missing renderer are harmless and stop an existing trip',async()=>{
  const {createFunnyCakeEffects}=await effects,world=canvas(),fx=createFunnyCakeEffects({canvas:world});
  for(const remaining of [NaN,Infinity,-1,undefined]){
    fx.update({remaining:30,time:1});fx.update({remaining:29,time:2});fx.update({remaining,time:3});assert.deepEqual(world,canvas());
  }
  fx.update({remaining:30,time:1});fx.update({remaining:30,time:NaN});assert.deepEqual(world,canvas());
  assert.doesNotThrow(()=>createFunnyCakeEffects({canvas:null}).update({remaining:30,time:1}));
});
