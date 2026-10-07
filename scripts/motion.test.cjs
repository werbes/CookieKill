const assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path');
const {test}=require('node:test');

(async()=>{
  const source=fs.readFileSync(path.join(__dirname,'../web/motion.js'),'utf8');
  const {createMotionClock,createMotionTrack,LOCAL_MOTION_DELAY_MS,REMOTE_MOTION_DELAY_MS}=await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
  const near=(actual,expected,message='')=>assert.ok(Math.abs(actual-expected)<1e-8,`${message}: ${actual} != ${expected}`);

  for(const fps of [25,60,120])for(const [label,delay,jitter]of [
    ['local',LOCAL_MOTION_DELAY_MS,[0,18,-12,6,-8]],
    ['remote',REMOTE_MOTION_DELAY_MS,[0,35,-25,22,-8]],
  ])test(`${label} constant-speed 10Hz movement stays uniform at ${fps}fps with arrival jitter`,()=>{
    const clock=createMotionClock(),track=createMotionTrack(),out={};
    let next=0,previous;
    for(let frame=0;frame<=fps*4;frame++){
      const now=frame*1000/fps;
      while(next*100+jitter[next%jitter.length]<=now){
        const time=next*100,arrival=time+jitter[next%jitter.length];
        assert.equal(clock.accept(time,arrival),true);
        track.push(time,{x:time*.006,y:0,z:time*.002},{epoch:clock.generation});next++;
      }
      const pose=track.sample(clock.time(now,delay),out);
      if(now<350)continue;
      near(pose.x,(now-delay)*.006,'linear x position');
      near(pose.z,(now-delay)*.002,'linear z position');
      if(previous)near((pose.x-previous.x)/(now-previous.now)*1000,6,'constant speed between packets');
      previous={x:pose.x,now};
    }
  });

  test('stopping at a wall never predicts beyond the last confirmed position',()=>{
    const track=createMotionTrack();
    for(let time=0;time<=900;time+=100)track.push(time,{x:Math.min(2,time*.01),y:0,z:0});
    for(let time=150;time<=2000;time+=7){const pose=track.sample(time);assert.ok(pose.x<=2);if(time>=200)assert.equal(pose.x,2);}
    const latest=track.sample(2000);assert.deepEqual(latest,track.sample(100000),'a stalled stream must hold still');
  });

  test('sprint changes and jump takeoff/landing do not reset the position timeline',()=>{
    const track=createMotionTrack();
    const poses=[
      {x:0,y:0,z:0,sprinting:false,grounded:true},
      {x:.6,y:0,z:0,sprinting:false,grounded:true},
      {x:1.59,y:.57,z:0,sprinting:true,grounded:false},
      {x:2.58,y:.96,z:0,sprinting:true,grounded:false},
      {x:3.18,y:0,z:0,sprinting:false,grounded:true},
    ];
    poses.forEach((pose,i)=>assert.equal(track.push(i*100,pose),i===0));
    near(track.sample(150).x,1.095);near(track.sample(150).y,.285);
    near(track.sample(299.999).x,track.sample(300.001).x-.0000159,'speed changes have no position jolt');
    near(track.sample(350).x,2.88);near(track.sample(350).y,.48);
  });

  test('teleports, island/death/posture changes and egg transitions snap instead of sweeping',()=>{
    for(const key of ['island:friend','deaths:1','posture:chair','ridingCamel:1','egg']){
      const track=createMotionTrack();track.push(0,{x:0,y:0,z:0},{key:'before'});track.push(100,{x:1,y:0,z:0},{key:'before'});
      assert.equal(track.push(200,{x:4,y:2,z:5},{key}),true);
      assert.deepEqual(track.sample(80),{x:4,y:2,z:5,yaw:0});
      assert.equal(track.size,1);
    }
    const track=createMotionTrack();track.push(0,{x:0,y:0,z:0});
    assert.equal(track.push(100,{x:120,y:-10,z:-117}),true);
    assert.equal(track.sample(0).x,120);
  });

  test('out-of-order and invalid samples cannot pull movement backward',()=>{
    const clock=createMotionClock(),track=createMotionTrack();
    assert.equal(clock.accept(100,500),true);assert.equal(clock.accept(200,600),true);
    const epoch=clock.generation;
    assert.equal(clock.accept(150,650),false);assert.equal(clock.accept(300,550),false);assert.equal(clock.accept(NaN,650),false);
    assert.equal(clock.generation,epoch);near(clock.time(700,120),180);
    track.push(100,{x:1,z:0});track.push(200,{x:2,z:0});
    assert.equal(track.push(150,{x:-10,z:0}),false);assert.equal(track.push(300,{x:NaN,z:0}),false);
    assert.equal(track.sample(1000).x,2);assert.equal(track.size,2);
    track.push(200,{x:2.1,z:0});assert.equal(track.size,2);assert.equal(track.sample(200).x,2.1);
  });

  test('reconnects and long pauses rebase clocks and discard stale paths',()=>{
    const clock=createMotionClock(),track=createMotionTrack();
    clock.accept(500,1000);track.push(500,{x:1,z:0},{epoch:clock.generation});
    clock.accept(600,1100);track.push(600,{x:2,z:0},{epoch:clock.generation});
    assert.equal(track.sample(clock.time(10000)).x,2,'inactive tab holds latest authoritative point');
    const before=clock.generation;
    clock.accept(9500,10000);assert.ok(clock.generation>before);
    assert.equal(track.push(9500,{x:3,z:0},{epoch:clock.generation}),true);
    assert.equal(track.sample(clock.time(10000)).x,3);
    clock.accept(0,10010,{reset:true});assert.equal(clock.latestTime,0);
    assert.equal(track.push(0,{x:-35,z:-35},{epoch:clock.generation}),true);
    assert.equal(track.sample(clock.time(10010)).x,-35);
    track.push(400,{x:-34,z:-35},{epoch:clock.generation});assert.equal(track.size,1,'missing packet spans must not replay an unobserved route');
    clock.reset();track.reset();assert.equal(track.sample(0),null);assert.equal(clock.latestTime,null);
  });

  test('rotation crosses the shortest angle and sample storage is bounded and reusable',()=>{
    const track=createMotionTrack({maxSamples:6}),out={};
    track.push(0,{x:0,z:0,yaw:179*Math.PI/180});track.push(100,{x:1,z:0,yaw:-179*Math.PI/180});
    near(track.sample(50,out).yaw,Math.PI);assert.equal(track.sample(50,out),out);
    for(let i=2;i<1000;i++)track.push(i*100,{x:i,z:0});
    assert.equal(track.size,6);assert.equal(track.sample(1e9).x,999);
  });
})();
