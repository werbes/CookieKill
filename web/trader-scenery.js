// Small, hand-crafted market details. Leave the space around each trader open.
export function createTraderScenery({THREE,box,sphere,cylinder,mesh,mat,flatLabel}) {
  const rainbow=['#d87874','#dfab58','#e6d17c','#83ad85','#76afbd','#a58eba'];
  const sand='#ead8ad', wood='#8c6647';
  const rod=(a,b,r,color,g)=>{
    const start=new THREE.Vector3(...a),end=new THREE.Vector3(...b),delta=end.clone().sub(start);
    const m=cylinder(r,r,delta.length(),color,...start.clone().add(end).multiplyScalar(.5).toArray(),g,5);
    m.quaternion.setFromUnitVectors(new THREE.Vector3(0,1,0),delta.normalize());return m;
  };
  function fabric(w,h,x,y,z,g,style='woven') {
    const canvas=document.createElement('canvas');canvas.width=512;canvas.height=512;
    const ctx=canvas.getContext('2d');
    if(style==='tie-dye') {
      ctx.fillStyle='#edd9ac';ctx.fillRect(0,0,512,512);
      for(let i=18;i>=0;i--){ctx.fillStyle=rainbow[i%rainbow.length];ctx.beginPath();for(let j=0;j<=120;j++){const angle=j*Math.PI/60,r=i*19+8+Math.sin(angle*7+i*.8)*8;const px=256+Math.cos(angle)*r,py=256+Math.sin(angle)*r;j?ctx.lineTo(px,py):ctx.moveTo(px,py);}ctx.closePath();ctx.fill();}
      ctx.globalAlpha=.2;ctx.strokeStyle='#fff7dc';for(let i=0;i<45;i++){const angle=i*Math.PI*2/45;ctx.beginPath();ctx.moveTo(256,256);ctx.lineTo(256+Math.cos(angle)*370,256+Math.sin(angle)*370);ctx.stroke();}ctx.globalAlpha=1;
    }else{
      ctx.fillStyle='#9b5b50';ctx.fillRect(0,0,512,512);
      for(const [inset,color] of [[13,'#e0ba76'],[26,'#466f73'],[40,'#d8b67b'],[49,'#884f49']]){ctx.strokeStyle=color;ctx.lineWidth=10;ctx.strokeRect(inset,inset,512-inset*2,512-inset*2);}
      for(let row=0;row<3;row++)for(let col=0;col<3;col++){
        const x=134+col*122,y=134+row*122;ctx.fillStyle=(row+col)%2?'#d9ba7f':'#73a1a0';ctx.beginPath();ctx.moveTo(x,y-45);ctx.lineTo(x+35,y);ctx.lineTo(x,y+45);ctx.lineTo(x-35,y);ctx.closePath();ctx.fill();ctx.fillStyle='#ad7057';ctx.fillRect(x-8,y-8,16,16);
      }
    }
    ctx.strokeStyle='#eaddbd';ctx.globalAlpha=.07;ctx.lineWidth=1;
    for(let i=0;i<512;i+=4){ctx.beginPath();ctx.moveTo(0,i);ctx.lineTo(512,i);ctx.stroke();}ctx.globalAlpha=1;
    const map=new THREE.CanvasTexture(canvas);map.colorSpace=THREE.SRGBColorSpace;
    return mesh(new THREE.PlaneGeometry(w,h),new THREE.MeshStandardMaterial({map,roughness:1,side:THREE.DoubleSide}),x,y,z,g);
  }
  function rug(x,z,w,d,g,style='woven',turn=0){
    const carpet=fabric(w,d,x,.035,z,g,style);carpet.rotation.set(-Math.PI/2,0,turn);carpet.castShadow=false;
    // A few grouped threads at the ends keep the rugs visibly handmade.
    for(const side of [-1,1])for(let i=0;i<7;i++){
      const xx=-w*.43+i*w*.86/6,zz=side*(d/2+.07),c=Math.cos(turn),s=Math.sin(turn);
      const fringe=box(.04,.015,.15,sand,x+xx*c+zz*s,.033,z-xx*s+zz*c,g);fringe.rotation.y=turn;fringe.castShadow=false;
    }
  }
  function pot(x,z,scale,g,flowers=false){
    cylinder(.34*scale,.23*scale,.58*scale,'#b97e59',x,.29*scale,z,g,9);
    cylinder(.37*scale,.37*scale,.10*scale,'#d19b72',x,.57*scale,z,g,9);
    cylinder(.3*scale,.3*scale,.03*scale,'#65583d',x,.625*scale,z,g,9);
    for(let i=0;i<4;i++){
      const a=i*2.4,xx=x+Math.sin(a)*.19*scale,zz=z+Math.cos(a)*.19*scale;
      const leaf=sphere(.26*scale,['#789767','#91a878','#5d855f'][i%3],xx,(.78+i*.07)*scale,zz,g);leaf.scale.set(.56,1.6,.55);leaf.rotation.z=Math.sin(a)*.6;
      if(flowers){const y=(1+i*.06)*scale;for(let p=0;p<5;p++){const angle=p*Math.PI*2/5;sphere(.075*scale,rainbow[i%6],xx+Math.sin(angle)*.1*scale,y+Math.cos(angle)*.1*scale,zz+.08*scale,g);}sphere(.05*scale,'#f2ce71',xx,y,zz+.13*scale,g);}
    }
  }
  function peaceMark(x,y,z,r,g){
    mesh(new THREE.TorusGeometry(r,r*.075,5,32),'#f3d994',x,y,z,g);
    rod([x,y+r,z],[x,y-r,z],r*.06,'#f3d994',g);
    for(const side of [-1,1])rod([x,y,z],[x+side*r*.7,y-r*.7,z],r*.06,'#f3d994',g);
  }
  function lantern(x,y,z,g){
    cylinder(.018,.018,.35,wood,x,y+.28,z,g,5);
    cylinder(.19,.19,.3,'#e9ba61',x,y,z,g,6).material=mat('#e9ba61',{emissive:'#de963f',emissiveIntensity:.45});
    cylinder(.08,.24,.17,'#7d7558',x,y+.23,z,g,6);cylinder(.24,.12,.12,'#7d7558',x,y-.21,z,g,6);
    for(let i=0;i<4;i++){const a=i*Math.PI/2;rod([x+Math.cos(a)*.18,y-.17,z+Math.sin(a)*.18],[x+Math.cos(a)*.18,y+.17,z+Math.sin(a)*.18],.015,'#7d7558',g);}
  }
  function decorateAbu(g){
    rug(-.1,-1.35,4.5,4.25,g);
    rug(-2.65,-2.45,1.5,2.1,g,'woven',.23);
    const colors=['#658d8b','#e2bb73','#ad6154','#ead5a6','#658d8b'];
    for(let i=0;i<5;i++)box(.8,.035,3,colors[i],-1.6+i*.8,3.08,-2,g);
    box(4,.17,.055,'#915448',0,2.96,-.49,g);
    for(let i=0;i<13;i++){
      const x=-1.87+i*.312;rod([x,2.91,-.48],[x,2.72-(i%2)*.08,-.48],.017,sand,g);
      sphere(.038,colors[i%colors.length],x,2.7-(i%2)*.08,-.48,g);
    }
    fabric(2.5,1.9,0,1.93,-3.47,g);
    lantern(-1.4,2.38,-.73,g);lantern(1.4,2.38,-.73,g);
    pot(-2.7,-.85,1.25,g);pot(2.1,-3.05,.85,g);
    for(let i=0;i<3;i++){
      const sack=sphere(.42,['#baaa7a','#d6bd89','#b89468'][i],-2.45+i*.49,.43,-3.15-(i%2)*.25,g);sack.scale.set(.9,1.1,.78);
      cylinder(.12,.2,.12,wood,-2.45+i*.49,.84,-3.15-(i%2)*.25,g,6);
    }
    const cushion=box(.85,.22,.65,'#719593',-1.05,.16,-2.8,g);cushion.rotation.y=.18;
    const cushion2=box(.75,.18,.6,'#cf9b60',-.45,.15,-2.85,g);cushion2.rotation.y=-.13;
    // The camel drinks behind the right side of the stall, clear of Abu's approach.
    box(1.7,.13,.8,wood,4.05,.2,-2.85,g);
    for(const side of [-1,1]){box(1.8,.48,.11,'#b49161',4.05,.4,-2.85+side*.43,g);box(.11,.48,.8,'#b49161',4.05+side*.85,.4,-2.85,g);}
    box(1.58,.025,.68,'#75b0b2',4.05,.48,-2.85,g);
    box(.85,.55,.75,'#c9af6d',4.55,.28,-.5,g);for(const x of [4.3,4.8])box(.045,.57,.77,wood,x,.29,-.5,g);
    cylinder(.06,.08,1.55,wood,-2.75,.775,.65,g);
    box(1.7,.9,.09,'#6d8b84',-2.75,1.26,.65,g);
    const title=flatLabel('CAMEL CARAVAN','#f5e3b4',null,512,95);title.position.set(-2.75,1.45,.71);title.scale.set(1.6,.3,1);g.add(title);
    const price=flatLabel('RENT 50  /  OWN 350','#f6e6c2',null,512,85);price.position.set(-2.75,1.10,.71);price.scale.set(1.5,.25,1);g.add(price);
  }
  function decoratePeace(g){
    rug(0,-.25,3.65,2.5,g,'tie-dye');
    fabric(3.3,1.3,0,1.98,-2.39,g,'tie-dye');
    for(let i=0;i<6;i++){
      const x=-1.67+i*.667,strip=box(.66,.025,2.4,rainbow[i],x,2.99+x*.06,-1.8,g);strip.rotation.z=.06;
    }
    rod([-2,2.84,-.55],[2,3.08,-.55],.025,sand,g);
    for(let i=0;i<10;i++){
      const x=-1.8+i*.4,flag=mesh(new THREE.ConeGeometry(.13,.27,3),rainbow[i%6],x,2.69+x*.06,-.54,g);flag.rotation.z=Math.PI;flag.rotation.y=Math.PI/2;
    }
    // An unmistakable peace sign and a small dreamcatcher frame the trader.
    cylinder(.05,.07,2.9,wood,-2.5,1.45,-1.65,g,5);
    peaceMark(-2.5,2.05,-1.6,.56,g);
    for(let i=0;i<5;i++)sphere(.055,rainbow[i],-2.5,1.32-i*.12,-1.61,g);
    const love=flatLabel('PEACE & LOVE','#725c58','#f0d7a4',384,90);love.position.set(-2.5,.63,-1.56);love.scale.set(1.25,.29,1);g.add(love);
    cylinder(.045,.065,2.8,wood,2.5,1.4,-1.7,g,5);
    mesh(new THREE.TorusGeometry(.33,.028,5,24),'#dec99a',2.5,2.13,-1.65,g);
    for(let i=0;i<6;i++){
      const a=i*Math.PI/3,b=a+Math.PI*2/3;
      rod([2.5+Math.cos(a)*.3,2.13+Math.sin(a)*.3,-1.65],[2.5+Math.cos(b)*.3,2.13+Math.sin(b)*.3,-1.65],.008,'#e5d8b4',g);
    }
    for(let i=0;i<3;i++){
      const x=2.5+(i-1)*.17,y=1.72-Math.abs(i-1)*.08;rod([x,y+.13,-1.65],[x,y-.25,-1.65],.012,sand,g);
      for(let j=0;j<2;j++)sphere(.045,rainbow[(i+j+2)%6],x,y-j*.1,-1.65,g);
      const feather=sphere(.13,rainbow[i+3],x,y-.32,-1.65,g);feather.scale.set(.35,1.6,.18);feather.rotation.z=(i-1)*.2;
    }
    pot(-2.5,-2.7,.88,g,true);pot(2.7,-2.65,1,g,true);
    const cushion=box(.7,.17,.6,'#aa8dad',-1.16,.13,-.55,g);cushion.rotation.y=-.22;
    const cushion2=box(.62,.2,.65,'#d9ab68',1.1,.15,-.65,g);cushion2.rotation.y=.18;
    // Painted flowers on the counter corners complement its existing shop sign.
    for(const x of [-1.5,1.5]){for(let i=0;i<5;i++){const a=i*Math.PI*2/5;sphere(.09,'#e9c67b',x+Math.sin(a)*.12,.73+Math.cos(a)*.12,-1.18,g).scale.z=.2;}sphere(.06,'#b67d88',x,.73,-1.15,g).scale.z=.2;}
  }
  return {decorateAbu,decoratePeace};
}
