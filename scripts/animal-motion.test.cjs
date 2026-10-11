const assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const {test}=require('node:test');
const load=async name=>import(`data:text/javascript;base64,${Buffer.from(fs.readFileSync(path.join(__dirname,'../web',name),'utf8')).toString('base64')}`);
const near=(actual,expected,label='')=>assert.ok(Math.abs(actual-expected)<1e-8,`${label}: ${actual} != ${expected}`);
const route=(extra={})=>({kind:'orbit',epoch:10,transition:3,fromX:20,fromZ:30,heading:1,centerX:25,centerZ:28,radius:4,phase:.5,angularSpeed:.4,wanderPhase:2,jumpPhase:0,speed:2,age:100,hatchAt:0,...extra});
const animal=(extra={})=>({id:'fish_1',species:'fish',egg:false,health:50,need:'',disposition:'neutral',x:20,y:.02,z:30,heading:1,yaw:1,age:100,hatchIn:0,motion:route(),...extra});

(async()=>{
  const {sampleAnimalMotion,projectAnimalSnapshot}=await load('animal-motion.js');
  const {createSnapshotDecoder}=await load('state-sync.js');
  const {createMotionClock,createMotionTrack}=await load('motion.js');

  test('JavaScript matches Go-generated route, transition, jump and egg fixtures',()=>{
    const fixtures=JSON.parse(fs.readFileSync(path.join(__dirname,'fixtures/animal-motion.json'),'utf8'));
    assert.ok(fixtures.length>=10);
    for(const fixture of fixtures){
      const pose=sampleAnimalMotion(fixture.animal,fixture.time);
      for(const [key,value] of Object.entries(fixture.expected))near(pose[key],value,`${fixture.name} ${key}`);
      near(pose.yaw,pose.heading,`${fixture.name} yaw`);
    }
  });

  test('route entry and return start at the current authoritative position without a jump',()=>{
    const a=animal();
    const start=sampleAnimalMotion(a,10);
    assert.equal(start.x,a.x);assert.equal(start.z,a.z);assert.equal(start.heading,a.heading);
    const after=sampleAnimalMotion(a,10.00001);
    assert.ok(Math.hypot(after.x-start.x,after.z-start.z)<1e-7);
    const beforeEnd=sampleAnimalMotion(a,12.99999),afterEnd=sampleAnimalMotion(a,13.00001);
    assert.ok(Math.hypot(afterEnd.x-beforeEnd.x,afterEnd.z-beforeEnd.z)<.001);
    const returning=animal({motion:route({epoch:50,fromX:32,fromZ:14,heading:-2})});
    const returned=sampleAnimalMotion(returning,50);
    assert.equal(returned.x,32);assert.equal(returned.z,14);assert.equal(returned.heading,-2);
  });

  test('sampling is independent of frame rate, packet frequency and previous display state',()=>{
    const baseline=animal(),expected=sampleAnimalMotion(baseline,20);
    for(const fps of [25,60,120]){
      const display={...baseline};
      for(let frame=0;frame<=10*fps;frame++)sampleAnimalMotion(display,10+frame/fps,display);
      for(const key of Object.keys(expected))near(display[key],expected[key],`${fps}fps ${key}`);
    }
    assert.equal(baseline.x,20);assert.equal(baseline.motion.epoch,10);
  });

  test('still animals and eggs keep their position while age and hatch deadlines advance',()=>{
    const still=animal({motion:route({kind:'still',hatchAt:130})});
    const pose=sampleAnimalMotion(still,40);
    assert.deepEqual(pose,{x:20,y:.02,z:30,heading:1,yaw:1,speed:0,age:130,hatchIn:0});
    const egg={...still,egg:true,motion:{...still.motion,age:0}};
    assert.equal(sampleAnimalMotion(egg,40).hatchIn,90);
    assert.equal(sampleAnimalMotion(egg,40).age,0);
    assert.equal(sampleAnimalMotion(egg,135).hatchIn,0);
    assert.equal(egg.egg,true,'only the server may hatch an egg');
    assert.equal(sampleAnimalMotion({...egg,motion:{...egg.motion,hatchAt:150}},40).hatchIn,110,'disturbance supplies a new deadline');
  });

  test('dolphin jump height and speed follow the shared world clock',()=>{
    const dolphin=animal({species:'dolphin',motion:route({epoch:0,transition:0})});
    near(sampleAnimalMotion(dolphin,16).y,.02);
    near(sampleAnimalMotion(dolphin,17).y,1.7);
    near(sampleAnimalMotion(dolphin,17).speed,1.5);
    near(sampleAnimalMotion(dolphin,18).y,.02);
    near(sampleAnimalMotion(animal({motion:dolphin.motion}),17).y,.02);
  });

  test('time-only deltas project unchanged routes without mutating the decoder baseline',()=>{
    const decoder=createSnapshotDecoder();
    const raw=decoder.apply({type:'snapshot',seq:1,time:10,me:{id:'swimmer'},animals:[animal()]});
    const before=JSON.stringify(raw),display=projectAnimalSnapshot(raw);
    assert.notEqual(display.animals,raw.animals);assert.notEqual(display.animals[0],raw.animals[0]);
    sampleAnimalMotion(display.animals[0],20,display.animals[0]);
    assert.equal(JSON.stringify(raw),before);
    const next=decoder.apply({type:'delta',seq:2,base:1,time:20});
    assert.equal(next.animals[0].x,20,'raw baseline stays at the transmitted anchor');
    const projected=projectAnimalSnapshot(next);
    assert.equal(projected.animals[0].x,display.animals[0].x);
    assert.equal(projected.animals[0].age,110);
    const reactive=decoder.apply({type:'delta',seq:3,base:2,time:21,animals:{upsert:[{id:'fish_1',motion:null,x:90,z:80,age:111}]}});
    const interactive=projectAnimalSnapshot(reactive);
    assert.equal(interactive.animals[0].x,90);assert.equal(interactive.animals[0].motion,null);
    assert.equal(sampleAnimalMotion(interactive.animals[0],22),null,'interactive animals use authoritative interpolation');
    interactive.animals[0].x=91;
    assert.equal(reactive.animals[0].x,90,'reactive display entities are isolated too');
    const resumed=projectAnimalSnapshot(decoder.apply({type:'delta',seq:4,base:3,time:22,animals:{upsert:[{id:'fish_1',motion:route({epoch:22,fromX:90,fromZ:80,age:112})}]}}));
    assert.equal(resumed.animals[0].x,90);assert.equal(resumed.animals[0].age,112);
  });

  test('legacy snapshots and unsupported paths preserve server-provided positions',()=>{
    for(const motion of [null,undefined,{kind:'future'}]){
      const a=animal({motion}),snapshot={time:40,animals:[a]};
      assert.equal(sampleAnimalMotion(a,40),null);
      assert.deepEqual(projectAnimalSnapshot(snapshot),snapshot);
    }
    assert.equal(sampleAnimalMotion(animal(),NaN),null);
    assert.deepEqual(projectAnimalSnapshot({time:1}).animals,[]);
  });

  test('rendering updates nearby/label data and holds routes when the snapshot stream stalls',()=>{
    const source=fs.readFileSync(path.join(__dirname,'../web/game.js'),'utf8');
    const animate=source.slice(source.indexOf('function animateAnimals('),source.indexOf('function animate(now){'));
    const clock=createMotionClock();clock.accept(10000,1000,{reset:true});
    const a=animal(),track=createMotionTrack();track.push(10000,a);
    const object={userData:{animal:a,motion:track,pose:{},lastNeed:'|neutral'},position:{set(x,y,z){this.x=x;this.y=y;this.z=z;}},rotation:{},getObjectByName:()=>null};
    const context=vm.createContext({motionClock:clock,sampleAnimalMotion,state:{animals:new Map([[a.id,object]])}});
    vm.runInContext(animate,context);
    clock.accept(11000,2000);
    context.animateAnimals(2000,10850,{x:20,z:30});
    const expected=sampleAnimalMotion(a,10.85);
    near(a.x,expected.x);near(object.position.x,a.x);near(a.age,expected.age);
    context.animateAnimals(4000,12850,{x:20,z:30});
    const held={x:a.x,z:a.z,age:a.age};
    context.animateAnimals(50000,58850,{x:20,z:30});
    assert.deepEqual({x:a.x,z:a.z,age:a.age},held);
    const next=animal({motion:route({epoch:0,fromX:-40,fromZ:-20})});
    object.userData.animal=next;clock.accept(0,51000,{reset:true});
    context.animateAnimals(51000,-150,{x:-40,z:-20});
    assert.equal(next.x,-40);assert.equal(next.z,-20,'reconnect world clock replaces the prior route');
  });
})();
