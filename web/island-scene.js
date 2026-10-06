// Procedural home-island scenery and furniture. Dimensions come from the server.
export function createIslandScene({THREE, scene, box, sphere, cylinder, mesh, mat, textSprite, disposeObject, makeAnimal}) {
  const terrain = new THREE.Group(), buildings = new THREE.Group();
  scene.add(terrain, buildings); terrain.visible = buildings.visible = false;
  const sea = mesh(new THREE.PlaneGeometry(800,800), '#559baa',0,-.35,0,terrain);
  sea.rotation.x=-Math.PI/2; sea.castShadow=false;
  cylinder(45,48,2.5,'#c9b48a',0,-1.45,0,terrain,64);
  cylinder(40,44,.3,'#e8d5a6',0,-.1,0,terrain,64);
  cylinder(34,39,.12,'#91a66d',0,.015,0,terrain,64);
  for(let i=0;i<16;i++) {
    const angle=i*2.4, radius=35+(i%3)*1.5, x=Math.sin(angle)*radius,z=Math.cos(angle)*radius;
    if(z>24&&Math.abs(x)<10)continue;
    cylinder(.13,.25,3.6,'#a18459',x,1.8,z,terrain);
    for(let j=0;j<5;j++) {const leaf=sphere(1.4,'#6d925d',x+Math.sin(j*1.26),3.8,z+Math.cos(j*1.26),terrain);leaf.scale.set(.7,.2,1);leaf.rotation.y=-j*1.26;}
    sphere(.5,'#b1b294',x+1,.25,z-.8,terrain).scale.y=.6;
  }
  box(4,.18,9,'#b49b6b',0,.13,37,terrain);
  const sign=textSprite('HOME ISLAND','#fff3d6','#3d6655',512,100);sign.position.set(0,2.3,36);sign.scale.set(4.8,.94,1);terrain.add(sign);
  const portal=new THREE.Group();portal.position.set(0,0,28);terrain.add(portal);
  cylinder(2.4,2.5,.1,'#a7aaa0',0,.035,0,portal,12);
  cylinder(2.08,2.15,.035,'#678e91',0,.102,0,portal,24);
  const portalMaterial=new THREE.MeshStandardMaterial({color:'#a1e4df',emissive:'#67c4c4',emissiveIntensity:.55,roughness:.35});
  for(const radius of [1.65,2.15]){const ring=mesh(new THREE.TorusGeometry(radius,.035,6,64),portalMaterial,0,.145,0,portal);ring.rotation.x=-Math.PI/2;}
  for(let i=0;i<8;i++) {
    const angle=i*Math.PI/4,x=Math.sin(angle)*1.9,z=Math.cos(angle)*1.9;
    const rune=box(.13,.02,.22,'#e9dcaa',x,.146,z,portal);rune.rotation.y=angle+Math.PI/4;
    if(i%2===0){const pillar=cylinder(.12,.18,.75,'#c4c5ae',Math.sin(angle)*2.45,.38,Math.cos(angle)*2.45,portal,5);sphere(.13,'#b5e8dc',pillar.position.x,.88,pillar.position.z,portal);}
  }
  const arrow=mesh(new THREE.ConeGeometry(.23,.5,3),portalMaterial,0,.8,0,portal);arrow.rotation.z=Math.PI;arrow.name='return-beacon';
  const portalLabel=textSprite('RETURN TO THE CLAY CAVE','#dbf6e8','#375d5bcc',768,100);portalLabel.position.set(0,3.2,0);portalLabel.scale.set(3.6,.47,1);portal.add(portalLabel);
  const objects = new Map();
  function objectView(object, catalog=[]) {
    const g=new THREE.Group(), recipe=catalog.find(r=>r.id===object.kind)||{};
    const w=recipe.width||2,d=recipe.depth||1,h=recipe.height||1;
    const wood='#b78c61', dark='#775a42', cream='#eee1c2';
    const legs=(width,depth,height)=>{for(const x of [-1,1])for(const z of [-1,1])box(.12,height,.12,dark,x*(width/2-.15),height/2,z*(depth/2-.15),g);};
    switch(object.kind) {
      case 'wall': box(w,h,d,cream,0,h/2,0,g);box(w,.12,d+.04,wood,0,.08,0,g);box(w,.13,d+.04,wood,0,h-.08,0,g);break;
      case 'floor': case 'roof':
        box(w,h,d,object.kind==='roof'?'#897c62':'#bd9c71',0,h/2,0,g);
        for(let i=1;i<5;i++)box(.025,.015,d,dark,-w/2+w*i/5,h+.008,0,g);break;
      case 'stairs': for(let i=0;i<6;i++)box(w,h*(i+1)/6,d/6,wood,0,h*(i+1)/12,-d/2+d*(i+.5)/6,g);break;
      case 'window':
        box(w,.65,d,cream,0,.325,0,g);box(w,.3,d,cream,0,h-.15,0,g);
        for(const x of [-1,1])box(.18,h,d,wood,x*(w/2-.09),h/2,0,g);
        box(w-.3,h-.95,.04,'#99c8c3',0,(h+.35)/2,0,g);box(.07,h-.95,d+.03,wood,0,(h+.35)/2,0,g);break;
      case 'door': {
        for(const x of [-1,1])box(.16,h,d,wood,x*(w/2-.08),h/2,0,g);
        box(w,.16,d,wood,0,h-.08,0,g);
        const hinge=new THREE.Group();g.add(hinge);hinge.position.x=-w/2+.16;hinge.rotation.y=object.open?-Math.PI/2:0;
        box(w-.32,h-.2,d*.7,dark,(w-.32)/2,(h-.2)/2,0,hinge);sphere(.065,'#ddbd72',w-.5,h*.46,d*.6,hinge);break;
      }
      case 'table': legs(w,d,h-.12);box(w,.12,d,wood,0,h-.06,0,g);break;
      case 'chair': legs(w,d,.5);box(w,.13,d,wood,0,.53,0,g);box(w,.65,.12,wood,0,.88,-d/2+.06,g);break;
      case 'bed': legs(w,d,.3);box(w,.25,d,dark,0,.35,0,g);box(w-.08,.25,d-.1,cream,0,.6,0,g);box(w-.1,.06,d*.65,'#76958e',0,.77,d*.16,g);box(w*.8,.13,d*.19,'#f7edda',0,.78,-d*.33,g);break;
      case 'plant':
        cylinder(w*.28,w*.22,h*.35,'#c58562',0,h*.175,0,g);
        for(let i=0;i<5;i++){const leaf=sphere(w*.35,['#799b60','#5b8055','#94ae69'][i%3],Math.sin(i*2.4)*w*.2,h*.5+i*h*.07,Math.cos(i*2.4)*d*.2,g);leaf.scale.set(.65,1.1,.6);}break;
      case 'lantern': cylinder(.22,.3,h*.75,dark,0,h*.375,0,g);sphere(.23,'#ffdb85',0,h*.8,0,g);break;
      case 'chest': box(w,h*.7,d,wood,0,h*.35,0,g);box(w+.04,h*.2,d+.04,dark,0,h*.8,0,g);for(const x of [-1,1])box(.1,h,d+.05,'#cdb477',x*w*.32,h/2,0,g);box(.15,.2,.05,'#edd398',0,h*.55,d/2+.04,g);break;
      case 'mixer': box(w*.8,h*.1,d*.8,'#b0bab1',0,h*.05,0,g);cylinder(w*.3,w*.2,h*.35,'#d5d8c9',0,h*.25,0,g,16);box(w*.28,h,d*.45,'#739b99',-w*.3,h/2,-d*.2,g);box(w*.8,h*.2,d*.5,'#8eafaa',0,h*.88,-d*.1,g);break;
      case 'oven': box(w,h,d,'#938773',0,h/2,0,g);box(w*.72,h*.48,.05,'#394d48',0,h*.42,d/2+.03,g);box(w*.62,.06,.09,'#d1c49c',0,h*.7,d/2+.08,g);for(const x of [-1,1])cylinder(.15,.15,.05,'#3c4d43',x*w*.23,h+.025,0,g,10);break;
      case 'garden_bed':
        box(w,h*.65,d,'#735d46',0,h*.325,0,g);for(const z of [-1,1])box(w,.2,.1,wood,0,h*.6,z*d/2,g);for(const x of [-1,1])box(.1,.2,d,wood,x*w/2,h*.6,0,g);
        if(object.planted)for(let i=0;i<4;i++){const x=(i%2-.5)*w*.5,z=(Math.floor(i/2)-.5)*d*.5;sphere(.25,'#799956',x,h+.1,z,g);sphere(.09,object.planted==='berry'?'#b85d68':'#a47a52',x+.1,h+.24,z+.1,g);}break;
      case 'pond':
        cylinder(w*.5,w*.52,.22,'#a2a38a',0,.11,0,g,20);cylinder(w*.44,w*.44,.04,'#74b6bb',0,.24,0,g,20);
        if(object.adopted){const pet=makeAnimal({species:object.adopted,need:'',health:100});pet.scale.setScalar(.6);pet.position.y=.32;const label=pet.userData.label;if(label)label.visible=false;g.add(pet);}break;
      case 'fire':
        for(let i=0;i<7;i++)sphere(.2,'#96947c',Math.sin(i*6.28/7)*.6,.12,Math.cos(i*6.28/7)*.6,g);
        for(let i=0;i<3;i++){const log=box(.15,.15,1,dark,0,.16,0,g);log.rotation.y=i*2.1;}
        if(object.lit){const flame=mesh(new THREE.ConeGeometry(.3,.8,6),'#ffc665',0,.5,0,g);flame.material=mat('#ffc665',{emissive:'#ed8744',emissiveIntensity:.6});}break;
      default: box(w,h,d,wood,0,h/2,0,g);
    }
    g.position.set(object.x||0,object.y||0,object.z||0);g.rotation.y=(object.rotation||0)*Math.PI/180;
    return g;
  }
  function sync(snapshot) {
    const active=!!snapshot.me.island;terrain.visible=buildings.visible=active;
    const present=new Set();
    for(const object of snapshot.island?.objects||[]) {
      present.add(object.id);const signature=JSON.stringify(object),old=objects.get(object.id);
      if(old?.userData.signature===signature)continue;
      if(old){buildings.remove(old);disposeObject(old);}
      const view=objectView(object,snapshot.buildCatalog);view.userData.signature=signature;objects.set(object.id,view);buildings.add(view);
    }
    for(const [id,view] of objects)if(!present.has(id)){buildings.remove(view);disposeObject(view);objects.delete(id);}
  }
  function camel() {
    const g=new THREE.Group();sphere(.85,'#caa370',0,1.5,0,g,1).scale.set(.6,.8,1.3);sphere(.5,'#b78e5c',0,2.12,.15,g);
    for(const x of [-.35,.35])for(const z of [-.65,.65])cylinder(.1,.12,1.3,'#c09a66',x,.7,z,g);
    const neck=cylinder(.18,.28,1.5,'#cba573',0,2,-.95,g);neck.rotation.x=-.4;sphere(.33,'#d3ad7b',0,2.7,-1.35,g).scale.set(.7,.75,1.4);
    box(.9,.12,.75,'#997374',0,2.27,.35,g);return g;
  }
  function animate(now){arrow.position.y=.8+Math.sin(now*.002)*.12;arrow.rotation.y=now*.0004;portalMaterial.emissiveIntensity=.5+Math.sin(now*.0015)*.12;}
  return {sync,objectView,camel,animate};
}
